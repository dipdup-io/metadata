package service

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Service -
type Service struct {
	name           string
	tickerDuration time.Duration
	handler        func(ctx context.Context) error
	wg             sync.WaitGroup
}

// New -
func New(handler func(context.Context) error, opts ...ServiceOption) *Service {
	s := Service{
		tickerDuration: time.Second * 5,
		handler:        handler,
	}

	for i := range opts {
		opts[i](&s)
	}

	return &s
}

// Start -
func (s *Service) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.worker(ctx)
}

// Close -
func (s *Service) Close() error {
	s.wg.Wait()
	return nil
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warnf("processor service '%s': service without handler", s.name)
		return
	}

	ticker := time.NewTicker(s.tickerDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.handler(ctx); err != nil {
				log.Errorf("processor service '%s': %s", s.name, err.Error())
			}
		}
	}
}
