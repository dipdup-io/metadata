package models

import (
	"fmt"

	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/state"
	"gorm.io/gorm"
)

// index type
const (
	IndexTypeMetadata = "metadata"
)

// IndexName -
func IndexName(network string) string {
	return fmt.Sprintf("%s_%s", IndexTypeMetadata, network)
}

// OpenDatabaseConnection -
func OpenDatabaseConnection(cfg config.Database) (*gorm.DB, error) {
	db, err := state.OpenConnection(cfg)
	if err != nil {
		return nil, err
	}

	sql, err := db.DB()
	if err != nil {
		return nil, err
	}

	if cfg.Kind == config.DBKindSqlite {
		sql.SetMaxOpenConns(1)
	}

	if err := db.AutoMigrate(&state.State{}, &ContractMetadata{}, &TokenMetadata{}, &ContextItem{}); err != nil {
		if err := sql.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}
	return db, nil
}
