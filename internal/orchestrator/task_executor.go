// internal/orchestrator/task_executor.go
package orchestrator

import (
	"sync"

	"github.com/fawad-mazhar/naxos/internal/models"
)

// TaskGraph represents a directed acyclic graph of tasks
type TaskGraph struct {
	Tasks        map[string]models.Task // All tasks
	Dependencies map[string][]string    // Task ID -> Dependencies IDs
	Completed    map[string]bool        // Track completed tasks
	mutex        sync.RWMutex           // Protect concurrent access
}

// NewTaskGraph creates a task dependency graph from job definition
func NewTaskGraph(jobDef *models.JobDefinition) *TaskGraph {
	return &TaskGraph{
		Tasks:        jobDef.Tasks,
		Dependencies: jobDef.Graph,
		Completed:    make(map[string]bool),
		mutex:        sync.RWMutex{},
	}
}

// IsReady checks if a task is ready to execute (all dependencies completed)
func (g *TaskGraph) IsReady(taskID string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	for _, depID := range g.Dependencies[taskID] {
		if !g.Completed[depID] {
			return false
		}
	}
	return true
}

// MarkCompleted marks a task as completed
func (g *TaskGraph) MarkCompleted(taskID string) {
	g.mutex.Lock()
	g.Completed[taskID] = true
	g.mutex.Unlock()
}

// GetReadyTasks returns all tasks that are ready to execute
func (g *TaskGraph) GetReadyTasks() []string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	var readyTasks []string
	for taskID := range g.Tasks {
		if !g.Completed[taskID] && g.IsReady(taskID) {
			readyTasks = append(readyTasks, taskID)
		}
	}
	return readyTasks
}
