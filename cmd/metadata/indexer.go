package main

import (
	"context"
	"strings"
	"sync"

	"github.com/go-pg/pg/v10"
	"github.com/pkg/errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	generalConfig "github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/database"
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
	"github.com/dipdup-net/metadata/cmd/metadata/service"
	"github.com/dipdup-net/metadata/cmd/metadata/storage"
	"github.com/dipdup-net/metadata/cmd/metadata/tezoskeys"
	"github.com/dipdup-net/metadata/cmd/metadata/thumbnail"
	"github.com/dipdup-net/metadata/cmd/metadata/tzkt"
	"github.com/dipdup-net/metadata/internal/ipfs"
)

var createIndex sync.Once

// Indexer -
type Indexer struct {
	network   string
	indexName string
	state     *database.State
	resolver  resolver.Receiver
	db        *models.Database
	scanner   *tzkt.Scanner
	prom      *prometheus.Prometheus
	tezosKeys *tezoskeys.TezosKeys
	contracts *service.Service[*models.ContractMetadata]
	tokens    *service.Service[*models.TokenMetadata]
	thumbnail *thumbnail.Service
	settings  config.Settings
	filters   config.Filters

	wg sync.WaitGroup
}

// NewIndexer -
func NewIndexer(ctx context.Context, network string, indexerConfig *config.Indexer, database generalConfig.Database, filters config.Filters, settings config.Settings, prom *prometheus.Prometheus, node *ipfs.Node) (*Indexer, error) {
	db, err := models.NewDatabase(ctx, database)
	if err != nil {
		return nil, err
	}
	keys := tezoskeys.NewTezosKeys(db.TezosKeys)

	metadataResolver, err := resolver.New(ctx, settings, keys, node)
	if err != nil {
		return nil, err
	}
	scanner, err := tzkt.New(indexerConfig.DataSource.Tzkt.Struct(), filters.Addresses()...)
	if err != nil {
		return nil, err
	}

	indexer := &Indexer{
		scanner:   scanner,
		network:   network,
		indexName: models.IndexName(network),
		resolver:  metadataResolver,
		settings:  settings,
		tezosKeys: keys,
		db:        db,
		prom:      prom,
		filters:   filters,
	}

	if aws := storage.NewAWS(settings.AWS); aws != nil {
		indexer.thumbnail = thumbnail.New(
			aws, db.Tokens.(*models.Tokens), network, settings.IPFS.Gateways,
			thumbnail.WithPrometheus(prom),
			thumbnail.WithWorkers(settings.Thumbnail.Workers),
			thumbnail.WithFileSizeLimit(settings.Thumbnail.MaxFileSize),
			thumbnail.WithSize(settings.Thumbnail.Size),
			thumbnail.WithTimeout(settings.Thumbnail.Timeout),
		)
	}
	indexer.contracts = service.NewService(
		db.Contracts, indexer.resolveContractMetadata, network,
		service.WithMaxRetryCount[*models.ContractMetadata](settings.MaxRetryCountOnError),
		service.WithWorkersCount[*models.ContractMetadata](settings.ContractServiceWorkers),
		service.WithPrometheus[*models.ContractMetadata](prom, prometheus.MetadataTypeContract),
		service.WithDelay[*models.ContractMetadata](settings.IPFS.Delay),
	)
	indexer.tokens = service.NewService(
		db.Tokens, indexer.resolveTokenMetadata, network,
		service.WithMaxRetryCount[*models.TokenMetadata](settings.MaxRetryCountOnError),
		service.WithWorkersCount[*models.TokenMetadata](settings.TokenServiceWorkers),
		service.WithPrometheus[*models.TokenMetadata](prom, prometheus.MetadataTypeToken),
		service.WithDelay[*models.TokenMetadata](settings.IPFS.Delay),
	)

	return indexer, nil
}

// Start -
func (indexer *Indexer) Start(ctx context.Context) error {
	createIndex.Do(func() {
		if err := indexer.db.CreateIndices(); err != nil {
			log.Err(err).Msg("create indices")
		}
	})
	if err := indexer.initState(ctx); err != nil {
		return err
	}

	if indexer.filters.LastLevel > 0 && indexer.state.Level > indexer.filters.LastLevel {
		log.Warn().Msgf("You have arrived to a destination. Last level in config is %d. Current state level is %d.", indexer.filters.LastLevel, indexer.state.Level)
		return nil
	}

	if indexer.thumbnail != nil {
		indexer.thumbnail.Start(ctx)
	}

	if indexer.prom != nil {
		newContractCount, err := indexer.db.Contracts.CountByStatus(indexer.network, models.StatusNew)
		if err != nil {
			return err
		}
		indexer.prom.SetMetadataNew(indexer.network, prometheus.MetadataTypeContract, float64(newContractCount))

		newTokenCount, err := indexer.db.Tokens.CountByStatus(indexer.network, models.StatusNew)
		if err != nil {
			return err
		}
		indexer.prom.SetMetadataNew(indexer.network, prometheus.MetadataTypeToken, float64(newTokenCount))
	}

	indexer.contracts.Start(ctx)
	indexer.tokens.Start(ctx)

	indexer.wg.Add(1)
	go indexer.listen(ctx)

	startLevel := indexer.state.Level
	if indexer.filters.FirstLevel > 0 && startLevel < indexer.filters.FirstLevel {
		startLevel = indexer.filters.FirstLevel
	}
	indexer.scanner.Start(ctx, startLevel, indexer.filters.LastLevel)

	return nil
}

// Close -
func (indexer *Indexer) Close() error {
	indexer.wg.Wait()

	if err := indexer.scanner.Close(); err != nil {
		return err
	}

	if err := indexer.tokens.Close(); err != nil {
		return err
	}

	if err := indexer.contracts.Close(); err != nil {
		return err
	}

	if indexer.thumbnail != nil {
		if err := indexer.thumbnail.Close(); err != nil {
			return err
		}
	}

	if err := indexer.db.Close(); err != nil {
		return err
	}

	return nil
}

func (indexer *Indexer) initState(ctx context.Context) error {
	current, err := indexer.db.State(indexer.indexName)
	if err != nil {
		if !errors.Is(err, pg.ErrNoRows) {
			return err
		}
		indexer.state = &database.State{
			IndexType: models.IndexTypeMetadata,
			IndexName: indexer.indexName,
		}

		if err := indexer.db.CreateState(indexer.state); err != nil {
			return err
		}
	} else {
		indexer.state = current

		if err := indexer.initCounters(); err != nil {
			return err
		}
	}
	return indexer.initialTokenMetadata(ctx)
}

func (indexer *Indexer) initCounters() error {
	contractActionsCounter, err := indexer.db.Contracts.LastUpdateID()
	if err != nil {
		return err
	}
	models.ContractUpdateID.Set(contractActionsCounter)

	tokenActionsCounter, err := indexer.db.Tokens.LastUpdateID()
	if err != nil {
		return err
	}
	models.TokenUpdateID.Set(tokenActionsCounter)

	return nil
}

func (indexer *Indexer) log() *zerolog.Event {
	return log.Info().Uint64("state", indexer.state.Level).Str("name", indexer.indexName)
}

func (indexer *Indexer) listen(ctx context.Context) {
	defer indexer.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-indexer.scanner.BigMaps():
			if err := indexer.handlerUpdate(ctx, msg); err != nil {
				log.Err(err).Msg("handlerUpdate")
			} else {
				indexer.log().Msg("New level")
			}
		case block := <-indexer.scanner.Blocks():
			if block.Level-indexer.state.Level > 1 {
				indexer.state.Level = block.Level
				indexer.state.Hash = block.Hash
				indexer.state.Timestamp = block.Timestamp.UTC()
				if err := indexer.db.UpdateState(indexer.state); err != nil {
					log.Err(err).Msg("UpdateState")
				} else {
					indexer.log().Msg("New level")
				}
			}

			if indexer.filters.LastLevel > 0 && block.Level > indexer.filters.LastLevel {
				log.Warn().Msgf("You have arrived to a destination. Last level in config is %d.", indexer.filters.LastLevel)
				return
			}
		}
	}
}

func (indexer *Indexer) handlerUpdate(ctx context.Context, msg tzkt.Message) error {
	tokens := make([]*models.TokenMetadata, 0)
	contracts := make([]*models.ContractMetadata, 0)
	for i := range msg.Body {
		path := strings.Split(msg.Body[i].Path, ".")

		switch path[len(path)-1] {
		case "token_metadata":
			token, err := indexer.processTokenMetadata(msg.Body[i])
			if err != nil {
				return errors.Wrap(err, "token_metadata")
			}
			if token != nil {
				tokens = append(tokens, token)
			}
		case "metadata":
			contract, err := indexer.processContractMetadata(msg.Body[i])
			if err != nil {
				return errors.Wrap(err, "contract_metadata")
			}
			if contract != nil {
				indexer.prom.IncrementMetadataNew(indexer.network, prometheus.MetadataTypeContract)
				contracts = append(contracts, contract)
			}
		}
	}

	if err := indexer.db.Contracts.Save(contracts); err != nil {
		return err
	}

	if err := indexer.db.Tokens.Save(tokens); err != nil {
		return err
	}
	return nil
}
