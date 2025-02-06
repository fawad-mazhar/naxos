// internal/models/job.go
package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "PENDING"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusCompleted JobStatus = "COMPLETED"
	JobStatusFailed    JobStatus = "FAILED"
)

// JobDefinition represents a job template with its tasks and dependencies
type JobDefinition struct {
	ID    string              `json:"id" yaml:"id"`
	Name  string              `json:"name" yaml:"name"`
	Tasks map[string]Task     `json:"tasks" yaml:"tasks"`
	Graph map[string][]string `json:"graph" yaml:"graph"`
}

// internal/models/job.go update JobExecution struct
type JobExecution struct {
	ID           string                 `json:"id"`
	DefinitionID string                 `json:"definitionId"`
	Status       JobStatus              `json:"status"`
	WorkerID     string                 `json:"workerId"`
	StartTime    time.Time              `json:"startTime"`
	EndTime      *time.Time             `json:"endTime,omitempty"`
	Data         map[string]interface{} `json:"data"`
	TaskStatuses map[string]TaskStatus  `json:"taskStatuses"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// JobExecutionState provides the current state of a job execution
type JobExecutionState struct {
	ID           string      `json:"id"`
	DefinitionID string      `json:"definitionId"`
	Status       JobStatus   `json:"status"`
	StartTime    time.Time   `json:"startTime"`
	Tasks        []TaskState `json:"tasks"`
}

// ValidationError represents an error during job definition validation
type ValidationError struct {
	Field   string
	Message string
}

// NewJobExecution creates a new job execution instance
func NewJobExecution(definitionID string, data map[string]interface{}) *JobExecution {
	now := time.Now()
	return &JobExecution{
		ID:           uuid.New().String(),
		DefinitionID: definitionID,
		Status:       JobStatusPending,
		StartTime:    now,
		Data:         data,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// validateJobDefinition performs comprehensive validation of a job definition
func validateJobDefinition(jobDef *JobDefinition) error {
	// Basic validation
	if jobDef.ID == "" {
		return &ValidationError{Field: "id", Message: "must not be empty"}
	}
	if jobDef.Name == "" {
		return &ValidationError{Field: "name", Message: "must not be empty"}
	}
	if len(jobDef.Tasks) == 0 {
		return &ValidationError{Field: "tasks", Message: "must have at least one task"}
	}

	// Validate individual tasks
	for taskID, task := range jobDef.Tasks {
		if err := validateTask(taskID, &task); err != nil {
			return err
		}
	}

	// Validate task graph
	if err := validateTaskGraph(jobDef); err != nil {
		return err
	}

	return nil
}

// validateTask checks individual task configuration
func validateTask(taskID string, task *Task) error {
	if task.ID != taskID {
		return &ValidationError{
			Field:   fmt.Sprintf("tasks.%s.id", taskID),
			Message: "task ID must match map key",
		}
	}
	if task.Name == "" {
		return &ValidationError{
			Field:   fmt.Sprintf("tasks.%s.name", taskID),
			Message: "must not be empty",
		}
	}
	if task.MaxRetry < 0 {
		return &ValidationError{
			Field:   fmt.Sprintf("tasks.%s.maxRetry", taskID),
			Message: "must be non-negative",
		}
	}
	if task.FunctionName == "" {
		return &ValidationError{
			Field:   fmt.Sprintf("tasks.%s.functionName", taskID),
			Message: "must not be empty",
		}
	}
	return nil
}

// validateTaskGraph validates the task dependency graph
func validateTaskGraph(jobDef *JobDefinition) error {
	// Check that all tasks in graph exist in tasks map
	for taskID, deps := range jobDef.Graph {
		if _, exists := jobDef.Tasks[taskID]; !exists {
			return &ValidationError{
				Field:   "graph",
				Message: fmt.Sprintf("task %s in graph not found in tasks", taskID),
			}
		}

		// Check that all dependencies exist
		for _, depID := range deps {
			if _, exists := jobDef.Tasks[depID]; !exists {
				return &ValidationError{
					Field:   fmt.Sprintf("graph.%s", taskID),
					Message: fmt.Sprintf("dependency %s not found in tasks", depID),
				}
			}
		}
	}

	// Check that all tasks in tasks map are in graph
	for taskID := range jobDef.Tasks {
		if _, exists := jobDef.Graph[taskID]; !exists {
			return &ValidationError{
				Field:   "graph",
				Message: fmt.Sprintf("task %s not found in graph", taskID),
			}
		}
	}

	// Check for circular dependencies
	if err := detectCircularDependencies(jobDef); err != nil {
		return err
	}

	return nil
}

// detectCircularDependencies uses DFS to find cycles in the task graph
func detectCircularDependencies(jobDef *JobDefinition) error {
	visited := make(map[string]bool)
	path := make(map[string]bool)

	var dfs func(string) error
	dfs = func(taskID string) error {
		visited[taskID] = true
		path[taskID] = true

		for _, depID := range jobDef.Graph[taskID] {
			if !visited[depID] {
				if err := dfs(depID); err != nil {
					return err
				}
			} else if path[depID] {
				return &ValidationError{
					Field:   "graph",
					Message: fmt.Sprintf("circular dependency detected involving task %s", depID),
				}
			}
		}

		path[taskID] = false
		return nil
	}

	// Run DFS from each unvisited node
	for taskID := range jobDef.Tasks {
		if !visited[taskID] {
			if err := dfs(taskID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateAndNormalize validates and normalizes a job definition
// Returns the normalized job definition and any validation errors
func ValidateAndNormalize(jobDef *JobDefinition) (*JobDefinition, error) {
	// First validate the job definition
	if err := validateJobDefinition(jobDef); err != nil {
		return nil, err
	}

	// Create a copy for normalization
	normalized := *jobDef

	// Ensure all tasks have default values set
	for taskID, task := range normalized.Tasks {
		if task.MaxRetry < 0 {
			task.MaxRetry = 0
		}
		normalized.Tasks[taskID] = task
	}

	// Sort dependencies to ensure consistent ordering
	for taskID, deps := range normalized.Graph {
		if len(deps) > 1 {
			sortedDeps := make([]string, len(deps))
			copy(sortedDeps, deps)
			// Simple string sort for consistency
			for i := 0; i < len(sortedDeps)-1; i++ {
				for j := i + 1; j < len(sortedDeps); j++ {
					if sortedDeps[i] > sortedDeps[j] {
						sortedDeps[i], sortedDeps[j] = sortedDeps[j], sortedDeps[i]
					}
				}
			}
			normalized.Graph[taskID] = sortedDeps
		}
	}

	return &normalized, nil
}
