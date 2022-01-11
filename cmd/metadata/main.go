package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dipdup-net/go-lib/cmdline"
	golibConfig "github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/hasura"
	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
)

const (
	metricMetadataCounter     = "metadata_counter"
	metricsMetadataHttpErrors = "metadata_http_errors"
	metricsMetadataMimeType   = "metadata_mime_type"
)

type startResult struct {
	cancel  context.CancelFunc
	indexer *Indexer
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "2006-01-02 15:04:05",
	}).Level(zerolog.InfoLevel)

	args := cmdline.Parse()
	if args.Help {
		return
	}

	cfg, err := config.Load(args.Config)
	if err != nil {
		log.Err(err).Msg("")
		return
	}
	runtime.GOMAXPROCS(int(cfg.Metadata.Settings.MaxCPU))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	prometheusService := initPrometheus(cfg.Prometheus)

	var indexers sync.Map
	var indexerCancels sync.Map

	var hasuraInit sync.Once
	for network, indexer := range cfg.Metadata.Indexers {
		go func(network string, ind *config.Indexer) {
			result, err := startIndexer(ctx, cfg, *ind, network, prometheusService)
			if err != nil {
				log.Err(err).Msg("")
			} else {
				indexers.Store(network, result.indexer)
				indexerCancels.Store(network, result.cancel)
				hasuraInit.Do(func() {
					if err := hasura.Create(ctx, cfg.Hasura, cfg.Database, nil, new(models.TokenMetadata), new(models.ContractMetadata)); err != nil {
						log.Err(err).Msg("")
					}
				})
				return
			}

			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					result, err := startIndexer(ctx, cfg, *ind, network, prometheusService)
					if err != nil {
						log.Err(err).Msg("")
					} else {
						indexers.Store(network, result.indexer)
						indexerCancels.Store(network, result.cancel)
						hasuraInit.Do(func() {
							if err := hasura.Create(ctx, cfg.Hasura, cfg.Database, nil, new(models.TokenMetadata), new(models.ContractMetadata)); err != nil {
								log.Err(err).Msg("")
							}
						})
						return
					}
				}
			}
		}(network, indexer)
	}

	<-signals

	indexerCancels.Range(func(key, value interface{}) bool {
		log.Info().Msgf("stopping %s indexer...", key.(string))
		cancelIndexer := value.(context.CancelFunc)
		cancelIndexer()
		return true
	})

	log.Warn().Msgf("Trying carefully stopping....")
	indexers.Range(func(key, value interface{}) bool {
		if err := value.(*Indexer).Close(); err != nil {
			log.Err(err).Msg("")
		}
		return err == nil
	})

	if prometheusService != nil {
		if err := prometheusService.Close(); err != nil {
			log.Err(err).Msg("")
		}
	}

	close(signals)
}

func initPrometheus(cfg *golibConfig.Prometheus) *prometheus.Service {
	prometheusService := prometheus.NewService(cfg)

	prometheusService.RegisterGoBuildMetrics()
	prometheusService.RegisterCounter(metricMetadataCounter, "Count of metadata", "type", "status", "network")
	prometheusService.RegisterCounter(metricsMetadataHttpErrors, "Count of HTTP errors in metadata", "network", "code", "type")
	prometheusService.RegisterCounter(metricsMetadataMimeType, "Count of metadata mime types", "network", "mime")

	prometheusService.Start()
	return prometheusService
}

func startIndexer(ctx context.Context, cfg config.Config, indexerConfig config.Indexer, network string, prom *prometheus.Service) (startResult, error) {
	var result startResult
	indexerCtx, cancel := context.WithCancel(ctx)

	indexer, err := NewIndexer(indexerCtx, network, &indexerConfig, cfg.Database, indexerConfig.Filters, cfg.Metadata.Settings, prom)
	if err != nil {
		cancel()
		return result, err
	}
	result.indexer = indexer

	if err := indexer.Start(indexerCtx); err != nil {
		cancel()
		return result, err
	}

	result.cancel = cancel
	return result, nil
}
