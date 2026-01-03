# When Each Response is Triggered

## 📊 Response Flow Diagram

```
                    Request Arrives
                         │
                         ▼
            ┌────────────────────────┐
            │ Rate Limiter Check     │
            │ (MaxTokens > 0?)       │
            └────────┬───────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
        ▼                         ▼
   No Tokens                  Has Tokens
        │                         │
        ▼                         ▼
┌───────────────┐        ┌──────────────────┐
│ 429 Response  │        │ Try Submit Job    │
│ "Rate limit   │        │ to Channel        │
│  exceeded"    │        └────────┬──────────┘
└───────────────┘                 │
                                   │
                    ┌──────────────┴──────────────┐
                    │                           │
                    ▼                           ▼
            Channel Has Space            Channel Full
                    │                           │
                    ▼                           ▼
            ┌───────────────┐         ┌──────────────────┐
            │ Job Accepted │         │ 503 Response     │
            │ Processed     │         │ "Server busy,    │
            │ Success!      │         │  try again later"│
            └───────────────┘         └──────────────────┘
```

## 🔍 Detailed Scenarios

### Scenario 1: Rate Limit Exceeded (429)

**Trigger Condition:**
```go
if s.reqLimiter.MaxTokens > 0 && !s.reqLimiter.IsReqAllowed()
```

**When It Happens:**
- Rate limiter is configured (`token_limit > 0` in config)
- Token bucket is empty (all tokens consumed)
- New request arrives before tokens refill

**Example:**
```yaml
# config.yaml
token_limit: 5
token_rate: 2  # 2 tokens per second
```

```bash
# Send 6 requests rapidly (within 1 second)
for i in {1..6}; do curl -s http://localhost:8080 & done

# Output:
# Requests 1-5: "Hello world !" (5 tokens consumed)
# Request 6: "Rate limit exceeded" (no tokens left)
```

**Timeline:**
```
Time 0.0s: Request 1 → Token available → ✅ Processed
Time 0.0s: Request 2 → Token available → ✅ Processed
Time 0.0s: Request 3 → Token available → ✅ Processed
Time 0.0s: Request 4 → Token available → ✅ Processed
Time 0.0s: Request 5 → Token available → ✅ Processed
Time 0.0s: Request 6 → No tokens! → ❌ 429 "Rate limit exceeded"
Time 0.5s: 1 token refilled (rate: 2/sec)
Time 1.0s: 2 tokens refilled
```

---

### Scenario 2: Server Busy (503) - Queue Full

**Trigger Condition:**
```go
select {
case s.JobChan <- job:
    // Success
default:  // ← Channel is FULL
    // Send 503
}
```

**When It Happens:**
- All workers are busy processing jobs
- Channel buffer is full (queue is full)
- New request arrives

**Example:**
```yaml
# config.yaml
workers: 2
queue_size: 5
# Total capacity = 2 + 5 = 7 jobs
```

```bash
# Send 10 requests rapidly
for i in {1..10}; do curl -s http://localhost:8080 & done

# Output:
# Requests 1-7: "Hello world !" (accepted)
# Requests 8-10: "Server busy, try again later" (rejected)
```

**Visual State:**
```
┌─────────────────────────────────────────┐
│ Worker Pool State (Full)                │
├─────────────────────────────────────────┤
│ Worker 1: Processing Job 1            │ ← Active
│ Worker 2: Processing Job 2            │ ← Active
│                                         │
│ Channel Queue (5 slots):                │
│ [Job 3] [Job 4] [Job 5] [Job 6] [Job 7]│ ← Full
└─────────────────────────────────────────┘

Request 8 arrives → Channel FULL → 503 "Server busy"
```

**Timeline:**
```
Time 0.0s: Request 1 → Accepted → Worker 1 processes
Time 0.0s: Request 2 → Accepted → Worker 2 processes
Time 0.0s: Request 3 → Accepted → Queued
Time 0.0s: Request 4 → Accepted → Queued
Time 0.0s: Request 5 → Accepted → Queued
Time 0.0s: Request 6 → Accepted → Queued
Time 0.0s: Request 7 → Accepted → Queued
Time 0.0s: Request 8 → Channel FULL → ❌ 503 "Server busy"
Time 0.0s: Request 9 → Channel FULL → ❌ 503 "Server busy"
Time 0.0s: Request 10 → Channel FULL → ❌ 503 "Server busy"
```

---

### Scenario 3: Server Shutting Down (503)

**Trigger Condition:**
```go
defer func() {
    if r := recover(); r != nil {  // ← Panic from closed channel
        // Send "Server shutting down"
    }
}()
```

**When It Happens:**
- `s.Close()` is called (graceful shutdown)
- `WorkerPool.Close()` closes the channel (`close(JobChan)`)
- New request arrives and tries to send to closed channel
- Sending to closed channel **panics**
- `recover()` catches panic and sends response

**Example:**
```bash
# Terminal 1: Start server
$ go run cmd/main.go
2026/01/04 01:00:00 Starting server on localhost:8080

# Terminal 2: Send request
$ curl http://localhost:8080 &
[1] 12345

# Terminal 1: Press Ctrl+C (shutdown)
^C
# Server calls s.Close()
# Channel is closed
# Request in Terminal 2 receives: "Server shutting down"
```

**Timeline:**
```
Time 0.0s: Server running normally
Time 1.0s: Client sends request
Time 1.1s: Server accepts connection
Time 1.2s: Server tries: s.JobChan <- job
Time 1.3s: ⚠️ Someone calls s.Close()
Time 1.4s: WorkerPool.Close() → close(JobChan)
Time 1.5s: 💥 PANIC! (sending to closed channel)
Time 1.6s: 🛡️ recover() catches panic
Time 1.7s: ✅ Sends "Server shutting down" response
Time 1.8s: Connection closed
```

**Code Flow:**
```go
// In main or signal handler:
s.Close()  // Called during shutdown
  ↓
s.Listener.Close()  // Stop accepting
  ↓
s.WorkerPool.Close()
  ↓
close(w.JobChan)  // Channel closed!
  ↓
// Meanwhile, in handleRequests():
select {
case s.JobChan <- job:  // 💥 PANIC! Channel is closed
}
  ↓
recover() catches panic
  ↓
Sends "Server shutting down" response
```

---

## 🎯 Summary Table

| Response | Status Code | When Triggered | Location |
|----------|-------------|----------------|----------|
| **Rate limit exceeded** | 429 | No tokens available | `server.go:66` |
| **Server busy** | 503 | Channel/queue full | `server.go:93` |
| **Server shutting down** | 503 | Channel closed (shutdown) | `server.go:80` |
| **Hello world !** | 200 | Request processed successfully | `worker.go:60` |

---

## 🧪 How to Test Each Scenario

### Test 1: Rate Limiting
```bash
# Set low limits in config.yaml
token_limit: 3
token_rate: 1

# Send 5 rapid requests
for i in {1..5}; do curl -s http://localhost:8080; done

# Expected: First 3 succeed, last 2 get "Rate limit exceeded"
```

### Test 2: Queue Full
```bash
# Set low capacity in config.yaml
workers: 1
queue_size: 2
# Total = 3 jobs

# Send 5 rapid requests
for i in {1..5}; do curl -s http://localhost:8080 & done
wait

# Expected: First 3 succeed, last 2 get "Server busy"
```

### Test 3: Server Shutting Down
```bash
# Terminal 1: Start server
go run cmd/main.go

# Terminal 2: Send request and immediately kill server
curl http://localhost:8080 &
# In Terminal 1: Press Ctrl+C

# Expected: "Server shutting down" response
```

---

## 🔑 Key Points

1. **429 Rate Limit**: Happens when token bucket is empty
2. **503 Server Busy**: Happens when worker pool is at capacity
3. **503 Shutting Down**: Happens when channel is closed during shutdown
4. **All use 503**: Both "busy" and "shutting down" use 503, but different messages
5. **Non-blocking**: "Server busy" uses `select` with `default` to never block
6. **Panic Recovery**: "Server shutting down" uses `recover()` to handle closed channel gracefully

---

## 💡 Why These Responses Matter

- **429 Rate Limit**: Protects server from overload
- **503 Server Busy**: Prevents queue from growing indefinitely
- **503 Shutting Down**: Graceful shutdown - tells clients server is closing

All three prevent the server from hanging or crashing under different conditions!

