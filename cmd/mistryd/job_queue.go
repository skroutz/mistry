package main

import (
	"sync"
)

// JobQueue holds the jobs that are enqueued currently in the server.
type JobQueue struct {
	sync.Mutex
	j map[string]bool
}

// NewJobQueue returns a new JobQueue ready for use.
func NewJobQueue() *JobQueue {
	return &JobQueue{j: make(map[string]bool)}
}

// Add registers j to the list of pending jobs currently in the queue.
// It returns false if an identical job is already enqueued.
func (q *JobQueue) Add(j *Job) bool {
	q.Lock()
	defer q.Unlock()

	if q.j[j.ID] {
		return false
	}

	q.j[j.ID] = true
	return true
}

// Delete removes j from q.
func (q *JobQueue) Delete(j *Job) {
	q.Lock()
	defer q.Unlock()
	q.j[j.ID] = false
}
