// migrations/postgres/001_initial_schema.sql

-- Initial schema for naxos workflow system
-- Creates core tables for job definitions, job executions, and task executions


CREATE TABLE job_definitions (
    id VARCHAR PRIMARY KEY,
    name VARCHAR NOT NULL,
    tasks JSONB NOT NULL,
    graph JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE job_executions (
    id VARCHAR PRIMARY KEY,
    definition_id VARCHAR NOT NULL REFERENCES job_definitions(id),
    status VARCHAR NOT NULL,
    worker_id VARCHAR,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    data JSONB,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE task_executions (
    id VARCHAR PRIMARY KEY,
    job_id VARCHAR NOT NULL REFERENCES job_executions(id),
    task_id VARCHAR NOT NULL,
    status VARCHAR NOT NULL,
    error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_job_executions_status ON job_executions(status);
CREATE INDEX idx_task_executions_job_id ON task_executions(job_id);