# Naxos - Distributed Job Orchestration System

---

Fawad Mazhar <fawadmazhar@hotmail.com> 2024-2026

---

> ⚠️ **Development Status**: This repository is currently under active development. Features and APIs may change without notice.

Naxos is a lightweight distributed job orchestration system written in Go that manages the execution of task sequences with persistent state handling and a RESTful API interface. It supports both sequential and parallel task execution with dependency management.

## Features

- **Job Orchestration**: Define and execute sequences of tasks with dependencies
- **Persistent Storage**: State management using PostgreSQL
- **Local Caching**: Fast job definition retrieval using LevelDB
- **Message Queue**: NATS JetStream for reliable job distribution with queue groups
- **Load Balancing**: Durable consumers with queue groups distribute jobs evenly across runners
- **State Recovery**: Automatic recovery of interrupted jobs after system restart
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
      nats[(NATS JetStream)]
    end

    subgraph "Runner Layer"
      subgraph "Runner Node 1"
        runner1[Runner 1]
        ldb1[(LevelDB 1)]
      end
      
      subgraph "Runner Node 2"
        runner2[Runner 2]
        ldb2[(LevelDB 2)]
      end

      subgraph "Runner Node N"
        runnerN[Runner N]
        ldbN[(LevelDB N)]
      end
    end

    client -->|HTTP| api
    api -->|Store/Query| pg
    api -->|Publish Jobs| nats
    
    runner1 -->|Cache| ldb1
    runner2 -->|Cache| ldb2
    runnerN -->|Cache| ldbN
    
    runner1 -->|Consume Jobs<br/>Queue Group| nats
    runner2 -->|Consume Jobs<br/>Queue Group| nats
    runnerN -->|Consume Jobs<br/>Queue Group| nats
    runner1 & runner2 & runnerN -->|Read/Write| pg
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
- Go 1.24 or later (for development)

### Running with Docker Compose
1. Clone the repository
```bash
git clone https://github.com/fawad-mazhar/naxos.git
cd naxos
```

2. Start the system
```bash
docker compose up -d --build
```

This starts PostgreSQL, NATS (with JetStream), the API server, and 4 runner instances.

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `NAXOS_POSTGRES_URL` | Yes | - | PostgreSQL connection URL |
| `NAXOS_NATS_URL` | Yes | - | NATS server URL |
| `NAXOS_SERVER_PORT` | No | `8080` | API server port |
| `NAXOS_NATS_STREAM_NAME` | No | `NAXOS_JOBS` | JetStream stream name |
| `NAXOS_NATS_SUBJECT` | No | `naxos.jobs` | NATS subject for jobs |
| `NAXOS_NATS_QUEUE_GROUP` | No | `naxos-runners` | Consumer queue group name |
| `NAXOS_LEVELDB_PATH` | No | `./data/leveldb` | LevelDB cache path |
| `NAXOS_WORKER_SHUTDOWN_TIMEOUT` | No | `30` | Shutdown timeout in seconds |

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
curl -X POST http://localhost:8080/api/v1/jobs/sample-1/execute \
  -H "Content-Type: application/json" \
  -d '{"param": "value"}'
```

Check job status:
```bash
curl http://localhost:8080/api/v1/jobs/{execution-id}/status
```

### Building
```bash
go build -o bin/api cmd/api/main.go
go build -o bin/runner cmd/runner/main.go
```

### Load Testing
A load test script is included to verify job distribution across runners:
```bash
./scripts/test_load.sh http://localhost:8080 150
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
