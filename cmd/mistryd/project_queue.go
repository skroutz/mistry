package main

import "sync"

// ProjectQueue provides a goroutine-safe mutex per project
type ProjectQueue struct {
	mu sync.Mutex
	p  map[string]*sync.Mutex
}

// NewProjectQueue creates a new empty struct
func NewProjectQueue() *ProjectQueue {
	return &ProjectQueue{p: make(map[string]*sync.Mutex)}
}

// Lock locks the project's mutex
func (q *ProjectQueue) Lock(project string) {
	q.mu.Lock()
	plock, ok := q.p[project]
	if !ok {
		q.p[project] = new(sync.Mutex)
		plock = q.p[project]
	}
	q.mu.Unlock()
	plock.Lock()
}

// Unlock unlocks the project's mutex
func (q *ProjectQueue) Unlock(project string) {
	q.mu.Lock()
	q.p[project].Unlock()
	q.mu.Unlock()
}
