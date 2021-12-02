package service

import (
	"context"
	"sync"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/models"
	log "github.com/sirupsen/logrus"
)

// TokenService -
type TokenService struct {
	repo    models.TokenRepository
	handler func(ctx context.Context, contract models.TokenMetadata) error
	workers chan struct{}
	wg      sync.WaitGroup
}

// NewContractService -
func NewTokenService(repo models.TokenRepository, handler func(context.Context, models.TokenMetadata) error) *TokenService {
	return &TokenService{
		repo:    repo,
		handler: handler,
		workers: make(chan struct{}, 10),
	}
}

// Start -
func (s *TokenService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.manager(ctx)
}

// Close -
func (s *TokenService) Close() error {
	s.wg.Wait()

	close(s.workers)
	return nil
}

func (s *TokenService) manager(ctx context.Context) {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warn("processor token metadata service: service without handler")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			unresolved, err := s.repo.GetTokenMetadata(models.StatusNew, 15, 0)
			if err != nil {
				log.Error(err)
				continue
			}

			if len(unresolved) == 0 {
				time.Sleep(time.Second)
				continue
			}

			for i := range unresolved {
				s.workers <- struct{}{}
				s.wg.Add(1)
				go func(contract models.TokenMetadata) {
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
