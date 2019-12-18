package main

import (
	"sync"
)

// JobQueue holds the jobs that are enqueued currently in the server. It allows
// used as a means to do build coalescing.
type JobQueue struct {
	sync.Mutex
	jobs map[string]bool
}

// NewJobQueue returns a new JobQueue ready for use.
func NewJobQueue() *JobQueue {
	return &JobQueue{jobs: make(map[string]bool)}
}

// Add registers j to the list of pending jobs currently in the queue.
// It returns false if an identical job is already enqueued.
func (q *JobQueue) Add(j *Job) bool {
	q.Lock()
	defer q.Unlock()

	if q.jobs[j.ID] {
		return false
	}

	q.jobs[j.ID] = true
	return true
}

// Delete removes j from q.
func (q *JobQueue) Delete(j *Job) {
	q.Lock()
	defer q.Unlock()

	delete(q.jobs, j.ID)
}
