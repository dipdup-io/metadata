package main

import (
	"strings"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	generalConfig "github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/state"
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/context"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
	"github.com/dipdup-net/metadata/cmd/metadata/storage"
	"github.com/dipdup-net/metadata/cmd/metadata/tzkt"
)

// Indexer -
type Indexer struct {
	network          string
	indexName        string
	state            state.State
	resolver         resolver.Receiver
	db               *gorm.DB
	scanner          *tzkt.Scanner
	ctx              *context.Context
	contracts        *Queue
	tokens           *Queue
	thumbnailCreator *ThumbnailCreator
	settings         config.Settings

	stop chan struct{}
	wg   sync.WaitGroup
}

// NewIndexer -
func NewIndexer(network string, indexerConfig *config.Indexer, database generalConfig.Database, filters config.Filters, settings config.Settings) (*Indexer, error) {
	db, err := models.OpenDatabaseConnection(database)
	if err != nil {
		return nil, err
	}
	ctx := context.NewContext()

	log.Infof("Indices which will be processed: %s", strings.Join(settings.Index, ", "))

	rslvr, err := resolver.New(db, settings, ctx)
	if err != nil {
		return nil, err
	}

	indexer := &Indexer{
		scanner:   tzkt.New(indexerConfig.DataSource.Tzkt, filters.Accounts...),
		network:   network,
		indexName: models.IndexName(network),
		resolver:  rslvr,
		settings:  settings,
		ctx:       ctx,
		db:        db,
		stop:      make(chan struct{}, 1),
	}

	if aws := storage.NewAWS(settings.AWS.AccessKey, settings.AWS.Secret, settings.AWS.Region, settings.AWS.BucketName); aws != nil {
		indexer.thumbnailCreator = NewThumbnailCreator(aws, db, settings.IPFSGateways)
	}
	indexer.contracts = NewQueue(db, 15, 60, indexer.onContractFlush, indexer.onContractTick)
	indexer.tokens = NewQueue(db, 15, 60, indexer.onTokenFlush, indexer.onTokenTick)

	return indexer, nil
}

// Start -
func (indexer *Indexer) Start() error {
	if err := indexer.initState(); err != nil {
		return err
	}

	if err := indexer.ctx.Load(indexer.db); err != nil {
		return err
	}

	if indexer.thumbnailCreator != nil {
		indexer.thumbnailCreator.Start()
	}
	indexer.contracts.Start()
	indexer.tokens.Start()

	indexer.wg.Add(1)
	go indexer.listen()

	go indexer.scanner.Start(indexer.state.Level)

	return nil
}

// Close -
func (indexer *Indexer) Close() error {
	indexer.stop <- struct{}{}
	indexer.wg.Wait()

	if indexer.thumbnailCreator != nil {
		if err := indexer.thumbnailCreator.Close(); err != nil {
			return err
		}
	}

	if err := indexer.scanner.Close(); err != nil {
		return err
	}

	if err := indexer.contracts.Close(); err != nil {
		return err
	}

	if err := indexer.tokens.Close(); err != nil {
		return err
	}

	if err := indexer.ctx.Dump(indexer.db); err != nil {
		return err
	}

	sqlDB, err := indexer.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}

	close(indexer.stop)

	return nil
}

func (indexer *Indexer) initState() error {
	current, err := state.Get(indexer.db, indexer.indexName)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		indexer.state = state.State{
			IndexType: models.IndexTypeMetadata,
			IndexName: indexer.indexName,
		}
	} else {
		indexer.state = current
	}
	return nil
}

func (indexer *Indexer) log() *log.Entry {
	return log.WithField("state", indexer.state.Level).WithField("name", indexer.indexName)
}

func (indexer *Indexer) listen() {
	defer indexer.wg.Done()

	for {
		select {
		case <-indexer.stop:
			return
		case msg := <-indexer.scanner.BigMaps():
			if err := indexer.handlerUpdate(msg); err != nil {
				log.Error(err)
			} else {
				indexer.log().Infof("New level %d", msg.Level)
			}
		case level := <-indexer.scanner.Blocks():
			if level-indexer.state.Level > 1 {
				indexer.state.Level = level - 1
				if err := indexer.state.Update(indexer.db); err != nil {
					log.Error(err)
				} else {
					indexer.log().Infof("New level %d", indexer.state.Level)
				}
			}
		}
	}
}

func (indexer *Indexer) handlerUpdate(msg tzkt.Message) error {
	return indexer.db.Transaction(func(tx *gorm.DB) error {
		for i := range msg.Body {
			path := strings.Split(msg.Body[i].Path, ".")

			switch path[len(path)-1] {
			case "token_metadata":
				if err := indexer.processTokenMetadata(msg.Body[i], tx); err != nil {
					return errors.Wrap(err, "token_metadata")
				}
			case "metadata":
				if err := indexer.processContractMetadata(msg.Body[i], tx); err != nil {
					return errors.Wrap(err, "contract_metadata")
				}
			}
		}

		indexer.state.Level = msg.Level
		return indexer.state.Update(tx)
	})
}
