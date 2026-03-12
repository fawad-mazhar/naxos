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
	id            string
	config        *config.Config
	db            *postgres.Client
	cache         *leveldb.Client
	queue         *queue.NATS
	stopChan      chan struct{}
	isShutdown    bool
	shutdownLock  sync.RWMutex
	ongoingJobs   sync.Map
	taskFunctions map[string]worker.TaskFunction
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

// NewRunner creates a new Runner instance for processing jobs.
func NewRunner(cfg *config.Config, db *postgres.Client, cache *leveldb.Client, queue *queue.NATS, taskFunctions map[string]worker.TaskFunction) *Runner {
	return &Runner{
		id:            uuid.New().String(),
		config:        cfg,
		db:            db,
		cache:         cache,
		queue:         queue,
		stopChan:      make(chan struct{}),
		taskFunctions: taskFunctions,
	}
}

// Start begins the runner's main processing loop
func (r *Runner) Start(ctx context.Context) error {
	log.Printf("Starting runner %s", r.id)

	// Start consuming jobs from NATS JetStream
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
		case msg, ok := <-jobsChan:
			if !ok {
				return fmt.Errorf("jobs channel closed")
			}

			// Decode job execution message
			var job models.JobExecution
			if err := json.Unmarshal(msg.Data(), &job); err != nil {
				log.Printf("Error decoding job: %v", err)
				if termErr := msg.Term(); termErr != nil {
					log.Printf("Error terminating message: %v", termErr)
				}
				continue
			}

			// Process one job at a time
			if err := r.processJob(ctx, &job); err != nil {
				log.Printf("Error processing job %s: %v", job.ID, err)
				if nakErr := msg.Nak(); nakErr != nil {
					log.Printf("Error naking message: %v", nakErr)
				}
				continue
			}

			if err := msg.Ack(); err != nil {
				log.Printf("Error acking message: %v", err)
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
	r.shutdownLock.Lock()
	r.isShutdown = true
	r.shutdownLock.Unlock()

	// Signal main loop to stop
	close(r.stopChan)

	log.Printf("Runner %s shutdown initiated", r.id)
	return nil
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
