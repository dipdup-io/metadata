package main

import (
	"os"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/dipdup-net/go-lib/cmdline"
	"github.com/dipdup-net/go-lib/config"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var es *elasticsearch.Client

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	args := cmdline.Parse()
	if args.Help {
		return
	}

	cfg, err := config.Load(args.Config)
	if err != nil {
		log.Err(err).Msg("")
		return
	}

	if cfg.Database.Kind != "elastic" {
		log.Error().Msgf("Invalid database kind: want=elastic got=%s", cfg.Database.Kind)
		return
	}

	elastic, err := createElastic(cfg.Database.Path)
	if err != nil {
		log.Err(err).Msg("")
		return
	}
	es = elastic

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/search", search)

	// Start server
	e.Logger.Fatal(e.Start(":11111"))
}

func createElastic(path string) (*elasticsearch.Client, error) {
	retryBackoff := backoff.NewExponentialBackOff()
	elasticConfig := elasticsearch.Config{
		Addresses:     []string{path},
		RetryOnStatus: []int{502, 503, 504, 429},
		RetryBackoff: func(i int) time.Duration {
			if i == 1 {
				retryBackoff.Reset()
			}
			return retryBackoff.NextBackOff()
		},
		MaxRetries: 5,
	}

	elastic, err := elasticsearch.NewClient(elasticConfig)
	if err != nil {
		return nil, err
	}
	response, err := elastic.Ping()
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return elastic, nil
}
