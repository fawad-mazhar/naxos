// internal/worker/worker_pool.go
package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fawad-mazhar/naxos/internal/models"
)

type TaskExecutor struct {
	registry    *Registry
	workers     int
	tasks       chan taskWork
	wg          sync.WaitGroup
	taskTimeout time.Duration
}

// taskWork combines the task execution info and its data
type taskWork struct {
	task *models.TaskExecution
	data map[string]interface{}
}

func NewTaskExecutor(registry *Registry, workers int, taskTimeout time.Duration) *TaskExecutor {
	return &TaskExecutor{
		registry:    registry,
		workers:     workers,
		tasks:       make(chan taskWork, workers*2),
		taskTimeout: taskTimeout,
	}
}

func (e *TaskExecutor) Start(ctx context.Context) {
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(ctx)
	}
}

func (e *TaskExecutor) Stop() {
	close(e.tasks)
	e.wg.Wait()
}

func (e *TaskExecutor) ExecuteTask(task *models.TaskExecution, data map[string]interface{}) error {
	// First verify that the task function exists
	if _, err := e.registry.Get(task.TaskID); err != nil {
		return fmt.Errorf("task function not found: %w", err)
	}

	// Create work item
	work := taskWork{
		task: task,
		data: data,
	}

	// Try to queue the work
	select {
	case e.tasks <- work:
		return nil
	default:
		return fmt.Errorf("task queue full")
	}
}

func (e *TaskExecutor) worker(ctx context.Context) {
	defer e.wg.Done()

	for work := range e.tasks {
		func() {
			taskCtx, cancel := context.WithTimeout(ctx, e.taskTimeout)
			defer cancel()

			fn, err := e.registry.Get(work.task.TaskID)
			if err != nil {
				log.Printf("Error getting task function: %v", err)
				return
			}

			if err := fn(taskCtx, work.data); err != nil {
				log.Printf("Error executing task %s: %v", work.task.ID, err)
			}
		}()
	}
}
