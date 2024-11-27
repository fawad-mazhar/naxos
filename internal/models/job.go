// internal/models/job.go
package models

import (
	"encoding/json"
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
	ID    string              `json:"id"`
	Name  string              `json:"name"`
	Tasks map[string]Task     `json:"tasks"`
	Graph map[string][]string `json:"graph"` // key: taskID, value: dependency task IDs
}

// JobExecution represents a single instance of a job being executed
type JobExecution struct {
	ID           string                 `json:"id"`
	DefinitionID string                 `json:"definitionId"`
	Status       JobStatus              `json:"status"`
	WorkerID     string                 `json:"workerId"`
	StartTime    time.Time              `json:"startTime"`
	EndTime      *time.Time             `json:"endTime,omitempty"`
	Data         map[string]interface{} `json:"data"`
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

// ToJSON converts the job execution to JSON
func (je *JobExecution) ToJSON() ([]byte, error) {
	return json.Marshal(je)
}

// FromJSON populates the job execution from JSON
func (je *JobExecution) FromJSON(data []byte) error {
	return json.Unmarshal(data, je)
}
