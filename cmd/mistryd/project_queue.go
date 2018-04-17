package main

import "sync"

type ProjectQueue struct {
	mu sync.Mutex
	p  map[string]*sync.Mutex
}

func NewProjectQueue() *ProjectQueue {
	return &ProjectQueue{p: make(map[string]*sync.Mutex)}
}

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

func (q *ProjectQueue) Unlock(project string) {
	q.mu.Lock()
	q.p[project].Unlock()
	q.mu.Unlock()
}
