// internal/storage/postgres/client.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fawad-mazhar/naxos/internal/config"
	"github.com/fawad-mazhar/naxos/internal/models"
	_ "github.com/lib/pq"
)

type Client struct {
	db *sql.DB
}

func NewClient(cfg config.PostgresConfig) (*Client, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &Client{db: db}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

// Job related functions

func (c *Client) StoreJobDefinition(ctx context.Context, job *models.JobDefinition) error {
	tasks, err := json.Marshal(job.Tasks)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	graph, err := json.Marshal(job.Graph)
	if err != nil {
		return fmt.Errorf("failed to marshal graph: %w", err)
	}

	query := `
		INSERT INTO job_definitions (id, name, tasks, graph, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
			tasks = EXCLUDED.tasks,
			graph = EXCLUDED.graph,
			updated_at = NOW()`

	_, err = c.db.ExecContext(ctx, query, job.ID, job.Name, tasks, graph)
	return err
}

func (c *Client) GetJobDefinition(ctx context.Context, id string) (*models.JobDefinition, error) {
	query := `
		SELECT id, name, tasks, graph
		FROM job_definitions
		WHERE id = $1`

	var job models.JobDefinition
	var tasksJSON, graphJSON []byte

	err := c.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.Name,
		&tasksJSON,
		&graphJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("job definition not found")
		}
		return nil, err
	}

	if err := json.Unmarshal(tasksJSON, &job.Tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	if err := json.Unmarshal(graphJSON, &job.Graph); err != nil {
		return nil, fmt.Errorf("failed to unmarshal graph: %w", err)
	}

	return &job, nil
}

func (c *Client) CreateJobExecution(ctx context.Context, job *models.JobExecution) error {
	data, err := json.Marshal(job.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal job data: %w", err)
	}

	query := `
		INSERT INTO job_executions 
		(id, definition_id, status, worker_id, start_time, data, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`

	_, err = c.db.ExecContext(ctx, query,
		job.ID,
		job.DefinitionID,
		job.Status,
		job.WorkerID,
		job.StartTime,
		data,
	)
	return err
}

func (c *Client) UpdateJobStatus(ctx context.Context, jobID string, status models.JobStatus) error {
	query := `
		UPDATE job_executions
		SET status = $1, updated_at = NOW()
		WHERE id = $2`

	result, err := c.db.ExecContext(ctx, query, status, jobID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("job not found")
	}

	return nil
}

func (c *Client) ClaimJob(ctx context.Context, jobID, workerID string) (bool, error) {
	query := `
		UPDATE job_executions
		SET status = $1, worker_id = $2, updated_at = NOW()
		WHERE id = $3 AND status = $4
		RETURNING id`

	var id string
	err := c.db.QueryRowContext(ctx, query,
		models.JobStatusRunning,
		workerID,
		jobID,
		models.JobStatusPending,
	).Scan(&id)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Task related functions

func (c *Client) CreateTaskExecution(ctx context.Context, task *models.TaskExecution) error {
	query := `
        INSERT INTO task_executions (
            id, job_id, task_id, status, retry_count, start_time, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`

	_, err := c.db.ExecContext(ctx, query,
		task.ID,
		task.JobID,
		task.TaskID,
		task.Status,
		task.RetryCount,
		task.StartTime,
	)
	if err != nil {
		return fmt.Errorf("failed to create task execution: %w", err)
	}

	return nil
}

func (c *Client) UpdateTaskStatus(ctx context.Context, taskID string, status models.TaskStatus, err error) error {
	var errMsg *string
	if err != nil {
		msg := err.Error()
		errMsg = &msg
	}

	query := `
        UPDATE task_executions
        SET status = $1, 
            error = $2,
            updated_at = NOW(),
            end_time = CASE WHEN $1 IN ($3, $4) THEN NOW() ELSE end_time END
        WHERE id = $5`

	result, err := c.db.ExecContext(ctx, query,
		status,
		errMsg,
		models.TaskStatusCompleted,
		models.TaskStatusFailed,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

// GetInProgressJobs retrieves jobs that are in RUNNING state
func (c *Client) GetInProgressJobs(ctx context.Context) ([]*models.JobExecution, error) {
	query := `
        SELECT id, definition_id, status, worker_id, start_time, data, created_at, updated_at
        FROM job_executions
        WHERE status = $1 AND updated_at < $2
    `
	// Only recover jobs that haven't been updated in the last 5 minutes
	// This helps prevent recovering jobs that are actively being worked on
	staleTimeout := time.Now().Add(-5 * time.Minute)

	rows, err := c.db.QueryContext(ctx, query, models.JobStatusRunning, staleTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to query in-progress jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.JobExecution
	for rows.Next() {
		var job models.JobExecution
		var dataBytes []byte
		err := rows.Scan(
			&job.ID,
			&job.DefinitionID,
			&job.Status,
			&job.WorkerID,
			&job.StartTime,
			&dataBytes,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		if err := json.Unmarshal(dataBytes, &job.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal job data: %w", err)
		}

		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// ClaimStaleJob attempts to claim a stale job for recovery
func (c *Client) ClaimStaleJob(ctx context.Context, jobID string, workerID string) (bool, error) {
	query := `
        UPDATE job_executions
        SET worker_id = $1, 
            updated_at = NOW()
        WHERE id = $2 
        AND status = $3 
        AND updated_at < $4
        RETURNING id
    `

	staleTimeout := time.Now().Add(-5 * time.Minute)
	var id string
	err := c.db.QueryRowContext(ctx, query,
		workerID,
		jobID,
		models.JobStatusRunning,
		staleTimeout,
	).Scan(&id)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to claim stale job: %w", err)
	}

	return true, nil
}

// GetJobExecutionDetails retrieves complete job execution info including tasks
func (c *Client) GetJobExecutionDetails(ctx context.Context, executionID string) (*models.JobExecution, []models.TaskExecution, error) {
	// First get job execution
	query := `
        SELECT id, definition_id, status, worker_id, start_time, end_time, data, created_at, updated_at
        FROM job_executions 
        WHERE id = $1`

	var job models.JobExecution
	var dataBytes []byte

	err := c.db.QueryRowContext(ctx, query, executionID).Scan(
		&job.ID,
		&job.DefinitionID,
		&job.Status,
		&job.WorkerID,
		&job.StartTime,
		&job.EndTime,
		&dataBytes,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get job execution: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &job.Data); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	// Then get all tasks for this job
	taskQuery := `
        SELECT id, job_id, task_id, status, error, retry_count, start_time, end_time, created_at, updated_at
        FROM task_executions 
        WHERE job_id = $1 
        ORDER BY created_at ASC`

	rows, err := c.db.QueryContext(ctx, taskQuery, executionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get task executions: %w", err)
	}
	defer rows.Close()

	var tasks []models.TaskExecution
	for rows.Next() {
		var task models.TaskExecution
		err := rows.Scan(
			&task.ID,
			&task.JobID,
			&task.TaskID,
			&task.Status,
			&task.Error,
			&task.RetryCount,
			&task.StartTime,
			&task.EndTime,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan task execution: %w", err)
		}
		tasks = append(tasks, task)
	}

	return &job, tasks, nil
}

// GetSystemStatus retrieves the current state of the system
func (c *Client) GetSystemStatus(ctx context.Context) (*models.SystemState, error) {
	var status models.SystemState

	// Get queued jobs count
	queuedQuery := `
        SELECT COUNT(*) 
        FROM job_executions 
        WHERE status = $1`

	err := c.db.QueryRowContext(ctx, queuedQuery, models.JobStatusPending).Scan(&status.QueuedCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get queued count: %w", err)
	}

	// Get active jobs
	activeQuery := `
        SELECT id, definition_id, status, start_time
        FROM job_executions 
        WHERE status = $1`

	rows, err := c.db.QueryContext(ctx, activeQuery, models.JobStatusRunning)
	if err != nil {
		return nil, fmt.Errorf("failed to get active jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var job models.JobExecutionState
		err := rows.Scan(
			&job.ID,
			&job.DefinitionID,
			&job.Status,
			&job.StartTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan active job: %w", err)
		}
		status.ActiveJobs = append(status.ActiveJobs, job)
	}

	// Get executed jobs count
	executedQuery := `
        SELECT COUNT(*) 
        FROM job_executions 
        WHERE status = $1`

	err = c.db.QueryRowContext(ctx, executedQuery, models.JobStatusCompleted).Scan(&status.ExecutedJobs)
	if err != nil {
		return nil, fmt.Errorf("failed to get executed count: %w", err)
	}

	status.UpdatedAt = time.Now()
	return &status, nil
}

// GetJobTasks retrieves all tasks for a specific job
func (c *Client) GetJobTasks(ctx context.Context, jobID string) ([]models.TaskExecution, error) {
	query := `
        SELECT id, job_id, task_id, status, error, retry_count, start_time, end_time, created_at, updated_at
        FROM task_executions 
        WHERE job_id = $1 
        ORDER BY created_at ASC`

	rows, err := c.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.TaskExecution
	for rows.Next() {
		var task models.TaskExecution
		err := rows.Scan(
			&task.ID,
			&task.JobID,
			&task.TaskID,
			&task.Status,
			&task.Error,
			&task.RetryCount,
			&task.StartTime,
			&task.EndTime,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
