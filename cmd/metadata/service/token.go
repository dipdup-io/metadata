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
	network       string
	maxRetryCount int64
	repo          models.TokenRepository
	handler       func(ctx context.Context, token *models.TokenMetadata) error
	tasks         chan *models.TokenMetadata
	result        chan *models.TokenMetadata
	workers       chan struct{}
	wg            sync.WaitGroup
}

// NewContractService -
func NewTokenService(repo models.TokenRepository, handler func(context.Context, *models.TokenMetadata) error, network string, maxRetryCount int64) *TokenService {
	return &TokenService{
		maxRetryCount: maxRetryCount,
		network:       network,
		repo:          repo,
		handler:       handler,
		tasks:         make(chan *models.TokenMetadata, 512),
		result:        make(chan *models.TokenMetadata, 16),
		workers:       make(chan struct{}, 5),
	}
}

// Start -
func (s *TokenService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.saver(ctx)

	s.wg.Add(1)
	go s.manager(ctx)
}

// Close -
func (s *TokenService) Close() error {
	s.wg.Wait()

	close(s.workers)
	return nil
}

// ToProcessQueue -
func (s *TokenService) ToProcessQueue(token *models.TokenMetadata) {
	s.tasks <- token
}

func (s *TokenService) manager(ctx context.Context) {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warn().Msg("processor token metadata service: service without handler")
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

			tokens, err := s.repo.GetTokenMetadata(s.network, models.StatusNew, 100, 0, int(s.maxRetryCount))
			if err != nil {
				log.Err(err).Msg("GetTokenMetadata")
				continue
			}
			for i := range tokens {
				s.tasks <- &tokens[i]
			}
		case unresolved := <-s.tasks:
			s.workers <- struct{}{}
			s.wg.Add(1)
			go s.worker(ctx, unresolved)
		}
	}
}

func (s *TokenService) saver(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	tokens := make([]*models.TokenMetadata, 0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.repo.UpdateTokenMetadata(ctx, tokens); err != nil {
				log.Err(err).Msg("UpdateTokenMetadata")
				continue
			}
			tokens = nil

		case token := <-s.result:
			tokens = append(tokens, token)

			if len(tokens) == 8 {
				if err := s.repo.UpdateTokenMetadata(ctx, tokens); err != nil {
					log.Err(err).Msg("UpdateTokenMetadata")
					continue
				}
				tokens = nil
				ticker.Reset(time.Second * 15)
			}
		}
	}

}

func (s *TokenService) worker(ctx context.Context, token *models.TokenMetadata) {
	defer func() {
		s.wg.Done()
		<-s.workers
	}()

	resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.handler(resolveCtx, token); err != nil {
		log.Err(err).Msg("resolve token")
		return
	}

	s.result <- token
}
