// internal/storage/postgres/client.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
		INSERT INTO task_executions 
		(id, job_id, task_id, status, retry_count, start_time, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`

	_, err := c.db.ExecContext(ctx, query,
		task.ID,
		task.JobID,
		task.TaskID,
		task.Status,
		task.RetryCount,
		task.StartTime,
	)
	return err
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
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("task not found")
	}

	return nil
}
