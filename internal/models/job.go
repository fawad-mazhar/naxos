// internal/models/job.go
package models

import (
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

// JobExecutionState provides the current state of a job execution
type JobExecutionState struct {
	ID           string      `json:"id"`
	DefinitionID string      `json:"definitionId"`
	Status       JobStatus   `json:"status"`
	StartTime    time.Time   `json:"startTime"`
	Tasks        []TaskState `json:"tasks"`
}
