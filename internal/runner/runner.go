// internal/runner/runner.go
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fawad-mazhar/naxos/internal/config"
	"github.com/fawad-mazhar/naxos/internal/models"
	"github.com/fawad-mazhar/naxos/internal/queue"
	"github.com/fawad-mazhar/naxos/internal/storage/leveldb"
	"github.com/fawad-mazhar/naxos/internal/storage/postgres"
	"github.com/fawad-mazhar/naxos/internal/worker"
	"github.com/google/uuid"
)

type Runner struct {
	id                string
	config            *config.Config
	db                *postgres.Client
	cache             *leveldb.Client
	queue             *queue.RabbitMQ
	workerPool        chan struct{}
	workers           sync.WaitGroup
	stopChan          chan struct{}
	isShutdown        bool
	shutdownLock      sync.RWMutex
	healthCheckTicker *time.Ticker
	ongoingJobs       sync.Map
	taskFunctions     map[string]worker.TaskFunction
}

type jobState struct {
	taskStatuses map[string]models.TaskStatus
	mu           sync.RWMutex
}

// Helper method for job definition cache key
func getJobDefinitionCacheKey(definitionID string) string {
	const jobDefinitionCachePrefix = "jobdef:"
	return fmt.Sprintf("%s%s", jobDefinitionCachePrefix, definitionID)
}

func newJobState() *jobState {
	return &jobState{
		taskStatuses: make(map[string]models.TaskStatus),
	}
}

func (js *jobState) getStatus(taskID string) models.TaskStatus {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.taskStatuses[taskID]
}

func (js *jobState) setStatus(taskID string, status models.TaskStatus) {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.taskStatuses[taskID] = status
}

func NewRunner(cfg *config.Config, db *postgres.Client, cache *leveldb.Client, queue *queue.RabbitMQ, taskFunctions map[string]worker.TaskFunction) *Runner {
	return &Runner{
		id:            uuid.New().String(),
		config:        cfg,
		db:            db,
		cache:         cache,
		queue:         queue,
		workerPool:    make(chan struct{}, cfg.Worker.MaxWorkers),
		stopChan:      make(chan struct{}),
		taskFunctions: taskFunctions,
	}
}

// Start begins the runner's main processing loop
func (r *Runner) Start(ctx context.Context) error {
	log.Printf("Starting runner %s with %d workers", r.id, r.config.Worker.MaxWorkers)

	// Publish started status
	if err := r.publishStatus(models.RunnerStarted); err != nil {
		log.Printf("Failed to publish start status: %v", err)
	}

	// Start health check ticker
	r.healthCheckTicker = time.NewTicker(1 * time.Minute)
	go r.runHealthChecks()

	// Start consuming jobs from RabbitMQ
	jobsChan, err := r.queue.ConsumeJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to start consuming jobs: %w", err)
	}

	// Recovery of in-progress jobs from previous sessions
	if err := r.recoverInProgressJobs(ctx); err != nil {
		log.Printf("Warning: job recovery failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.stopChan:
			return nil
		case delivery, ok := <-jobsChan:
			if !ok {
				return fmt.Errorf("jobs channel closed")
			}

			// Decode job execution message
			var job models.JobExecution
			if err := json.Unmarshal(delivery.Body, &job); err != nil {
				log.Printf("Error decoding job: %v", err)
				delivery.Nack(false, false) // Don't requeue malformed messages
				continue
			}

			// Try to acquire worker slot
			select {
			case r.workerPool <- struct{}{}:
				r.workers.Add(1)
				go func(job models.JobExecution) {
					defer func() {
						<-r.workerPool // Release worker slot
						r.workers.Done()
					}()

					if err := r.processJob(ctx, &job); err != nil {
						log.Printf("Error processing job %s: %v", job.ID, err)
						delivery.Nack(false, true) // Requeue on processing error
						return
					}

					delivery.Ack(false)
				}(job)
			default:
				// Worker pool full, nack with requeue
				delivery.Nack(false, true)
			}
		}
	}
}

// processJob handles the execution of a single job
func (r *Runner) processJob(ctx context.Context, job *models.JobExecution) error {
	r.ongoingJobs.Store(job.ID, job)
	defer r.ongoingJobs.Delete(job.ID)

	// Try to claim the job
	claimed, err := r.db.ClaimJob(ctx, job.ID, r.id)
	if err != nil {
		return fmt.Errorf("failed to claim job: %w", err)
	}

	if !claimed {
		// Job was already claimed by another runner
		return nil
	}

	// Get job definition
	jobDef, err := r.getJobDefinition(ctx, job.DefinitionID)
	if err != nil {
		return fmt.Errorf("failed to get job definition: %w", err)
	}

	// Create execution context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, time.Duration(r.config.Worker.ShutdownTimeout)*time.Second)
	defer cancel()

	// Execute job tasks
	if err := r.executeTasks(jobCtx, job, jobDef); err != nil {
		// Update job status to failed
		if updateErr := r.db.UpdateJobStatus(ctx, job.ID, models.JobStatusFailed); updateErr != nil {
			log.Printf("Error updating failed job status: %v", updateErr)
		}
		return fmt.Errorf("failed to execute tasks: %w", err)
	}

	// All tasks completed successfully, update job status with end time
	now := time.Now()
	if err := r.db.CompleteJob(ctx, job.ID, now); err != nil {
		return fmt.Errorf("failed to update job completion status: %w", err)
	}

	return nil
}

func (r *Runner) getJobDefinition(ctx context.Context, definitionID string) (*models.JobDefinition, error) {
	cacheKey := getJobDefinitionCacheKey(definitionID)

	// Try to get from cache first
	cachedData, err := r.cache.Get(cacheKey)
	if err == nil && cachedData != nil {
		var jobDef models.JobDefinition
		if err := json.Unmarshal(cachedData, &jobDef); err == nil {
			log.Printf("Cache hit: retrieved job definition %s from cache", definitionID)
			return &jobDef, nil
		}
		log.Printf("Warning: failed to unmarshal cached job definition: %v", err)
	}

	// Cache miss or error, get from database
	jobDef, err := r.db.GetJobDefinition(ctx, definitionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job definition from database: %w", err)
	}

	// Cache the job definition for future use
	if data, err := json.Marshal(jobDef); err == nil {
		if err := r.cache.Put(cacheKey, data); err != nil {
			log.Printf("Warning: failed to cache job definition %s: %v", definitionID, err)
		} else {
			log.Printf("Cached job definition %s", definitionID)
		}
	}

	return jobDef, nil
}

// recoverInProgressJobs attempts to recover jobs that were in progress during previous shutdown
func (r *Runner) recoverInProgressJobs(ctx context.Context) error {
	jobs, err := r.db.GetInProgressJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get in-progress jobs: %w", err)
	}

	for _, job := range jobs {
		// Try to claim this stale job
		claimed, err := r.db.ClaimStaleJob(ctx, job.ID, r.id)
		if err != nil {
			log.Printf("Error claiming stale job %s: %v", job.ID, err)
			continue
		}

		if !claimed {
			// Job was either claimed by another runner or is no longer stale
			continue
		}

		// Get the current state of tasks for this job
		tasks, err := r.db.GetJobTasks(ctx, job.ID)
		if err != nil {
			log.Printf("Error getting tasks for job %s: %v", job.ID, err)
			continue
		}

		// Initialize task statuses map
		job.TaskStatuses = make(map[string]models.TaskStatus)
		for _, task := range tasks {
			job.TaskStatuses[task.TaskID] = task.Status
		}

		// Re-queue the job for processing
		if err := r.queue.PublishJob(ctx, job); err != nil {
			log.Printf("Failed to requeue job %s: %v", job.ID, err)
			continue
		}

		log.Printf("Successfully recovered job %s", job.ID)
	}

	return nil
}

// Shutdown initiates a graceful shutdown of the runner
func (r *Runner) Shutdown(timeout time.Duration) error {
	// Publish stopping status
	if err := r.publishStatus(models.RunnerStopping); err != nil {
		log.Printf("Failed to publish stopping status: %v", err)
	}

	r.shutdownLock.Lock()
	r.isShutdown = true
	r.shutdownLock.Unlock()

	// Stop health check ticker
	if r.healthCheckTicker != nil {
		r.healthCheckTicker.Stop()
	}

	// Signal main loop to stop
	close(r.stopChan)

	// Wait for ongoing jobs with timeout
	done := make(chan struct{})
	go func() {
		r.workers.Wait()
		close(done)
	}()

	var shutdownErr error
	select {
	case <-done:
		shutdownErr = nil
	case <-time.After(timeout):
		shutdownErr = fmt.Errorf("shutdown timed out after %v", timeout)
	}

	// Publish final stopped status
	if err := r.publishStatus(models.RunnerStopped); err != nil {
		log.Printf("Failed to publish stopped status: %v", err)
	}

	return shutdownErr
}

// IsShutdown returns the current shutdown status
func (r *Runner) IsShutdown() bool {
	r.shutdownLock.RLock()
	defer r.shutdownLock.RUnlock()
	return r.isShutdown
}

// executeTasks handles the execution of all tasks in the job
func (r *Runner) executeTasks(ctx context.Context, job *models.JobExecution, jobDef *models.JobDefinition) error {
	// Create thread-safe state tracking
	jobState := newJobState()
	completedTasks := &sync.Map{}
	scheduledTasks := &sync.Map{} // Track which tasks have been scheduled

	// Create error channel for task execution errors
	errChan := make(chan error, len(jobDef.Tasks))

	// Create wait group for parallel task execution
	var wg sync.WaitGroup

	// Counter for remaining tasks with mutex protection
	remainingCounter := &struct {
		count int
		mu    sync.Mutex
	}{count: len(jobDef.Tasks)}

	// Function to safely decrease remaining count
	decrementRemaining := func() {
		remainingCounter.mu.Lock()
		remainingCounter.count--
		remainingCounter.mu.Unlock()
	}

	// Function to check if dependencies are met
	dependenciesMet := func(taskID string) bool {
		for _, depID := range jobDef.Graph[taskID] {
			if _, completed := completedTasks.Load(depID); !completed {
				return false
			}
		}
		return true
	}

	// Iterate until all tasks are completed or an error occurs
	for {
		remainingCounter.mu.Lock()
		if remainingCounter.count == 0 {
			remainingCounter.mu.Unlock()
			break
		}
		remainingCounter.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return fmt.Errorf("task execution failed: %w", err)
		default:
			// Find tasks that are ready to execute
			for taskID, task := range jobDef.Tasks {
				// Skip if task is already completed
				if _, completed := completedTasks.Load(taskID); completed {
					continue
				}

				// Skip if task is already scheduled
				if _, scheduled := scheduledTasks.LoadOrStore(taskID, true); scheduled {
					continue
				}

				// Check if task is already running
				status := jobState.getStatus(taskID)
				if status == models.TaskStatusRunning {
					continue
				}

				// Check if dependencies are met
				if !dependenciesMet(taskID) {
					scheduledTasks.Delete(taskID) // Allow task to be scheduled later
					continue
				}

				// Execute task in parallel
				wg.Add(1)
				go func(taskID string, task models.Task) {
					defer wg.Done()

					if err := r.executeTask(ctx, taskID, &task, job, jobState); err != nil {
						errChan <- fmt.Errorf("task %s failed: %w", taskID, err)
						return
					}

					completedTasks.Store(taskID, true)
					decrementRemaining()
				}(taskID, task)
			}
		}

		// Small sleep to prevent tight loop
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all tasks to complete
	wg.Wait()

	// Check for any errors that occurred
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// executeTask handles execution of a single task
func (r *Runner) executeTask(ctx context.Context, taskID string, task *models.Task, job *models.JobExecution, state *jobState) error {
	// Create task execution record
	taskExec := &models.TaskExecution{
		ID:        uuid.New().String(),
		JobID:     job.ID,
		TaskID:    taskID,
		Status:    models.TaskStatusPending,
		StartTime: time.Now(),
	}

	// Store the initial task execution record
	if err := r.db.CreateTaskExecution(ctx, taskExec); err != nil {
		return fmt.Errorf("failed to create task execution: %w", err)
	}

	// Update task status to running
	state.setStatus(taskID, models.TaskStatusRunning)
	if err := r.db.UpdateTaskStatus(ctx, taskExec.ID, models.TaskStatusRunning, nil); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Execute task with retries
	var lastErr error
	for attempt := 0; attempt <= task.MaxRetry; attempt++ {
		if attempt > 0 {
			backoffDuration := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDuration):
			}
		}

		err := r.executeTaskFunction(ctx, task.FunctionName, job.Data)
		if err == nil {
			// Task completed successfully
			state.setStatus(taskID, models.TaskStatusCompleted)
			return r.db.UpdateTaskStatus(ctx, taskExec.ID, models.TaskStatusCompleted, nil)
		}

		lastErr = err
		log.Printf("Task %s attempt %d/%d failed: %v", taskID, attempt+1, task.MaxRetry+1, err)
	}

	// All retries failed
	state.setStatus(taskID, models.TaskStatusFailed)
	return r.db.UpdateTaskStatus(ctx, taskExec.ID, models.TaskStatusFailed, lastErr)
}

// executeTaskFunction executes the actual task function
func (r *Runner) executeTaskFunction(ctx context.Context, functionName string, data map[string]interface{}) error {
	// Get task function from our map
	fn, exists := r.taskFunctions[functionName]
	if !exists {
		return fmt.Errorf("task function %s not found", functionName)
	}

	// Execute the task function with context and data
	return fn(ctx, data)
}

func (r *Runner) publishStatus(event models.RunnerEventType) error {
	activeJobs := 0
	r.ongoingJobs.Range(func(key, value interface{}) bool {
		activeJobs++
		return true
	})

	// Create runner status
	runnerStatus := &models.RunnerStatus{
		ID:          r.id,
		Event:       event,
		Timestamp:   time.Now(),
		WorkerCount: r.config.Worker.MaxWorkers,
		ActiveJobs:  activeJobs,
	}

	// Create status message that wraps runner status
	statusMsg := &models.StatusMessage{
		Type:      "runner",
		ID:        r.id,
		Status:    string(event),
		Timestamp: time.Now(),
		Metadata:  runnerStatus,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.queue.PublishStatus(ctx, statusMsg); err != nil {
		return fmt.Errorf("failed to publish status: %w", err)
	}

	return nil
}

// Health check routine
func (r *Runner) runHealthChecks() {
	for {
		select {
		case <-r.stopChan:
			return
		case <-r.healthCheckTicker.C:
			if err := r.publishStatus(models.RunnerHealthy); err != nil {
				log.Printf("Failed to publish health status: %v", err)
			}
		}
	}
}
