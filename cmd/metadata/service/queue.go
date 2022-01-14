package service

import "sync"

// Queue -
type Queue struct {
	q  map[uint64]struct{}
	mx sync.RWMutex
}

// NewQueue -
func NewQueue() *Queue {
	return &Queue{
		q: make(map[uint64]struct{}),
	}
}

// Add -
func (q *Queue) Add(id uint64) {
	q.mx.Lock()
	q.q[id] = struct{}{}
	q.mx.Unlock()
}

// Contains -
func (q *Queue) Contains(id uint64) bool {
	q.mx.RLock()
	_, ok := q.q[id]
	q.mx.RUnlock()
	return ok
}

// Delete -
func (q *Queue) Delete(id uint64) {
	q.mx.Lock()
	delete(q.q, id)
	q.mx.Unlock()
}
