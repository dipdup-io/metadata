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
	db      models.Database
	handler func(ctx context.Context, contract *models.ContractMetadata) error
	workers chan struct{}
	wg      sync.WaitGroup
}

// NewContractService -
func NewContractService(db models.Database, handler func(context.Context, *models.ContractMetadata) error) *ContractService {
	return &ContractService{
		db:      db,
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
		log.Warn().Msg("processor contract metadata service: service without handler")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			unresolved, err := s.db.GetContractMetadata(models.StatusNew, 15, 0)
			if err != nil {
				log.Err(err).Msg("")
				continue
			}

			if len(unresolved) == 0 {
				time.Sleep(time.Second)
				continue
			}

			result := make([]*models.ContractMetadata, 0)
			for i := range unresolved {
				s.workers <- struct{}{}
				s.wg.Add(1)
				go func(contract *models.ContractMetadata) {
					defer func() {
						<-s.workers
						s.wg.Done()
					}()

					handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()

					if err := s.handler(handlerCtx, contract); err != nil {
						log.Err(err).Msg("")
						return
					}

					result = append(result, contract)
				}(&unresolved[i])
			}

			if err := s.db.UpdateContractMetadata(ctx, result); err != nil {
				log.Err(err).Msg("")
				continue
			}
		}
	}
}
