package main

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Queue struct {
	db           *gorm.DB
	queue        []interface{}
	mux          sync.Mutex
	flushTimeout time.Duration
	onFlush      func(tx *gorm.DB, flushed []interface{}) error
	onTick       func(tx *gorm.DB) error
	full         chan struct{}
	stop         chan struct{}
	wg           sync.WaitGroup
}

// NewQueue -
func NewQueue(db *gorm.DB, capacity, flushTimeout int,
	onFlush func(tx *gorm.DB, flushed []interface{}) error,
	onTick func(tx *gorm.DB) error) *Queue {
	flushTimeoutDuration := time.Second * time.Duration(flushTimeout)
	return &Queue{
		db:           db,
		queue:        make([]interface{}, 0, capacity),
		onFlush:      onFlush,
		onTick:       onTick,
		flushTimeout: flushTimeoutDuration,
		full:         make(chan struct{}, 1),
		stop:         make(chan struct{}, 1),
	}
}

// Add -
func (q *Queue) Add(item interface{}) {
	defer q.mux.Unlock()
	q.mux.Lock()

	q.queue = append(q.queue, item)

	if len(q.queue) == cap(q.queue) {
		q.full <- struct{}{}
	}
}

// Start -
func (q *Queue) Start() {
	q.wg.Add(1)
	go q.listen()
}

// Close -
func (q *Queue) Close() error {
	q.stop <- struct{}{}
	q.wg.Wait()

	close(q.stop)
	close(q.full)
	q.onFlush = nil
	q.onTick = nil
	return nil
}

func (q *Queue) flush() error {
	defer q.mux.Unlock()
	q.mux.Lock()

	if q.onFlush != nil {
		if err := q.onFlush(q.db, q.queue); err != nil {
			return err
		}
	}

	q.queue = make([]interface{}, 0, cap(q.queue))
	return nil
}

func (q *Queue) listen() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.flushTimeout)
	defer ticker.Stop()

	if q.onTick != nil {
		if err := q.onTick(q.db); err != nil {
			log.Error(err)
		}
	}

	for {
		select {
		case <-q.stop:
			return
		case <-ticker.C:
			if err := q.flush(); err != nil {
				log.Error(err)
			}
			if q.onTick != nil && len(q.stop) == 0 {
				if err := q.onTick(q.db); err != nil {
					log.Error(err)
				}
			}
		case <-q.full:
			if err := q.flush(); err != nil {
				log.Error(err)
			}
			ticker.Reset(q.flushTimeout)
			if q.onTick != nil && len(q.stop) == 0 {
				if err := q.onTick(q.db); err != nil {
					log.Error(err)
				}
			}
		}
	}
}
