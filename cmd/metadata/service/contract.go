package service

import (
	"context"
	"sync"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/rs/zerolog/log"
)

// ContractService -
type ContractService struct {
	network       string
	maxRetryCount int
	workersCount  int
	db            models.Database
	handler       func(ctx context.Context, contract *models.ContractMetadata) error
	tasks         chan *models.ContractMetadata
	result        chan *models.ContractMetadata
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

	return nil
}

// ToProcessQueue -
func (s *ContractService) ToProcessQueue(contract *models.ContractMetadata) {
	s.tasks <- contract
}

func (s *ContractService) manager(ctx context.Context) {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warn().Msg("processor contract metadata service: service without handler")
		return
	}

	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if len(s.tasks) > 100 {
				continue
			}
			contracts, err := s.db.GetContractMetadata(s.network, models.StatusNew, 100, 0, s.maxRetryCount)
			if err != nil {
				log.Err(err).Msg("GetContractMetadata")
				continue
			}
			for i := range contracts {
				s.tasks <- &contracts[i]
			}
		}
	}
}

func (s *ContractService) saver(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second * 15)
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
			contracts = nil

		case contract := <-s.result:
			contracts = append(contracts, contract)

			if len(contracts) == 8 {
				if err := s.db.UpdateContractMetadata(ctx, contracts); err != nil {
					log.Err(err).Msg("UpdateContractMetadata")
					continue
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
				continue
			}

			s.result <- unresolved
		}
	}

}
