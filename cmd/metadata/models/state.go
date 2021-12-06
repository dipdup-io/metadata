package models

import (
	"context"
	"fmt"
	"time"

	pg "github.com/go-pg/pg/v10"
)

// StateRepository -
type StateRepository interface {
	GetState(indexName string) (State, error)
	UpdateState(s State) error
}

const (
	IndexTypeMempool = "mempool"
)

// MempoolIndexName -
func MempoolIndexName(network string) string {
	return fmt.Sprintf("%s_%s", IndexTypeMempool, network)
}

// State -
type State struct {
	//nolint
	tableName struct{} `pg:"dipdup_state"`
	IndexName string   `pg:",pk" json:"index_name"`
	IndexType string   `json:"index_type"`
	Hash      string   `json:"hash,omitempty"`
	Level     uint64   `json:"level"`
	UpdatedAt int64
}

// BeforeInsert -
func (s *State) BeforeInsert(ctx context.Context) (context.Context, error) {
	s.UpdatedAt = time.Now().Unix()
	return ctx, nil
}

// BeforeUpdate -
func (s *State) BeforeUpdate(ctx context.Context) (context.Context, error) {
	s.UpdatedAt = time.Now().Unix()
	return ctx, nil
}

// TableName -
func (State) TableName() string {
	return "dipdup_state"
}

// UpdateState -
func (s State) Update(db pg.DBI) error {
	_, err := db.Model(&s).
		OnConflict("(index_name) DO UPDATE").
		Set("hash = EXCLUDED.hash").
		Set("level = EXCLUDED.level").
		Set("updated_at = EXCLUDED.updated_at").
		Insert()
	return err
}

// GetState -
func GetState(db pg.DBI, indexName string) (State, error) {
	var state State
	err := db.Model(&state).Where("index_name = ?", indexName).Limit(1).Select()
	return state, err
}
