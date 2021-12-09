package main

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/go-pg/pg/v10"
	"github.com/pkg/errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	generalConfig "github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/database"
	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	internalContext "github.com/dipdup-net/metadata/cmd/metadata/context"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
	"github.com/dipdup-net/metadata/cmd/metadata/service"
	"github.com/dipdup-net/metadata/cmd/metadata/storage"
	"github.com/dipdup-net/metadata/cmd/metadata/thumbnail"
	"github.com/dipdup-net/metadata/cmd/metadata/tzkt"
)

// Indexer -
type Indexer struct {
	network   string
	indexName string
	state     database.State
	resolver  resolver.Receiver
	db        models.Database
	scanner   *tzkt.Scanner
	prom      *prometheus.Service
	ctx       *internalContext.Context
	contracts *service.ContractService
	tokens    *service.TokenService
	thumbnail *thumbnail.Service
	settings  config.Settings

	contractActionsCounter *helpers.Counter
	tokenActionsCounter    *helpers.Counter

	wg sync.WaitGroup
}

// NewIndexer -
func NewIndexer(ctx context.Context, network string, indexerConfig *config.Indexer, database generalConfig.Database, filters config.Filters, settings config.Settings, prom *prometheus.Service) (*Indexer, error) {
	db, err := models.NewDatabase(ctx, database)
	if err != nil {
		return nil, err
	}
	cont := internalContext.NewContext()

	indexer := &Indexer{
		scanner:                tzkt.New(indexerConfig.DataSource.Tzkt, filters.Accounts...),
		network:                network,
		indexName:              models.IndexName(network),
		resolver:               resolver.New(settings, cont),
		settings:               settings,
		ctx:                    cont,
		db:                     db,
		prom:                   prom,
		contractActionsCounter: helpers.NewCounter(0),
		tokenActionsCounter:    helpers.NewCounter(0),
	}

	if aws := storage.NewAWS(settings.AWS.AccessKey, settings.AWS.Secret, settings.AWS.Region, settings.AWS.BucketName); aws != nil {
		indexer.thumbnail = thumbnail.New(
			aws, db, network, settings.IPFSGateways,
			thumbnail.WithPrometheus(prom),
			thumbnail.WithWorkers(settings.Thumbnail.Workers),
			thumbnail.WithFileSizeLimit(settings.Thumbnail.MaxFileSize),
			thumbnail.WithSize(settings.Thumbnail.Size),
		)
	}
	indexer.contracts = service.NewContractService(db, indexer.resolveContractMetadata)
	indexer.tokens = service.NewTokenService(db, indexer.resolveTokenMetadata)

	return indexer, nil
}

// Start -
func (indexer *Indexer) Start(ctx context.Context) error {
	if err := indexer.initState(); err != nil {
		return err
	}

	if err := indexer.ctx.Load(indexer.db); err != nil {
		return err
	}

	if indexer.thumbnail != nil {
		indexer.thumbnail.Start(ctx)
	}

	indexer.contracts.Start(ctx)
	indexer.tokens.Start(ctx)

	indexer.wg.Add(1)
	go indexer.listen(ctx)

	indexer.scanner.Start(ctx, indexer.state.Level)

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

	if err := indexer.ctx.Dump(indexer.db); err != nil {
		return err
	}

	if err := indexer.db.Close(); err != nil {
		return err
	}

	return nil
}

func (indexer *Indexer) initState() error {
	current, err := indexer.db.State(indexer.indexName)
	if err != nil {
		if !errors.Is(err, pg.ErrNoRows) {
			return err
		}
		indexer.state = database.State{
			IndexType: models.IndexTypeMetadata,
			IndexName: indexer.indexName,
		}
	} else {
		indexer.state = current

		if err := indexer.initCounters(); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) initCounters() error {
	contractActionsCounter, err := indexer.db.LastContractUpdateID()
	if err != nil {
		return err
	}
	indexer.contractActionsCounter.Set(contractActionsCounter)

	tokenActionsCounter, err := indexer.db.LastTokenUpdateID()
	if err != nil {
		return err
	}
	indexer.tokenActionsCounter.Set(tokenActionsCounter)

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
				log.Err(err)
			} else {
				indexer.log().Msg("New level")
			}
		case level := <-indexer.scanner.Blocks():
			if level-indexer.state.Level > 1 {
				indexer.state.Level = level - 1
				if err := indexer.db.UpdateState(indexer.state); err != nil {
					log.Err(err).Msg("")
				} else {
					indexer.log().Msg("New level")
				}
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
				contracts = append(contracts, contract)
			}
		}
	}

	if err := indexer.db.SaveContractMetadata(ctx, contracts); err != nil {
		return err
	}
	if err := indexer.db.SaveTokenMetadata(ctx, tokens); err != nil {
		return err
	}

	indexer.state.Level = msg.Level
	return indexer.db.UpdateState(indexer.state)
}

func (indexer *Indexer) incrementCounter(typ string, status models.Status) {
	if indexer.prom == nil {
		return
	}
	indexer.prom.IncrementCounter(metricMetadataCounter, map[string]string{
		"network": indexer.network,
		"type":    typ,
		"status":  status.String(),
	})
}

func (indexer *Indexer) incrementErrorCounter(err resolver.ResolvingError) {
	if indexer.prom == nil {
		return
	}
	indexer.prom.IncrementCounter(metricsMetadataHttpErrors, map[string]string{
		"network": indexer.network,
		"type":    string(err.Type),
		"code":    strconv.FormatInt(int64(err.Code), 10),
	})
}
