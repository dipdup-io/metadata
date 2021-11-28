package models

import (
	"context"
	"io"

	"github.com/dipdup-net/go-lib/config"
)

// Database -
type Database interface {
	ContractRepository
	TokenRepository
	ContextRepository
	StateRepository
	io.Closer
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

// Transactable -
type Transactable interface {
	BeginTx()
	RollbackTx() error
	CommitTx() error
}
