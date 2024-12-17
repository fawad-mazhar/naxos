// internal/models/task.go
package models

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "PENDING"
	TaskStatusRunning   TaskStatus = "RUNNING"
	TaskStatusCompleted TaskStatus = "COMPLETED"
	TaskStatusFailed    TaskStatus = "FAILED"
)

// Task represents a single unit of work within a job
type Task struct {
	ID           string `json:"id" yaml:"id"`
	Name         string `json:"name" yaml:"name"`
	MaxRetry     int    `json:"maxRetry" yaml:"maxRetry"`
	FunctionName string `json:"functionName" yaml:"functionName"`
}

// TaskExecution represents a single instance of a task being executed
type TaskExecution struct {
	ID         string     `json:"id"`
	JobID      string     `json:"jobId"`
	TaskID     string     `json:"taskId"`
	Status     TaskStatus `json:"status"`
	Error      *string    `json:"error,omitempty"`
	StartTime  time.Time  `json:"startTime"`
	EndTime    *time.Time `json:"endTime,omitempty"`
	RetryCount int        `json:"retryCount"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// TaskState represents the current state of a task in a job
type TaskState struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Status TaskStatus `json:"status"`
}

// NewTaskExecution creates a new task execution instance
func NewTaskExecution(jobID string, taskID string) *TaskExecution {
	now := time.Now()
	return &TaskExecution{
		ID:        uuid.New().String(),
		JobID:     jobID,
		TaskID:    taskID,
		Status:    TaskStatusPending,
		StartTime: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
