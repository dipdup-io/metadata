package service

import (
	"context"
	"sync"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/prometheus"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Service -
type Service[T models.Model] struct {
	repo models.ModelRepository[T]

	network       string
	maxRetryCount int
	workersCount  int
	delay         int
	handler       func(ctx context.Context, t T) error
	prom          *prometheus.Prometheus
	gaugeType     string
	tasks         chan T
	result        chan T
	queue         *Queue
	wg            *sync.WaitGroup
}

// NewService -
func NewService[T models.Model](repo models.ModelRepository[T], handler func(context.Context, T) error, network string, opts ...ServiceOption[T]) *Service[T] {
	cs := &Service[T]{
		maxRetryCount: 3,
		workersCount:  5,
		repo:          repo,
		handler:       handler,
		tasks:         make(chan T, 512),
		result:        make(chan T, 16),
		network:       network,
		queue:         NewQueue(),
		delay:         10,
		wg:            new(sync.WaitGroup),
	}

	for i := range opts {
		opts[i](cs)
	}

	return cs
}

// Start -
func (s *Service[T]) Start(ctx context.Context) {
	for i := 0; i < s.workersCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}

	s.wg.Add(1)
	go s.manager(ctx)

	s.wg.Add(1)
	go s.saver(ctx)

	s.wg.Add(1)
	go s.lastHope(ctx)
}

// Close -
func (s *Service[T]) Close() error {
	s.wg.Wait()

	close(s.tasks)
	close(s.result)

	return nil
}

func (s *Service[T]) manager(ctx context.Context) {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warn().Msg("processor contract metadata service: service without handler")
		return
	}

	ticker := time.NewTicker(time.Millisecond * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if len(s.tasks) > s.workersCount {
				continue
			}
			data, err := s.repo.Get(s.network, models.StatusNew, 200, 0, s.maxRetryCount, s.delay)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Err(err).Msg("repo.Get")
				}
				continue
			}

			if len(data) == 0 {
				time.Sleep(time.Second)
				continue
			}

			for i := range data {
				if s.queue.Contains(data[i].GetID()) {
					continue
				}
				s.queue.Add(data[i].GetID())

				s.tasks <- data[i]
			}
		}
	}
}

func (s *Service[T]) saver(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	data := make([]T, 0)
	for {
		select {
		case <-ctx.Done():
			return

		case result := <-s.result:
			data = append(data, result)

			if len(data) < 10 {
				continue
			}

			if err := s.bulkSave(data); err != nil {
				log.Err(err).Msg("bulkSave")
				data = nil
				continue
			}
			data = nil
			ticker.Reset(time.Second * 15)

		case <-ticker.C:
			if len(data) == 0 {
				continue
			}
			if err := s.bulkSave(data); err != nil {
				log.Err(err).Msg("bulkSave")
				data = nil
				continue
			}

			data = nil
		}
	}
}

func (s *Service[T]) bulkSave(data []T) error {
	if err := s.repo.Update(data); err != nil {
		return err
	}

	for i := range data {
		s.queue.Delete(data[i].GetID())

		if s.prom != nil {
			status := data[i].GetStatus()
			switch status {
			case models.StatusApplied, models.StatusFailed:
				s.prom.DecrementMetadataNew(s.network, s.gaugeType)
				s.prom.IncrementMetadataCounter(s.network, s.gaugeType, status.String())
			}
		}
	}
	return nil
}

func (s *Service[T]) worker(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case unresolved := <-s.tasks:
			resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			if err := s.handler(resolveCtx, unresolved); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Err(err).Msg("resolve")
			}

			s.result <- unresolved
		}
	}

}

func (s *Service[T]) lastHope(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.repo.Retry(s.network, s.maxRetryCount, 3*time.Hour/time.Second); err != nil {
				log.Err(err).Msg("repo.Retry")
				continue
			}
		}
	}

}
