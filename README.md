# Naxos
---

Fawad Mazhar <fawadmazhar@hotmail.com> 2024-2025

---

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

    subgraph "Orchestration Layer"
      orch1[Orchestrator Node 1]
      orch2[Orchestrator Node 2]
      
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
    
    orch1 -->|Spawn| w1
    orch1 -->|Spawn| w2
    orch2 -->|Spawn| w3
    orch2 -->|Spawn| w4
    
    w1 & w2 -->|Cache| ldb1
    w3 & w4 -->|Cache| ldb2
    
    orch1 & orch2 -->|Consume Jobs| rmq
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
