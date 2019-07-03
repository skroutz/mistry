package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/skroutz/mistry/pkg/types"
)

// WorkResult contains the result of a build, either a BuildInfo or an error
type WorkResult struct {
	BuildInfo *types.BuildInfo
	Err       error
}

// FutureWorkResult is a WorkResult that may not yet have become available and
// can be Wait()'ed on
type FutureWorkResult struct {
	result <-chan WorkResult
}

// Wait waits for WorkResult to become available and returns it
func (f FutureWorkResult) Wait() WorkResult {
	r, ok := <-f.result
	if !ok {
		// this should never happen, reading from the result channel
		// is exclusive to this future
		panic("Failed to read from result channel")
	}
	return r
}

// workItem contains a job and a channel to place the job result. struct
// used in the internal work queue
type workItem struct {
	job    *Job
	result chan<- WorkResult
}

// WorkerPool implements a fixed-size pool of workers that build jobs
// build jobs and communicate their result
type WorkerPool struct {
	// the fixed amount of goroutines that will be handling running jobs
	concurrency int

	// the maximum backlog of pending requests. if exceeded, sending new work
	// to the pool will return an error
	backlogSize int

	queue chan workItem
	wg    sync.WaitGroup
}

// NewWorkerPool initializes and starts a new worker pool, waiting for incoming
// jobs.
func NewWorkerPool(s *Server, concurrency, backlog int, logger *log.Logger) *WorkerPool {
	p := new(WorkerPool)
	p.concurrency = concurrency
	p.backlogSize = backlog
	p.queue = make(chan workItem, backlog)

	for i := 0; i < concurrency; i++ {
		go work(s, i, p.queue, &p.wg)
		p.wg.Add(1)
	}
	logger.Printf("Set up %d workers", concurrency)
	return p
}

// Stop signals the workers to close and blocks until they are closed.
func (p *WorkerPool) Stop() {
	close(p.queue)
	p.wg.Wait()
}

// SendWork schedules the work j on p and returns a FutureWorkResult.
// The actual result can be obtained by calling FutureWorkResult.Wait().
//
// An error is returned if the work backlog is full.
func (p *WorkerPool) SendWork(j *Job) (FutureWorkResult, error) {
	resultQueue := make(chan WorkResult, 1)
	wi := workItem{j, resultQueue}
	result := FutureWorkResult{resultQueue}

	select {
	case p.queue <- wi:
		return result, nil
	default:
		return result, errors.New("queue is full")
	}
}

// work listens to the workQueue, runs Work() on any incoming work items, and
// sends the result through the result queue
func work(s *Server, id int, queue <-chan workItem, wg *sync.WaitGroup) {
	defer wg.Done()
	logPrefix := fmt.Sprintf("[worker %d]", id)
	for item := range queue {
		buildInfo, err := s.Work(context.Background(), item.job)

		select {
		case item.result <- WorkResult{buildInfo, err}:
		default:
			// this should never happen, the result chan should be unique for this worker
			s.Log.Panicf("%s failed to write result to the result channel", logPrefix)
		}
		close(item.result)
	}
	s.Log.Printf("%s exiting...", logPrefix)
}
