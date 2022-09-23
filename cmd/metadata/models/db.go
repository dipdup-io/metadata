package models

import (
	"context"
	"fmt"
	"time"

	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/database"
	pg "github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/rs/zerolog/log"
)

// index type
const (
	IndexTypeMetadata = "metadata"
)

// IndexName -
func IndexName(network string) string {
	return fmt.Sprintf("%s_%s", IndexTypeMetadata, network)
}

// Database -
type Database struct {
	*database.PgGo

	Tokens    ModelRepository[*TokenMetadata]
	Contracts ModelRepository[*ContractMetadata]
	TezosKeys *TezosKeys
}

// NewDatabase -
func NewDatabase(ctx context.Context, cfg config.Database) (*Database, error) {
	db := database.NewPgGo()
	if err := db.Connect(ctx, cfg); err != nil {
		return nil, err
	}

	database.Wait(ctx, db, 5*time.Second)

	for _, data := range []interface{}{
		&database.State{}, &ContractMetadata{}, &TokenMetadata{}, &TezosKey{},
	} {
		if err := db.DB().WithContext(ctx).Model(data).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
		}); err != nil {
			if err := db.Close(); err != nil {
				return nil, err
			}
			return nil, err
		}
	}
	db.DB().AddQueryHook(&dbLogger{})

	return &Database{
		PgGo:      db,
		Tokens:    NewTokens(db),
		Contracts: NewContracts(db),
		TezosKeys: NewTezosKeys(db),
	}, nil
}

type dbLogger struct{}

// BeforeQuery -
func (d *dbLogger) BeforeQuery(ctx context.Context, event *pg.QueryEvent) (context.Context, error) {
	event.StartTime = time.Now()
	return ctx, nil
}

func (d *dbLogger) AfterQuery(ctx context.Context, event *pg.QueryEvent) error {
	query, err := event.FormattedQuery()
	if err != nil {
		return err
	}

	if event.Err != nil {
		log.Error().Msgf("[%d ms] %s : %s", time.Since(event.StartTime).Milliseconds(), event.Err.Error(), string(query))
	} else {
		log.Debug().Msgf("[%d ms] %d rows | %s", time.Since(event.StartTime).Milliseconds(), event.Result.RowsReturned(), string(query))
	}
	return nil
}

// Close -
func (db *Database) Close() error {
	return db.PgGo.Close()
}

// CreateIndices -
func (db *Database) CreateIndices() error {
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_network_status_idx ON contract_metadata (network, status)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_idx ON contract_metadata (network, contract)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_sort_idx ON contract_metadata (retry_count, updated_at)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_update_id_idx ON contract_metadata (update_id)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_network_status_idx ON token_metadata (network, status)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_sort_idx ON token_metadata (retry_count, updated_at)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_idx ON token_metadata (network, contract, token_id)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_update_id_idx ON token_metadata (update_id)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS tezos_key_idx ON tezos_keys (network, address, key)
	`); err != nil {
		return err
	}
	return nil
}

// Exec -
func (db *Database) Exec(sql string) error {
	_, err := db.DB().Exec(sql)
	return err
}
