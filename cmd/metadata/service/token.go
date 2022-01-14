package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/internal/ipfs"
	"github.com/go-pg/pg/v10"
	"github.com/rs/zerolog/log"
)

// TokenService -
type TokenService struct {
	network       string
	maxRetryCount int
	workersCount  int
	repo          models.Database
	handler       func(ctx context.Context, token *models.TokenMetadata) error
	prom          *prometheus.Service
	tasks         chan *models.TokenMetadata
	result        chan *models.TokenMetadata
	wg            sync.WaitGroup
}

// NewContractService -
func NewTokenService(repo models.Database, handler func(context.Context, *models.TokenMetadata) error, network string, opts ...TokenServiceOption) *TokenService {
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

	close(s.tasks)
	close(s.result)
	return nil
}

func (s *TokenService) manager(ctx context.Context) {
	defer s.wg.Done()

	if s.handler == nil {
		log.Warn().Msg("processor token metadata service: service without handler")
		return
	}

	ticker := time.NewTicker(time.Millisecond * 10)
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
				if ipfs.Is(tokens[i].Link) {
					link, err := s.repo.IPFSLinkByURL(tokens[i].Link)
					if err == nil {
						tokens[i].Status = models.StatusApplied
						tokens[i].Metadata = link.Data
						tokens[i].RetryCount += 1
						s.result <- &tokens[i]
						continue
					}

					if !errors.Is(err, pg.ErrNoRows) {
						log.Err(err).Msg("token IPFSLinkByURL")
					}
				}

				s.tasks <- &tokens[i]
			}
		}
	}
}

func (s *TokenService) saver(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	tokens := make([]*models.TokenMetadata, 0)
	for {
		select {
		case <-ctx.Done():
			return

		case token := <-s.result:
			tokens = append(tokens, token)

			if s.prom != nil {
				switch token.Status {
				case models.StatusApplied, models.StatusFailed:
					s.prom.DecGaugeValue("metadata_new", map[string]string{
						"network": s.network,
						"type":    "token",
					})
					s.prom.IncrementCounter("metadata_counter", map[string]string{
						"network": s.network,
						"type":    "token",
						"status":  token.Status.String(),
					})
				}
			}

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
			}

			s.result <- unresolved
		}
	}
}
