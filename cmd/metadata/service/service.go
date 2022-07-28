package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/internal/ipfs"
	"github.com/go-pg/pg/v10"
	"github.com/rs/zerolog/log"
)

// Service -
type Service[T models.Constraint] struct {
	repo     models.ModelRepository[T]
	ipfsRepo *models.IPFS

	network       string
	maxRetryCount int
	workersCount  int
	delay         int
	handler       func(ctx context.Context, t T) error
	prom          *prometheus.Service
	gaugeType     string
	tasks         chan T
	result        chan T
	queue         *Queue
	wg            sync.WaitGroup
}

// NewService -
func NewService[T models.Constraint](repo models.ModelRepository[T], handler func(context.Context, T) error, network string, opts ...ServiceOption[T]) *Service[T] {
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
				log.Err(err).Msg("GetContractMetadata")
				continue
			}

			if len(data) == 0 {
				time.Sleep(time.Second)
				continue
			}

			resolvedIPFS := make(map[string]models.IPFSLink)

			if s.ipfsRepo != nil {
				links := make([]string, 0)

				for i := range data {
					if s.queue.Contains(data[i].GetID()) || !ipfs.Is(data[i].GetLink()) {
						continue
					}
					links = append(links, data[i].GetLink())
				}
				if len(links) > 0 {
					resolved, err := s.ipfsRepo.GetByURLs(links...)
					if err != nil && !errors.Is(err, pg.ErrNoRows) {
						log.Err(err).Msg("contract IPFSLinkByURLs")
					}
					for i := range resolved {
						resolvedIPFS[resolved[i].Link] = resolved[i]
					}
				}
			}

			for i := range data {
				if s.queue.Contains(data[i].GetID()) {
					continue
				}
				s.queue.Add(data[i].GetID())

				if s.ipfsRepo != nil {
					if ipfsData, ok := resolvedIPFS[data[i].GetLink()]; ok {
						data[i].SetStatus(models.StatusApplied)
						data[i].SetMetadata(helpers.Escape(ipfsData.Data))
						data[i].IncrementRetryCount()
						s.result <- data[i]
						continue
					}
				}
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

			if err := s.bulkSave(ctx, data); err != nil {
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
			if err := s.bulkSave(ctx, data); err != nil {
				log.Err(err).Msg("bulkSave")
				data = nil
				continue
			}

			data = nil
		}
	}
}

func (s *Service[T]) bulkSave(ctx context.Context, data []T) error {
	if err := s.repo.Update(data); err != nil {
		return err
	}

	for i := range data {
		s.queue.Delete(data[i].GetID())

		if s.prom != nil {
			switch data[i].GetStatus() {
			case models.StatusApplied, models.StatusFailed:
				s.prom.DecGaugeValue("metadata_new", map[string]string{
					"network": s.network,
					"type":    s.gaugeType,
				})
				s.prom.IncrementCounter("metadata_counter", map[string]string{
					"network": s.network,
					"type":    s.gaugeType,
					"status":  data[i].GetStatus().String(),
				})
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
				log.Err(err).Msg("resolve contract")
			}

			s.result <- unresolved
		}
	}

}
