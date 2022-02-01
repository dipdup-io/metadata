package models

import (
	"context"
	"io"

	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/database"
)

// Database -
type Database interface {
	ContractRepository
	TokenRepository
	TezosKeyRepository
	IPFSLinkRepository
	database.StateRepository
	io.Closer

	CreateIndices() error
	Exec(sql string) error
}

// NewDatabase -
func NewDatabase(ctx context.Context, cfg config.Database) (Database, error) {
	switch cfg.Kind {
	case "elastic":
		return NewElastic(cfg)
	default:
		return NewRelativeDatabase(ctx, cfg)
	}
}
