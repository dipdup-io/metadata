package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

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
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	args := cmdline.Parse()
	if args.Help {
		return
	}

	cfg, err := config.Load(args.Config)
	if err != nil {
		log.Error(err)
		return
	}
	runtime.GOMAXPROCS(int(cfg.Metadata.Settings.MaxCPU))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	prometheusService := initPrometheus(cfg.Prometheus)

	indexers := make(map[string]*Indexer)
	indexerCancels := make(map[string]context.CancelFunc)

	var hasuraInit sync.Once
	for network, indexer := range cfg.Metadata.Indexers {
		go func(network string, ind *config.Indexer) {
			result, err := startIndexer(ctx, cfg, *ind, network, prometheusService)
			if err != nil {
				log.Error(err)
			} else {
				indexers[network] = result.indexer
				indexerCancels[network] = result.cancel
				hasuraInit.Do(func() {
					if err := hasura.Create(ctx, cfg.Hasura, cfg.Database, nil, new(models.TokenMetadata), new(models.ContractMetadata)); err != nil {
						log.Error(err)
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
						log.Error(err)
					} else {
						indexers[network] = result.indexer
						indexerCancels[network] = result.cancel
						hasuraInit.Do(func() {
							if err := hasura.Create(ctx, cfg.Hasura, cfg.Database, nil, new(models.TokenMetadata), new(models.ContractMetadata)); err != nil {
								log.Error(err)
							}
						})
						return
					}
				}
			}
		}(network, indexer)
	}

	<-signals

	for newtork, cancelIndexer := range indexerCancels {
		log.Infof("stopping %s indexer...", newtork)
		cancelIndexer()
	}

	log.Warn("Trying carefully stopping....")
	for _, indexer := range indexers {
		if err := indexer.Close(); err != nil {
			log.Error(err)
			return
		}
	}

	if prometheusService != nil {
		if err := prometheusService.Close(); err != nil {
			log.Error(err)
		}
	}

	close(signals)
}

func initPrometheus(cfg *golibConfig.Prometheus) *prometheus.Service {
	prometheusService := prometheus.NewService(cfg)

	prometheusService.RegisterGoBuildMetrics()
	prometheusService.RegisterCounter(metricMetadataCounter, "Count of metadata", "type", "status", "network")
	prometheusService.RegisterCounter(metricsMetadataHttpErrors, "Count of HTTP errors in metadata", "network", "code", "type")
	prometheusService.RegisterCounter(metricsMetadataMimeType, "Count of metadata mime types", "network", "type", "mime")

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
