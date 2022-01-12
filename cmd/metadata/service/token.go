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
	maxRetryCount int
	workersCount  int
	repo          models.TokenRepository
	handler       func(ctx context.Context, token *models.TokenMetadata) error
	tasks         chan *models.TokenMetadata
	result        chan *models.TokenMetadata
	wg            sync.WaitGroup
}

// NewContractService -
func NewTokenService(repo models.TokenRepository, handler func(context.Context, *models.TokenMetadata) error, network string, opts ...TokenServiceOption) *TokenService {
	ts := &TokenService{
		maxRetryCount: 3,
		workersCount:  5,
		network:       network,
		repo:          repo,
		handler:       handler,
		tasks:         make(chan *models.TokenMetadata, 512),
		result:        make(chan *models.TokenMetadata, 16),
	}
	for i := range opts {
		opts[i](ts)
	}
	return ts
}

// Start -
func (s *TokenService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.manager(ctx)

	for i := 0; i < s.workersCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}

	s.wg.Add(1)
	go s.saver(ctx)

}

// Close -
func (s *TokenService) Close() error {
	s.wg.Wait()

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

			tokens, err := s.repo.GetTokenMetadata(s.network, models.StatusNew, 100, 0, s.maxRetryCount)
			if err != nil {
				log.Err(err).Msg("GetTokenMetadata")
				continue
			}
			for i := range tokens {
				s.tasks <- &tokens[i]
			}
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

		case <-ticker.C:
			if len(tokens) == 0 {
				continue
			}
			if err := s.repo.UpdateTokenMetadata(ctx, tokens); err != nil {
				log.Err(err).Msg("UpdateTokenMetadata")
				continue
			}
			tokens = nil

		}
	}

}

func (s *TokenService) worker(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case unresolved := <-s.tasks:
			resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			if err := s.handler(resolveCtx, unresolved); err != nil {
				log.Err(err).Msg("resolve token")
				return
			}

			s.result <- unresolved
		}
	}
}
