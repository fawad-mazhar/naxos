# Naxos - Distributed Job Orchestration System

---

Fawad Mazhar <fawadmazhar@hotmail.com> 2024-2025

---

> ⚠️ **Development Status**: This repository is currently under active development (as of January 2025). Features and APIs may change without notice.

Naxos is a lightweight distributed job orchestration system written in Go that manages the execution of task sequences with persistent state handling and a RESTful API interface. It supports both sequential and parallel task execution with dependency management.


## Features

- **Job Orchestration**: Define and execute sequences of tasks with dependencies
- **Persistent Storage**: State management using PostgreSQL
- **Local Caching**: Fast job definition retrieval using LevelDB
- **Message Queue**: RabbitMQ for job distribution and status updates
- **Concurrent Execution**: Configurable worker pool for parallel job processing
- **State Recovery**: Automatic recovery of interrupted jobs after system restart
- **Health Monitoring**: System and orchestrator health checks
- **RESTful API**: HTTP interface for job management and monitoring

## System Design
<details>
  <summary>Naxos Architecture</summary>
  
  ```mermaid
  flowchart TB
    subgraph "API Layer"
      client[Client]
      api[API Server]
    end

    subgraph "Storage Layer"
      pg[(PostgreSQL)]
      rmq[(RabbitMQ)]
    end

    subgraph "Runner Layer"
      runner1[Runner Node 1]
      runner2[Runner Node 2]
      
      subgraph "Workers (Node 1)"
        w1[Worker 1]
        w2[Worker 2]
        ldb1[(LevelDB 1)]
      end
      
      subgraph "Workers (Node 2)"
        w3[Worker 1]
        w4[Worker 2]
        ldb2[(LevelDB 2)]
      end
    end

    client -->|HTTP| api
    api -->|Store/Query| pg
    api -->|Enqueue Jobs| rmq
    
    runner1 -->|Spawn| w1
    runner1 -->|Spawn| w2
    runner2 -->|Spawn| w3
    runner2 -->|Spawn| w4
    
    w1 & w2 -->|Cache| ldb1
    w3 & w4 -->|Cache| ldb2
    
    runner1 -->|Consume Jobs| rmq
    runner2 -->|Consume Jobs| rmq
    w1 & w2 & w3 & w4 -->|Read Job Definitions| pg
  ```
</details>

<details>
  <summary>Parallel Task Execution</summary>

  ```mermaid
  graph TD
    task1[Initial Task<br/>MaxRetry: 3<br/>Function: Task1]
    task2[Parallel Task A<br/>MaxRetry: 2<br/>Function: Task2]
    task3[Parallel Task B<br/>MaxRetry: 2<br/>Function: Task3]
    task4[Final Task<br/>MaxRetry: 1<br/>Function: Task4]

    task1 --> task2
    task1 --> task3
    task2 --> task4
    task3 --> task4

    style task1 fill:#fff,stroke:#333,stroke-width:1px
    style task2 fill:#fff,stroke:#333,stroke-width:1px
    style task3 fill:#fff,stroke:#333,stroke-width:1px
    style task4 fill:#fff,stroke:#333,stroke-width:1px
  ```
  This diagram represents a parallel task execution flow where:
  1. `Initial Task` runs first
  2. `Parallel Task A` and `Parallel Task B` run in parallel after `Initial Task` completes
  3. `Final Task` runs only after both parallel tasks complete
  4. Each task has its own retry policy defined by `maxRetry`
</details>

<details>
  <summary>Sequential Task Execution</summary>

  ```mermaid
  graph TD
    task1[First Task<br/>MaxRetry: 3<br/>Function: Task1]
    task2[Second Task<br/>MaxRetry: 2<br/>Function: Task2]
    task3[Third Task<br/>MaxRetry: 1<br/>Function: Task3]

    task1 --> task2
    task2 --> task3

    style task1 fill:#fff,stroke:#333,stroke-width:1px
    style task2 fill:#fff,stroke:#333,stroke-width:1px
    style task3 fill:#fff,stroke:#333,stroke-width:1px
  ```
  This diagram represents a sequential task execution flow where:
  1. `First Task` runs first with 3 retry attempts
  2. `Second Task` starts after successful completion of First Task with 2 retry attempts
  3. `Third Task` executes last with 1 retry attempt
  4. Each task must complete successfully before the next task begins
</details>

## Getting Started

### Prerequisites
- Docker and Docker Compose
- Go 1.21 or later (for development)

### Running with Docker Compose
1. Clone the repository
```bash
git clone https://github.com/fawad-mazhar/naxos.git
cd naxos
```

2. Start the system
```bash
docker-compose up -d --build
```

## API Endpoints

### Job Management
- `POST /api/v1/job-definitions` - Register new job definition
- `GET /api/v1/job-definitions/{id}` - Get job definition
- `POST /api/v1/jobs/{id}/execute` - Execute a job
- `GET /api/v1/jobs/{id}/status` - Get job execution status

### System Status
- `GET /api/v1/system/status` - Get system status
- `GET /health` - Health check endpoint

## Example Usage

Execute a job:
```bash
curl -X POST http://localhost:8080/api/v1/jobs/sample-1/execute -d '{"param": "value"}'
```

Check job status:
```bash
curl http://localhost:8080/api/v1/jobs/{execution-id}/status
```

### Building
```bash
go build -o bin/api cmd/api/main.go
go build -o bin/orchestrator cmd/orchestrator/main.go
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.