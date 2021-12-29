package models

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// TokenMetadata -
type TokenMetadata struct {
	//nolint
	tableName struct{} `pg:"token_metadata"`

	ID             uint64          `gorm:"autoIncrement;not null;" json:"-"`
	CreatedAt      int64           `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      int64           `gorm:"autoUpdateTime" json:"updated_at"`
	UpdateID       int64           `gorm:"uniqueIndex:token_metadata_update_id_key;autoIncrement:false;not null;" json:"-" pg:",use_zero,notnull"`
	TokenID        decimal.Decimal `gorm:"primaryKey" json:"token_id" pg:",type:numeric,unique:token,use_zero"`
	Network        string          `gorm:"primaryKey" json:"network" pg:",unique:token"`
	Contract       string          `gorm:"primaryKey" json:"contract" pg:",unique:token"`
	Link           string          `json:"link"`
	Metadata       JSONB           `json:"metadata,omitempty" pg:",type:jsonb,use_zero"`
	RetryCount     int8            `gorm:"type:SMALLINT;default:0" json:"retry_count" pg:",use_zero"`
	Status         Status          `gorm:"type:SMALLINT" json:"status"`
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
	return ctx, nil
}

// BeforeUpdate -
func (tm *TokenMetadata) BeforeUpdate(ctx context.Context) (context.Context, error) {
	tm.UpdatedAt = time.Now().Unix()
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
}
