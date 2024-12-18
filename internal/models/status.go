// internal/models/status.go
package models

import (
	"time"
)

// StatusMessage represents a status update messages for jobs, tasks and orchestrator
type StatusMessage struct {
	Type      string      `json:"type"`      // "orchestrator", "job", or "task"
	ID        string      `json:"id"`        // unique identifier of the entity (orchestrator/job/task id)
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

type OrchestratorEventType string

const (
	OrchestratorStarted  OrchestratorEventType = "STARTED"
	OrchestratorStopping OrchestratorEventType = "STOPPING"
	OrchestratorStopped  OrchestratorEventType = "STOPPED"
	OrchestratorHealthy  OrchestratorEventType = "HEALTHY"
)

type OrchestratorStatus struct {
	ID           string                `json:"id"`
	Event        OrchestratorEventType `json:"event"`
	Timestamp    time.Time             `json:"timestamp"`
	WorkerCount  int                   `json:"workerCount"`
	ActiveJobs   int                   `json:"activeJobs"`
	HealthStatus string                `json:"healthStatus"`
}
