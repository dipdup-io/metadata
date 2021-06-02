package adapter

import (
	"io"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Adapter -
type Adapter interface {
	Do() error
	io.Closer
}

// Runner -
type Runner struct {
	adapter Adapter
	wg      sync.WaitGroup
	stop    chan struct{}
}

// NewRunner -
func NewRunner(adapter Adapter) *Runner {
	return &Runner{
		adapter: adapter,
		stop:    make(chan struct{}, 1),
	}
}

// Start -
func (r *Runner) Start() {
	r.wg.Add(1)
	go r.do()
}

func (r *Runner) do() {
	defer r.wg.Done()

	if r.adapter == nil {
		return
	}

	if err := r.adapter.Do(); err != nil {
		log.Error(err)
	}

	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			if err := r.adapter.Do(); err != nil {
				log.Error(err)
			}
		}
	}
}

// Close -
func (r *Runner) Close() error {
	r.stop <- struct{}{}
	r.wg.Wait()

	if r.adapter != nil {
		if err := r.adapter.Close(); err != nil {
			return err
		}
	}

	close(r.stop)
	return nil
}
