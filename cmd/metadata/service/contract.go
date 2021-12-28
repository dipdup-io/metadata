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
	maxRetryCount int64
	db            models.Database
	handler       func(ctx context.Context, contract *models.ContractMetadata) error
	tasks         chan *models.ContractMetadata
	result        chan *models.ContractMetadata
	workers       chan struct{}
	wg            sync.WaitGroup
}

// NewContractService -
func NewContractService(db models.Database, handler func(context.Context, *models.ContractMetadata) error, maxRetryCount int64) *ContractService {
	return &ContractService{
		maxRetryCount: maxRetryCount,
		db:            db,
		handler:       handler,
		tasks:         make(chan *models.ContractMetadata, 1024*128),
		result:        make(chan *models.ContractMetadata, 15),
		workers:       make(chan struct{}, 10),
	}
}

// Start -
func (s *ContractService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.manager(ctx)

	s.wg.Add(1)
	go s.saver(ctx)

	var offset int
	var end bool
	for !end {
		contracts, err := s.db.GetContractMetadata(models.StatusNew, 100, offset, int(s.maxRetryCount))
		if err != nil {
			log.Err(err).Msg("GetContractMetadata")
			continue
		}
		for i := range contracts {
			s.tasks <- &contracts[i]
		}
		offset += len(contracts)
		end = len(contracts) < 100
	}
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

	for {
		select {
		case <-ctx.Done():
			return
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

	contracts := make([]*models.ContractMetadata, 0, 8)
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if err := s.db.UpdateContractMetadata(ctx, contracts); err != nil {
				log.Err(err).Msg("UpdateContractMetadata")
				return
			}
			contracts = make([]*models.ContractMetadata, 0, 8)

		case contract := <-s.result:
			contracts = append(contracts, contract)

			if len(contracts) == cap(contracts) {
				if err := s.db.UpdateContractMetadata(ctx, contracts); err != nil {
					log.Err(err).Msg("UpdateContractMetadata")
					return
				}
				contracts = make([]*models.ContractMetadata, 0, 8)
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

	if contract.Status != models.StatusApplied && int64(contract.RetryCount) < s.maxRetryCount {
		s.tasks <- contract
	}
}
