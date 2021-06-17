package service

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Service -
type Service struct {
	name           string
	tickerDuration time.Duration
	handler        func() error
	stop           chan struct{}
	wg             sync.WaitGroup
}

// New -
func New(handler func() error, opts ...ServiceOption) *Service {
	s := Service{
		stop:           make(chan struct{}, 1),
		tickerDuration: time.Second * 5,
		handler:        handler,
	}

	for i := range opts {
		opts[i](&s)
	}

	return &s
}

// Start -
func (s *Service) Start() {
	s.wg.Add(1)
	go s.worker()
}

// Close -
func (s *Service) Close() error {
	s.stop <- struct{}{}
	s.wg.Wait()
	return nil
}

func (s *Service) worker() {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warnf("processor service '%s': service without handler", s.name)
		return
	}

	ticker := time.NewTicker(s.tickerDuration)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if err := s.handler(); err != nil {
				log.Errorf("processor service '%s': %s", s.name, err.Error())
			}
		}
	}
}
