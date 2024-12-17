// internal/models/status.go
package models

import (
	"time"
)

// StatusMessage represents a status update message for jobs and tasks
type StatusMessage struct {
	Type      string      `json:"type"`      // "job" or "task"
	ID        string      `json:"id"`        // job or task ID
	JobID     string      `json:"jobId"`     // parent job ID for tasks
	Status    string      `json:"status"`    // current status
	Error     string      `json:"error"`     // error message if any
	Timestamp time.Time   `json:"timestamp"` // when the status was updated
	Metadata  interface{} `json:"metadata"`  // additional status information
}

// SystemState represents the current state of the entire system
type SystemState struct {
	ActiveJobs   []JobExecutionState `json:"activeJobs"`
	QueuedCount  int                 `json:"queuedCount"`
	ExecutedJobs int                 `json:"executedJobs"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

// NewStatusMessage creates a new status message
func NewStatusMessage(msgType string, id string, jobID string, status string) *StatusMessage {
	return &StatusMessage{
		Type:      msgType,
		ID:        id,
		JobID:     jobID,
		Status:    status,
		Timestamp: time.Now(),
	}
}
