// internal/models/status.go
package models

import (
	"time"
)

// StatusMessage represents a status update messages for jobs, tasks and runner
type StatusMessage struct {
	Type      string      `json:"type"`      // "runner", "job", or "task"
	ID        string      `json:"id"`        // unique identifier of the entity (runner/job/task id)
	Status    string      `json:"status"`    // current status of the entity
	Timestamp time.Time   `json:"timestamp"` // when the status was updated
	Metadata  interface{} `json:"metadata"`  // additional entity-specific information
}

// SystemState represents the current state of the entire system
type SystemState struct {
	ActiveJobs   []JobExecutionState `json:"activeJobs"`
	QueuedCount  int                 `json:"queuedCount"`
	ExecutedJobs int                 `json:"executedJobs"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

type RunnerEventType string

const (
	RunnerStarted  RunnerEventType = "STARTED"
	RunnerStopping RunnerEventType = "STOPPING"
	RunnerStopped  RunnerEventType = "STOPPED"
	RunnerHealthy  RunnerEventType = "HEALTHY"
)

type RunnerStatus struct {
	ID          string          `json:"id"`
	Event       RunnerEventType `json:"event"`
	Timestamp   time.Time       `json:"timestamp"`
	WorkerCount int             `json:"workerCount"`
	ActiveJobs  int             `json:"activeJobs"`
}
