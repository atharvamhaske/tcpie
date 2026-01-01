# Worker Pool Architecture Explanation

> **Note**: This document contains Mermaid diagrams. For best viewing:
> - GitHub/GitLab: Diagrams render automatically
> - VS Code: Install "Markdown Preview Mermaid Support" extension
> - Other viewers: May need Mermaid.js support enabled

## Overview

This file implements a **worker pool pattern** using Go's concurrency primitives (goroutines and channels) to handle multiple network connections concurrently. It's designed to process HTTP-like requests efficiently by distributing work across a pool of worker goroutines.

---

## Core Components

### 1. **Job Structure**
```go
type Job struct {
    Id   int
    Conn net.Conn
}
```
- Represents a single task/request
- Contains a unique ID and a network connection to process

### 2. **WorkerPool Structure**
```go
type WorkerPool struct {
    MaxWorkers int      // Maximum concurrent workers
    QueueSize  int      // Buffer size for queued jobs
    JobChan    chan Job // Buffered channel for job distribution
    wg         *sync.WaitGroup // Synchronization primitive
}
```

---

## Architecture Diagram

```mermaid
graph TB
    Client[Client Connections]
    JobQueue[Job Channel Buffered Channel]
    W1[Worker 1 Goroutine]
    W2[Worker 2 Goroutine]
    W3[Worker N Goroutine]
    WG[WaitGroup Synchronization]
    
    Client -->|SubmitJob| JobQueue
    JobQueue -->|Distribute| W1
    JobQueue -->|Distribute| W2
    JobQueue -->|Distribute| W3
    W1 -->|Done| WG
    W2 -->|Done| WG
    W3 -->|Done| WG
```

---

## Concurrency Model: Channels

### How Channels Work Here

**Buffered Channel:**
```go
JobChan = make(chan Job, w.MaxWorkers + w.QueueSize)
```

- **Type**: Buffered channel (can hold `MaxWorkers + QueueSize` jobs)
- **Purpose**: Acts as a **job queue** and **synchronization mechanism**
- **Behavior**:
  - If channel has space → job is added immediately (non-blocking)
  - If channel is full → `SubmitJob` blocks until a worker takes a job
  - Workers block on `range w.JobChan` until a job arrives

### Channel Flow Diagram

```mermaid
sequenceDiagram
    participant Server
    participant Channel as Job Channel Buffered
    participant W1 as Worker 1
    participant W2 as Worker 2
    
    Note over Channel: Initial Empty Capacity = MaxWorkers + QueueSize
    
    Server->>Channel: SubmitJob Job1
    Channel->>W1: Job1 Worker picks up
    
    Server->>Channel: SubmitJob Job2
    Channel->>W2: Job2 Worker picks up
    
    Server->>Channel: SubmitJob Job3
    Note over Channel: Job3 queued if workers busy
    
    W1->>Channel: Finished Job1
    Channel->>W1: Job3 Worker picks up
    
    Note over Channel: Pattern repeats
```

---

## Worker Function Deep Dive

### Code Structure
```go
func (w *WorkerPool) worker(workerId int) {
    processRequests := func(j Job) {
        // Read request from connection
        request := make([]byte, 1024)
        j.Conn.Read(request)
        
        // Send response
        response := []byte("HTTP/1.1 200 OK\r\n\r\n Hello world ! \r\n")
        j.Conn.Write(response)
        j.Conn.Close()
    }

    // Main worker loop
    for job := range w.JobChan {
        log.Printf("Worker %d, processing request %d", workerId, job.Id)
        processRequests(job)
    }

    w.wg.Done()
}
```

### Worker Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Created: NewWorkerPool called
    Created --> Running: goroutine started
    Running --> Waiting: No jobs in channel
    Waiting --> Processing: Job received from channel
    Processing --> Waiting: Job completed
    Waiting --> Terminated: Channel closed
    Terminated --> [*]: wg.Done called
```

### Worker Execution Flow

```mermaid
flowchart TD
    Start([Worker Goroutine Starts]) --> Loop{Channel Has Job?}
    Loop -->|Yes| Receive[Receive Job from Channel]
    Loop -->|No| Block[Block Waiting]
    Block -->|Job Arrives| Receive
    Receive --> Log[Log Worker X processing Job Y]
    Log --> Read[Read Request from Connection]
    Read --> Process[Process Request]
    Process --> Write[Write Response]
    Write --> Close[Close Connection]
    Close --> Loop
    Loop -->|Channel Closed| Done[Call wg.Done]
    Done --> End([Worker Terminates])
```

---

## Worker Pool Lifecycle

### Initialization Phase

```mermaid
sequenceDiagram
    participant Main
    participant WP as WorkerPool
    participant WG as WaitGroup
    participant W1 as Worker 1
    participant W2 as Worker 2
    participant WN as Worker N
    
    Main->>WP: NewWorkerPool
    WP->>WG: new sync.WaitGroup
    WP->>WP: Create buffered channel MaxWorkers + QueueSize
    
    loop For each worker 0 to MaxWorkers-1
        WP->>WG: wg.Add 1
        WP->>W1: go worker i
        Note over W1: Worker starts blocks on channel
    end
    
    Note over WP,W1: All workers ready waiting for jobs
```

### Job Submission Flow

```mermaid
flowchart LR
    A[Server Receives Connection] --> B[Create Job with ID and Conn]
    B --> C[SubmitJob]
    C --> D{Channel Full?}
    D -->|No| E[Add to Channel]
    D -->|Yes| F[Block Until Space]
    F --> E
    E --> G[Idle Worker Receives]
    G --> H[Process Job]
```

### Shutdown Phase

```mermaid
sequenceDiagram
    participant Main
    participant WP as WorkerPool
    participant Channel as Job Channel
    participant W1 as Worker 1
    participant W2 as Worker 2
    participant WG as WaitGroup
    
    Main->>WP: Close
    WP->>Channel: close JobChan
    Note over Channel: Channel closed no more jobs accepted
    Channel->>W1: Signal channel closed
    Channel->>W2: Signal channel closed
    W1->>W1: Finish current job
    W2->>W2: Finish current job
    W1->>WG: wg.Done
    W2->>WG: wg.Done
    WP->>WG: wg.Wait
    Note over WG: Blocks until all workers done
    WG->>Main: All workers terminated
```

---

## Concurrency Patterns Used

### 1. **Producer-Consumer Pattern**
- **Producer**: `SubmitJob()` adds jobs to channel
- **Consumers**: Worker goroutines consume jobs from channel
- **Synchronization**: Channel handles coordination automatically

### 2. **Worker Pool Pattern**
- Fixed number of workers (controlled concurrency)
- Prevents resource exhaustion
- Better than spawning unlimited goroutines

### 3. **Graceful Shutdown**
- `WaitGroup` ensures all workers finish before shutdown
- Channel closure signals workers to stop
- No jobs are lost during shutdown

---

## Key Concurrency Concepts

### Channel Blocking Behavior

```mermaid
graph LR
    S1[Job Sent] --> S2{Channel Full?}
    S2 -->|No| S3[Non-blocking]
    S2 -->|Yes| S4[Block until space]
    
    R1[Worker Waiting] --> R2{Channel Empty?}
    R2 -->|No| R3[Receive job immediately]
    R2 -->|Yes| R4[Block until job arrives]
    
    style S3 fill:#90EE90
    style S4 fill:#FFB6C1
    style R3 fill:#90EE90
    style R4 fill:#FFB6C1
```

### WaitGroup Synchronization

```mermaid
graph TD
    Start[NewWorkerPool] --> Add[wg.Add for each worker]
    Add --> Spawn[Spawn N goroutines]
    Spawn --> Wait[Workers running]
    Close[Close called] --> CloseChan[Close channel]
    CloseChan --> Workers[Workers finish]
    Workers --> Done[Each calls wg.Done]
    Done --> WaitAll[wg.Wait blocks]
    WaitAll --> Complete[All workers done]
```

---

## Example Execution Timeline

```mermaid
gantt
    title Worker Pool Execution Timeline
    dateFormat X
    axisFormat %s
    
    section Worker 1
    Waiting           :0, 2
    Processing Job 1  :2, 3
    Waiting           :5, 1
    Processing Job 3  :6, 3
    
    section Worker 2
    Waiting           :0, 1
    Processing Job 2  :1, 4
    Waiting           :5, 2
    Processing Job 4  :7, 2
    
    section Channel
    Job 1 Queued      :0, 1
    Job 2 Queued      :1, 1
    Job 3 Queued      :5, 1
    Job 4 Queued      :6, 1
```

---

## Benefits of This Design

1. **Controlled Concurrency**: Limits simultaneous connections to `MaxWorkers`
2. **Backpressure**: Queue size limits prevent unbounded memory growth
3. **Efficient Resource Usage**: Reuses goroutines instead of creating new ones per request
4. **Graceful Shutdown**: Ensures all work completes before termination
5. **Load Distribution**: Channel automatically distributes work to available workers

---

## Potential Improvements

1. **Error Handling**: `Conn.Read()` and `Conn.Write()` errors are ignored
2. **Context Support**: Could add context for cancellation
3. **Metrics**: Could track job processing times, queue depth
4. **Timeout Handling**: No timeout for reading/writing connections

