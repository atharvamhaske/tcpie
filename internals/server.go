package server

import (
	"fmt"
	"log"
	"net"
	"sync/atomic"

	"github.com/atharvamhaske/tcpie/internals/metrics"
	ratelimiter "github.com/atharvamhaske/tcpie/internals/rate-limiter"
)

// for accepting tcp connections
type Server struct {
	WorkerPool
	Port       int
	URL        string
	Opts       ServerOpts
	Metrics    metrics.ServerMetrics
	Listener   net.Listener
	reqLimiter ratelimiter.TokenBucket
}

type ServerOpts struct {
	Rate       int64
	Tokens     int64
	MaxThreads int
	QueueSize  int
}

// createListener creates a TCP listener for the given address
func createListener(url string, port int) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%d", url, port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener on %s: %w", addr, err)
	}

	return listener, nil
}

func createWorkerPool(maxWorkers, queueSize int) *WorkerPool {
	return NewWorkerPool(maxWorkers, queueSize)
}

func createRateLimiter(rate, tokens int64) ratelimiter.TokenBucket {
	return ratelimiter.RateLimiter(rate, tokens)
}

func handleRequests(s *Server) {
	log.Println("start handling requests")

	var connCount int64

	for {
		client, err := s.Listener.Accept()
		if err != nil {
			log.Fatalf("accept error: %v", err)
		}

		connID := atomic.AddInt64(&connCount, 1)

		// Check rate limiter if configured
		if s.reqLimiter.MaxTokens > 0 && !s.reqLimiter.IsReqAllowed() {
			response := []byte("HTTP/1.1 429 Too Many Requests\r\nConnection: close\r\nContent-Length: 20\r\n\r\nRate limit exceeded")
			client.Write(response)
			client.Close()
			log.Printf("Request %d rate limited", connID)
			continue
		}

		// Submit job to worker pool (non-blocking)
		// Handle panic if channel is closed
		job := Job{Id: int(connID), Conn: client}
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel is closed - server is shutting down
					response := []byte("HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Length: 28\r\n\r\nServer shutting down")
					client.Write(response)
					client.Close()
					log.Printf("Request %d rejected - server shutting down", connID)
				}
			}()

			select {
			case s.JobChan <- job:
				// Job accepted - increment metrics
				s.Metrics.Requests.WithLabelValues("processed").Inc()
			default:
				// Worker pool is full - reject request
				response := []byte("HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Length: 28\r\n\r\nServer busy, try again later")
				client.Write(response)
				client.Close()
				log.Printf("Request %d rejected - server busy (queue full)", connID)
			}
		}()
	}
}

// NewServer creates a new server instance with all components initialized
func NewServer(url string, port int, opts ServerOpts, metrics metrics.ServerMetrics) (*Server, error) {
	// Create listener
	listener, err := createListener(url, port)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Create worker pool
	workerPool := createWorkerPool(opts.MaxThreads, opts.QueueSize)

	// Create rate limiter
	rateLimiter := createRateLimiter(opts.Rate, opts.Tokens)

	return &Server{
		WorkerPool: *workerPool,
		Port:       port,
		URL:        url,
		Opts:       opts,
		Metrics:    metrics,
		Listener:   listener,
		reqLimiter: rateLimiter,
	}, nil
}

// Start starts the server and begins handling requests (blocks)
func (s *Server) Start() {
	log.Printf("Starting server on %s:%d", s.URL, s.Port)
	handleRequests(s)
}

// Close closes the socket listener and worker pool
func (s *Server) Close() {
	s.Listener.Close()
	s.WorkerPool.Close()
}
