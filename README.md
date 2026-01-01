# TCP Server from Scratch (tcpie)

A high-performance TCP server built from scratch in Go featuring concurrent request handling via worker pools, token bucket rate limiting, and Prometheus metrics integration.

## Quick Start

### 1. Run the Server

```bash
# From project root
go run cmd/main.go
```

### 2. Test the TCP Server

**Using curl:**

```bash
curl http://localhost:8080
```

### 3. Check Metrics

**Raw Prometheus Format (for scraping):**
```bash
curl http://localhost:9090/metrics
```

## Project Structure

```
tcpie/
├── cmd/
│   └── main.go              # Application entry point
├── internals/
│   ├── config/
│   │   ├── config.go        # Config structs
│   │   └── config.yaml      # Configuration file
│   ├── metrics/
│   │   └── metrics.go       # Prometheus metrics
│   ├── rate-limiter/
│   │   └── rate-limiter.go  # Token bucket rate limiter
│   ├── server.go            # TCP server implementation
│   └── worker.go            # Worker pool implementation
└── README.md               # This file
```

## Testing the Server

1. **Start the server:**
   ```bash
   go run cmd/main.go
   ```

2. **In another terminal, test with curl:**
   ```bash
   curl http://localhost:8080
   ```
   Expected response: `HTTP/1.1 200 OK\r\n\r\n Hello world ! \r\n`

3. **Check metrics:**
   ```bash
   curl http://localhost:9090/metrics | grep total_requests
   ```

4. **Test rate limiting:**
   ```bash
   # Send multiple rapid requests
   for i in {1..20}; do curl http://localhost:8080 & done
   ```
