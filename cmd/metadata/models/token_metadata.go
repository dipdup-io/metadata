package models

import (
	"context"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/shopspring/decimal"
)

// TokenUpdateID - incremental counter
var TokenUpdateID = helpers.NewCounter(0)

// TokenMetadata -
type TokenMetadata struct {
	//nolint
	tableName struct{} `pg:"token_metadata"`

	ID             uint64          `json:"-"`
	CreatedAt      int64           `json:"created_at"`
	UpdatedAt      int64           `json:"updated_at"`
	UpdateID       int64           `json:"-" pg:",use_zero,notnull"`
	TokenID        decimal.Decimal `json:"token_id" pg:",type:numeric,unique:token,use_zero"`
	Network        string          `json:"network" pg:",unique:token"`
	Contract       string          `json:"contract" pg:",unique:token"`
	Link           string          `json:"link"`
	Metadata       JSONB           `json:"metadata,omitempty" pg:",type:jsonb,use_zero"`
	RetryCount     int8            `json:"retry_count" pg:",use_zero"`
	Status         Status          `json:"status"`
	ImageProcessed bool            `json:"image_processed" pg:",use_zero,notnull"`
}

// Table -
func (TokenMetadata) TableName() string {
	return "token_metadata"
}

// BeforeInsert -
func (tm *TokenMetadata) BeforeInsert(ctx context.Context) (context.Context, error) {
	tm.UpdatedAt = time.Now().Unix()
	tm.CreatedAt = tm.UpdatedAt
	tm.UpdateID = TokenUpdateID.Increment()
	return ctx, nil
}

// BeforeUpdate -
func (tm *TokenMetadata) BeforeUpdate(ctx context.Context) (context.Context, error) {
	tm.UpdatedAt = time.Now().Unix()
	tm.UpdateID = TokenUpdateID.Increment()
	return ctx, nil
}

// TokenRepository -
type TokenRepository interface {
	GetTokenMetadata(network string, status Status, limit, offset, retryCount int) ([]TokenMetadata, error)
	SetImageProcessed(token TokenMetadata) error
	GetUnprocessedImage(from uint64, limit int) ([]TokenMetadata, error)
	UpdateTokenMetadata(ctx context.Context, metadata []*TokenMetadata) error
	SaveTokenMetadata(ctx context.Context, metadata []*TokenMetadata) error
	LastTokenUpdateID() (int64, error)
	CountTokensByStatus(network string, status Status) (int, error)
}
