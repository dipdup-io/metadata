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
	maxRetryCount int64
	repo          models.TokenRepository
	handler       func(ctx context.Context, token *models.TokenMetadata) error
	tasks         chan *models.TokenMetadata
	result        chan *models.TokenMetadata
	workers       chan struct{}
	wg            sync.WaitGroup
}

// NewContractService -
func NewTokenService(repo models.TokenRepository, handler func(context.Context, *models.TokenMetadata) error, maxRetryCount int64) *TokenService {
	return &TokenService{
		maxRetryCount: maxRetryCount,
		repo:          repo,
		handler:       handler,
		tasks:         make(chan *models.TokenMetadata, 1024*128),
		result:        make(chan *models.TokenMetadata, 15),
		workers:       make(chan struct{}, 10),
	}
}

// Start -
func (s *TokenService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.saver(ctx)

	s.wg.Add(1)
	go s.manager(ctx)

	var offset int
	var end bool
	for !end {
		tokens, err := s.repo.GetTokenMetadata(models.StatusNew, 100, offset, int(s.maxRetryCount))
		if err != nil {
			log.Err(err).Msg("GetTokenMetadata")
			continue
		}
		for i := range tokens {
			s.tasks <- &tokens[i]
		}
		offset += len(tokens)
		end = len(tokens) < 100
	}
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

func (s *TokenService) saver(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	tokens := make([]*models.TokenMetadata, 0, 8)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.repo.UpdateTokenMetadata(ctx, tokens); err != nil {
				log.Err(err).Msg("UpdateTokenMetadata")
				return
			}
			tokens = make([]*models.TokenMetadata, 0, 8)

		case token := <-s.result:
			tokens = append(tokens, token)

			if len(tokens) == cap(tokens) {
				if err := s.repo.UpdateTokenMetadata(ctx, tokens); err != nil {
					log.Err(err).Msg("UpdateTokenMetadata")
					return
				}
				tokens = make([]*models.TokenMetadata, 0, 8)
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

	if token.Status != models.StatusApplied && int64(token.RetryCount) < s.maxRetryCount {
		s.tasks <- token
	}

}
