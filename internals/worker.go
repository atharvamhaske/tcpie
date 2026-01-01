package server

import (
	"log"
	"net"
	"sync"
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

func (w *WorkerPool) NewWorkerPool() {
	w.wg = new(sync.WaitGroup)
	w.JobChan = make(chan Job, w.MaxWorkers+w.QueueSize) //create new buffer channel of type Job

	for i := 0; i < w.MaxWorkers; i++ {
		w.wg.Add(1)
		log.Printf("Starting worker %d", i)
		go w.worker(i)
	}
}

// worker is a thread which processes the requests
func (w *WorkerPool) worker(workerId int) {
	processRequests := func(j Job) {
		request := make([]byte, 1024)
		j.Conn.Read(request)
		response := []byte("HTTP/1.1 200 OK\r\n\r\n Hello world ! \r\n")
		j.Conn.Write(response)
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
