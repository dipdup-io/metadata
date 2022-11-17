package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dipdup-net/go-lib/cmdline"
	golibConfig "github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/hasura"
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/prometheus"
	"github.com/dipdup-net/metadata/internal/ipfs"
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
	runtime.GOMAXPROCS(cfg.Metadata.Settings.MaxCPU)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	prometheusService := prometheus.NewPrometheus(cfg.Prometheus)
	if prometheusService != nil {
		prometheusService.Start()
	}

	err = execScripts(ctx, cfg.Database)
	if err != nil {
		log.Err(err).Msg("execScripts")
		return
	}

	views, err := createViews(ctx, cfg.Database)
	if err != nil {
		log.Err(err).Msg("createViews")
		return
	}

	custom_configs, err := hasura.ReadCustomConfigs(ctx, cfg.Database, "custom_hasura_config")
	if err != nil {
		log.Err(err).Msg("readCustomHasuraConfigs")
		return
	}

	ipfsNode, err := ipfs.NewNode(ctx, cfg.Metadata.Settings.IPFS.Dir, 1024*1024, cfg.Metadata.Settings.IPFS.Blacklist)
	if err != nil {
		log.Err(err).Msg("ipfs.NewNode")
		return
	}

	if err := ipfsNode.Start(ctx); err != nil {
		log.Err(err).Msg("ipfs.Start")
		return
	}

	var indexers sync.Map
	var indexerCancels sync.Map

	var hasuraInit sync.Once
	for network, indexer := range cfg.Metadata.Indexers {
		go func(network string, ind *config.Indexer) {
			result, err := startIndexer(ctx, cfg, *ind, network, prometheusService, ipfsNode, views, custom_configs, &hasuraInit)
			if err != nil {
				log.Err(err).Msg("")
			} else {
				indexers.Store(network, result.indexer)
				indexerCancels.Store(network, result.cancel)
				return
			}

			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					result, err := startIndexer(ctx, cfg, *ind, network, prometheusService, ipfsNode, views, custom_configs, &hasuraInit)
					if err != nil {
						log.Err(err).Msg("")
					} else {
						indexers.Store(network, result.indexer)
						indexerCancels.Store(network, result.cancel)
						return
					}
				}
			}
		}(network, indexer)
	}

	<-signals

	indexerCancels.Range(func(key, value interface{}) bool {
		log.Info().Msgf("stopping %s indexer...", key)
		cancelIndexer := value.(context.CancelFunc)
		cancelIndexer()
		return true
	})

	log.Warn().Msgf("Trying carefully stopping....")
	indexers.Range(func(key, value interface{}) bool {
		if err := value.(*Indexer).Close(); err != nil {
			log.Err(err).Msgf("%T.Close()", value)
		}
		return err == nil
	})

	if err := ipfsNode.Close(); err != nil {
		log.Err(err).Msgf("ipfsNode.Close()")
	}

	if prometheusService != nil {
		if err := prometheusService.Close(); err != nil {
			log.Err(err).Msg("prometheusService.Close()")
		}
	}

	close(signals)
}

func startIndexer(ctx context.Context, cfg config.Config, indexerConfig config.Indexer, network string, prom *prometheus.Prometheus, ipfsNode *ipfs.Node, views []string, customConfigs []hasura.Request, hasuraInit *sync.Once) (startResult, error) {
	var result startResult
	indexerCtx, cancel := context.WithCancel(ctx)

	indexer, err := NewIndexer(indexerCtx, network, &indexerConfig, cfg.Database, indexerConfig.Filters, cfg.Metadata.Settings, prom, ipfsNode)
	if err != nil {
		cancel()
		return result, err
	}
	result.indexer = indexer

	if err := indexer.Start(indexerCtx); err != nil {
		cancel()
		return result, err
	}

	hasuraInit.Do(func() {
		if err := hasura.Create(ctx, hasura.GenerateArgs{
			Config:               cfg.Hasura,
			DatabaseConfig:       cfg.Database,
			Views:                views,
			CustomConfigurations: customConfigs,
			Models:               []any{new(models.TokenMetadata), new(models.ContractMetadata)},
		}); err != nil {
			log.Err(err).Msg("hasura.Create")
		}
	})

	result.cancel = cancel
	return result, nil
}

func createViews(ctx context.Context, database golibConfig.Database) ([]string, error) {
	files, err := os.ReadDir("views")
	if err != nil {
		return nil, err
	}

	db, err := models.NewDatabase(ctx, database)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	views := make([]string, 0)
	for i := range files {
		if files[i].IsDir() {
			continue
		}

		path := fmt.Sprintf("views/%s", files[i].Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		if err := db.Exec(string(raw)); err != nil {
			return nil, err
		}
		views = append(views, strings.Split(files[i].Name(), ".")[0])
	}

	return views, nil
}

func execScripts(ctx context.Context, database golibConfig.Database) error {
	files, err := os.ReadDir("sql")
	if err != nil {
		return err
	}

	db, err := models.NewDatabase(ctx, database)
	if err != nil {
		return err
	}
	defer db.Close()

	for i := range files {
		if files[i].IsDir() {
			continue
		}

		path := fmt.Sprintf("sql/%s", files[i].Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := db.Exec(string(raw)); err != nil {
			return err
		}
	}

	return nil
}
