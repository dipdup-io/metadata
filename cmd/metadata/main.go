package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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

	ctx, cancel := context.WithCancel(context.Background())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	if err := hasura.Create(ctx, cfg.Hasura, cfg.Database, nil, &models.TokenMetadata{}, &models.ContractMetadata{}); err != nil {
		log.Error(err)
		return
	}

	prometheusService := initPrometheus(cfg.Prometheus)

	indexers := make(map[string]*Indexer)
	for network, indexer := range cfg.Metadata.Indexers {
		indexer, err := NewIndexer(ctx, network, indexer, cfg.Database, indexer.Filters, cfg.Metadata.Settings, prometheusService)
		if err != nil {
			log.Error(err)
			return
		}
		indexers[network] = indexer

		if err := indexer.Start(ctx); err != nil {
			log.Error(err)
			return
		}
	}

	<-signals

	cancel()

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
