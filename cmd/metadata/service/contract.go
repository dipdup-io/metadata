package service

import (
	"context"
	"sync"

	"github.com/dipdup-net/metadata/cmd/metadata/models"
	log "github.com/sirupsen/logrus"
)

// ContractService -
type ContractService struct {
	repo    models.ContractRepository
	handler func(ctx context.Context, contract models.ContractMetadata) error
	workers chan struct{}
	wg      sync.WaitGroup
}

// NewContractService -
func NewContractService(repo models.ContractRepository, handler func(context.Context, models.ContractMetadata) error) *ContractService {
	return &ContractService{
		repo:    repo,
		handler: handler,
		workers: make(chan struct{}, 10),
	}
}

// Start -
func (s *ContractService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.manager(ctx)
}

// Close -
func (s *ContractService) Close() error {
	s.wg.Wait()

	close(s.workers)
	return nil
}

func (s *ContractService) manager(ctx context.Context) {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warn("processor contract metadata service: service without handler")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			unresolved, err := s.repo.GetContractMetadata(models.StatusNew, 15, 0)
			if err != nil {
				log.Error(err)
				continue
			}
			for i := range unresolved {
				s.workers <- struct{}{}
				s.wg.Add(1)
				go func(contract models.ContractMetadata) {
					defer func() {
						<-s.workers
						s.wg.Done()
					}()
					if err := s.handler(ctx, contract); err != nil {
						log.Error(err)
					}
				}(unresolved[i])
			}
		}
	}
}
