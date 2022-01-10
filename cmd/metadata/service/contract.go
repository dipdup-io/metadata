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
	maxRetryCount int64
	db            models.Database
	handler       func(ctx context.Context, contract *models.ContractMetadata) error
	tasks         chan *models.ContractMetadata
	result        chan *models.ContractMetadata
	workers       chan struct{}
	wg            sync.WaitGroup
}

// NewContractService -
func NewContractService(db models.Database, handler func(context.Context, *models.ContractMetadata) error, network string, maxRetryCount int64) *ContractService {
	return &ContractService{
		maxRetryCount: maxRetryCount,
		db:            db,
		handler:       handler,
		tasks:         make(chan *models.ContractMetadata, 512),
		result:        make(chan *models.ContractMetadata, 16),
		workers:       make(chan struct{}, 5),
		network:       network,
	}
}

// Start -
func (s *ContractService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.manager(ctx)

	s.wg.Add(1)
	go s.saver(ctx)
}

// Close -
func (s *ContractService) Close() error {
	s.wg.Wait()

	close(s.workers)
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
			contracts, err := s.db.GetContractMetadata(s.network, models.StatusNew, 100, 0, int(s.maxRetryCount))
			if err != nil {
				log.Err(err).Msg("GetContractMetadata")
				continue
			}
			for i := range contracts {
				s.tasks <- &contracts[i]
			}

		case unresolved := <-s.tasks:
			s.workers <- struct{}{}
			s.wg.Add(1)
			go s.worker(ctx, unresolved)
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

func (s *ContractService) worker(ctx context.Context, contract *models.ContractMetadata) {
	defer func() {
		s.wg.Done()
		<-s.workers
	}()

	resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.handler(resolveCtx, contract); err != nil {
		log.Err(err).Msg("resolve contract")
		return
	}

	s.result <- contract
}
