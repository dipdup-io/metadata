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
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
	"github.com/dipdup-net/metadata/cmd/metadata/service"
	"github.com/dipdup-net/metadata/cmd/metadata/storage"
	"github.com/dipdup-net/metadata/cmd/metadata/tezoskeys"
	"github.com/dipdup-net/metadata/cmd/metadata/thumbnail"
	"github.com/dipdup-net/metadata/cmd/metadata/tzkt"
)

var createIndex sync.Once

// Indexer -
type Indexer struct {
	network   string
	indexName string
	state     *database.State
	resolver  resolver.Receiver
	db        models.Database
	scanner   *tzkt.Scanner
	prom      *prometheus.Service
	tezosKeys *tezoskeys.TezosKeys
	contracts *service.ContractService
	tokens    *service.TokenService
	thumbnail *thumbnail.Service
	settings  config.Settings

	wg sync.WaitGroup
}

// NewIndexer -
func NewIndexer(ctx context.Context, network string, indexerConfig *config.Indexer, database generalConfig.Database, filters config.Filters, settings config.Settings, prom *prometheus.Service) (*Indexer, error) {
	db, err := models.NewDatabase(ctx, database)
	if err != nil {
		return nil, err
	}
	keys := tezoskeys.NewTezosKeys(db)

	metadataResolver, err := resolver.New(settings, keys)
	if err != nil {
		return nil, err
	}

	indexer := &Indexer{
		scanner:   tzkt.New(indexerConfig.DataSource.Tzkt, filters.Accounts...),
		network:   network,
		indexName: models.IndexName(network),
		resolver:  metadataResolver,
		settings:  settings,
		tezosKeys: keys,
		db:        db,
		prom:      prom,
	}

	if aws := storage.NewAWS(settings.AWS); aws != nil {
		indexer.thumbnail = thumbnail.New(
			aws, db, network, settings.IPFS.Gateways,
			thumbnail.WithPrometheus(prom),
			thumbnail.WithWorkers(settings.Thumbnail.Workers),
			thumbnail.WithFileSizeLimit(settings.Thumbnail.MaxFileSize),
			thumbnail.WithSize(settings.Thumbnail.Size),
			thumbnail.WithTimeout(settings.Thumbnail.Timeout),
		)
	}
	indexer.contracts = service.NewContractService(
		db, indexer.resolveContractMetadata, network,
		service.WithMaxRetryCountContract(settings.MaxRetryCountOnError),
		service.WithWorkersCountContract(settings.ContractServiceWorkers),
		service.WithPrometheusContract(prom),
	)
	indexer.tokens = service.NewTokenService(
		db, indexer.resolveTokenMetadata, network,
		service.WithMaxRetryCountToken(settings.MaxRetryCountOnError),
		service.WithWorkersCountToken(settings.TokenServiceWorkers),
		service.WithPrometheusToken(prom),
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
	if err := indexer.initState(); err != nil {
		return err
	}

	if indexer.thumbnail != nil {
		indexer.thumbnail.Start(ctx)
	}

	if indexer.prom != nil {
		newContractCount, err := indexer.db.CountContractsByStatus(indexer.network, models.StatusNew)
		if err != nil {
			return err
		}
		indexer.prom.SetGaugeValue(metricMetadataNew, map[string]string{
			"network": indexer.network,
			"type":    "contract",
		}, float64(newContractCount))

		newTokenCount, err := indexer.db.CountTokensByStatus(indexer.network, models.StatusNew)
		if err != nil {
			return err
		}
		indexer.prom.SetGaugeValue(metricMetadataNew, map[string]string{
			"network": indexer.network,
			"type":    "token",
		}, float64(newTokenCount))
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
	return nil
}

func (indexer *Indexer) initCounters() error {
	contractActionsCounter, err := indexer.db.LastContractUpdateID()
	if err != nil {
		return err
	}
	models.ContractUpdateID.Set(contractActionsCounter)

	tokenActionsCounter, err := indexer.db.LastTokenUpdateID()
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
				indexer.incrementNewMetadataGauge("contract")
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
	return nil
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

func (indexer *Indexer) addHistogramResponseTime(data resolver.Resolved) {
	if indexer.prom == nil {
		return
	}
	indexer.prom.AddHistogramValue(metricsMetadataIPFSResponseTime, map[string]string{
		"network": indexer.network,
		"node":    data.Node,
	}, float64(len(data.Data))/float64(data.ResponseTime))
}

func (indexer *Indexer) incrementNewMetadataGauge(typ string) {
	if indexer.prom == nil {
		return
	}
	indexer.prom.IncGaugeValue(metricMetadataNew, map[string]string{
		"network": indexer.network,
		"type":    typ,
	})
}
