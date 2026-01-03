package server

import (
	"log"
	"net"
	"sync"
	"time"
)

// Job is a task submitted by server to the worker pool
type Job struct {
	Id   int
	Conn net.Conn
}

type WorkerPool struct {
	MaxWorkers int      //max no of workers worker pool can handle concurrently
	QueueSize  int      //number of task that will kept in queue if all the workers are busy
	JobChan    chan Job //buffered channel used to put job in worker pool
	wg         *sync.WaitGroup
}

func NewWorkerPool(maxWorkers, queueSize int) *WorkerPool {
	w := &WorkerPool{
		MaxWorkers: maxWorkers,
		QueueSize:  queueSize,
		JobChan:    make(chan Job, maxWorkers+queueSize), // Channel size = MaxWorkers + QueueSize
		wg:         new(sync.WaitGroup),
	}
	for i := 0; i < w.MaxWorkers; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	return w
}

// worker is a thread which processes the requests, ye jab tak maxworkers hai tab tak
// usko wo job execute krne dete hai
func (w *WorkerPool) worker(workerId int) {
	processRequests := func(j Job) {
		// Set read deadline to prevent hanging (3 seconds)
		j.Conn.SetReadDeadline(time.Now().Add(3 * time.Second))

		request := make([]byte, 4096)
		_, err := j.Conn.Read(request)
		if err != nil {
			// Timeout or read error - send error response before closing
			j.Conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			errorResponse := []byte("HTTP/1.1 408 Request Timeout\r\nConnection: close\r\nContent-Length: 0\r\n\r\n")
			j.Conn.Write(errorResponse)
			j.Conn.Close()
			return
		}

		// Set write deadline before sending response
		j.Conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

		// Send proper HTTP response with Connection: close header
		// Content-Length must match actual body length (14 bytes: "Hello world !\n")
		response := []byte("HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Length: 14\r\n\r\nHello world !\n")
		bytesWritten, writeErr := j.Conn.Write(response)
		if writeErr != nil || bytesWritten != len(response) {
			// Write failed or incomplete, close and return
			j.Conn.Close()
			return
		}

		// Close connection - TCP default behavior will send all pending data
		// before closing, ensuring curl receives the complete response
		j.Conn.Close()
	}

	for job := range w.JobChan {
		log.Printf("Worker %d, processing request %d", workerId, job.Id)
		processRequests(job)
	}

	w.wg.Done()
}

// SubmitJob puts the job into the channel and idle worker picks up
func (w *WorkerPool) SubmitJob(j Job) {
	w.JobChan <- j
}

// Close closes the channel and wait for all the workers to finish
func (w *WorkerPool) Close() {
	close(w.JobChan)
	w.wg.Wait()
}
