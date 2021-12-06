package service

import (
	"context"
	"sync"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/rs/zerolog/log"
)

// TokenService -
type TokenService struct {
	repo    models.TokenRepository
	handler func(ctx context.Context, token *models.TokenMetadata) error
	workers chan struct{}
	wg      sync.WaitGroup
}

// NewContractService -
func NewTokenService(repo models.TokenRepository, handler func(context.Context, *models.TokenMetadata) error) *TokenService {
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
		log.Warn().Msg("processor token metadata service: service without handler")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			unresolved, err := s.repo.GetTokenMetadata(models.StatusNew, 15, 0)
			if err != nil {
				log.Err(err).Msg("")
				continue
			}

			if len(unresolved) == 0 {
				time.Sleep(time.Second)
				continue
			}

			result := make([]*models.TokenMetadata, 0)
			for i := range unresolved {
				s.workers <- struct{}{}
				s.wg.Add(1)
				go func(token *models.TokenMetadata) {
					defer func() {
						<-s.workers
						s.wg.Done()
					}()

					resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()

					if err := s.handler(resolveCtx, token); err != nil {
						log.Err(err).Msg("")
						return
					}
					result = append(result, token)
				}(&unresolved[i])
			}

			if err := s.repo.UpdateTokenMetadata(ctx, result); err != nil {
				log.Err(err).Msg("")
				return
			}
		}
	}
}
