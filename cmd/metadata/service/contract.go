package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/internal/ipfs"
	"github.com/go-pg/pg/v10"
	"github.com/rs/zerolog/log"
)

// ContractService -
type ContractService struct {
	network       string
	maxRetryCount int
	workersCount  int
	db            models.Database
	handler       func(ctx context.Context, contract *models.ContractMetadata) error
	prom          *prometheus.Service
	tasks         chan *models.ContractMetadata
	result        chan *models.ContractMetadata
	queue         *Queue
	wg            sync.WaitGroup
}

// NewContractService -
func NewContractService(db models.Database, handler func(context.Context, *models.ContractMetadata) error, network string, opts ...ContractServiceOption) *ContractService {
	cs := &ContractService{
		maxRetryCount: 3,
		workersCount:  5,
		db:            db,
		handler:       handler,
		tasks:         make(chan *models.ContractMetadata, 512),
		result:        make(chan *models.ContractMetadata, 16),
		network:       network,
		queue:         NewQueue(),
	}

	for i := range opts {
		opts[i](cs)
	}

	return cs
}

// Start -
func (s *ContractService) Start(ctx context.Context) {
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
func (s *ContractService) Close() error {
	s.wg.Wait()

	close(s.tasks)
	close(s.result)

	return nil
}

func (s *ContractService) manager(ctx context.Context) {
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
			contracts, err := s.db.GetContractMetadata(s.network, models.StatusNew, 200, 0, s.maxRetryCount)
			if err != nil {
				log.Err(err).Msg("GetContractMetadata")
				continue
			}

			links := make([]string, 0)
			resolvedIPFS := make(map[string]models.IPFSLink)

			for i := range contracts {
				if s.queue.Contains(contracts[i].ID) || !ipfs.Is(contracts[i].Link) {
					continue
				}
				links = append(links, contracts[i].Link)
			}
			if len(links) > 0 {
				resolved, err := s.db.IPFSLinkByURLs(links...)
				if err != nil && !errors.Is(err, pg.ErrNoRows) {
					log.Err(err).Msg("contract IPFSLinkByURLs")
					continue
				}
				for i := range resolved {
					resolvedIPFS[resolved[i].Link] = resolved[i]
				}
			}

			for i := range contracts {
				if s.queue.Contains(contracts[i].ID) {
					continue
				}

				s.queue.Add(contracts[i].ID)

				if len(resolvedIPFS) > 0 && ipfs.Is(contracts[i].Link) {
					data, ok := resolvedIPFS[contracts[i].Link]
					if ok {
						contracts[i].Status = models.StatusApplied
						contracts[i].Metadata = data.Data
						contracts[i].RetryCount += 1
						s.result <- &contracts[i]
						continue
					}
				}
				s.tasks <- &contracts[i]
			}
			if len(contracts) == 0 {
				time.Sleep(time.Second)
			}
		}
	}
}

func (s *ContractService) saver(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	contracts := make([]*models.ContractMetadata, 0)
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if len(contracts) == 0 {
				continue
			}
			if err := s.db.UpdateContractMetadata(ctx, contracts); err != nil {
				log.Err(err).Msg("UpdateContractMetadata")
				continue
			}
			for i := range contracts {
				s.queue.Delete(contracts[i].ID)
			}
			contracts = nil

		case contract := <-s.result:
			contracts = append(contracts, contract)

			if s.prom != nil {
				switch contract.Status {
				case models.StatusApplied, models.StatusFailed:
					s.prom.DecGaugeValue("metadata_new", map[string]string{
						"network": s.network,
						"type":    "contract",
					})
					s.prom.IncrementCounter("metadata_counter", map[string]string{
						"network": s.network,
						"type":    "contract",
						"status":  contract.Status.String(),
					})
				}
			}

			if len(contracts) == 10 {
				if err := s.db.UpdateContractMetadata(ctx, contracts); err != nil {
					log.Err(err).Msg("UpdateContractMetadata")
					continue
				}
				for i := range contracts {
					s.queue.Delete(contracts[i].ID)
				}
				contracts = nil
				ticker.Reset(time.Second * 15)
			}
		}
	}

}

func (s *ContractService) worker(ctx context.Context) {
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
