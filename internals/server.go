package server

import (
	"fmt"
	"log"
	"net"

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

func (s *Server) createListener() {
	log.Println("creating a listener")
	addr := s.URL + ":" + fmt.Sprintf("%d", s.Port)
	socket, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("server connection failed with %v", err)
	}
	s.Listener = socket
}

func (s *Server) createThreadPool() {
	log.Println("creating thread pool")
	s.WorkerPool = WorkerPool{MaxWorkers: s.Opts.MaxThreads, QueueSize: s.Opts.QueueSize}
	s.NewWorkerPool()
}

func (s *Server) createRateLimiter(rate, token int64) {
	log.Println("creating a rate limiter")
	s.reqLimiter = ratelimiter.RateLimiter(rate, token)
}

func (s *Server) handleRequest() {
	log.Println("start handling requests")

	conn := 0

	for {
		client, err := s.Listener.Accept() //accept clients
		conn++
		if err != nil {
			log.Fatalf("%v", err)
		}

		// Check rate limiter if it's configured (MaxTokens > 0 means it's initialized)
		if s.reqLimiter.MaxTokens > 0 && !s.reqLimiter.IsReqAllowed() {
			response := []byte("HTTP/1.1 429 Too Many Requests\r\nConnection: close\r\nContent-Length: 20\r\n\r\nRate limit exceeded")
			client.Write(response)
			client.Close()
			log.Printf("Request %d rate limited", conn)
			continue
		}

		//submit job non-blocking - if channel is full, reject immediately
		select {
		case s.JobChan <- Job{Id: conn, Conn: client}:
			// Job accepted - increment metrics
			s.Metrics.Requests.WithLabelValues("processed").Inc()
		default:
			// Channel full - all workers busy and queue full, reject request
			response := []byte("HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Length: 28\r\n\r\nServer busy, try again later")
			client.Write(response)
			client.Close()
			log.Printf("Request %d rejected - server busy (queue full)", conn)
		}
	}
}

// Close closes the socket listener and worker pool
func (s *Server) Close() {
	s.Listener.Close()
	s.WorkerPool.Close()
}

func (s *Server) FireUpTheServer() {
	s.createListener()
	s.createThreadPool()
	s.createRateLimiter(s.Opts.Rate, s.Opts.Tokens)
	s.handleRequest()
}
