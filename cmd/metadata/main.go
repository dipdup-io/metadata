package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/dipdup-net/go-lib/cmdline"
	"github.com/dipdup-net/metadata/cmd/metadata/config"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	args := cmdline.Parse()
	if args.Help {
		return
	}
	runtime.GOMAXPROCS(4)

	cfg, err := config.Load(args.Config)
	if err != nil {
		log.Error(err)
		return
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	indexers := make(map[string]*Indexer)

	for network, indexer := range cfg.Metadata.Indexers {
		indexer, err := NewIndexer(network, indexer, cfg.Database, indexer.Filters, cfg.Metadata.Settings)
		if err != nil {
			log.Error(err)
			return
		}
		indexers[network] = indexer

		if err := indexer.Start(); err != nil {
			log.Error(err)
			return
		}
	}

	<-signals

	log.Warn("Trying carefully stopping....")
	for _, indexer := range indexers {
		if err := indexer.Close(); err != nil {
			log.Error(err)
			return
		}
	}

	close(signals)
}
