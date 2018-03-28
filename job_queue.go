package main

import (
	"sync"
)

type JobQueue struct {
	j map[string]bool
	sync.Mutex
}

func NewJobQueue() *JobQueue {
	return &JobQueue{j: make(map[string]bool)}
}

// Add registers j to the list of pending jobs currently in the queue.
// It returns false an identical job is already enqueued.
func (q *JobQueue) Add(j *Job) bool {
	q.Lock()
	defer q.Unlock()

	if q.j[j.ID] {
		return false
	}

	q.j[j.ID] = true
	return true
}

func (q *JobQueue) Delete(j *Job) {
	q.Lock()
	defer q.Unlock()
	q.j[j.ID] = false
}
