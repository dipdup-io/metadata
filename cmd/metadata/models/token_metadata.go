package models

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dipdup-net/go-lib/database"
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
	Metadata       JSONB           `json:"metadata,omitempty" pg:",type:json,use_zero"`
	RetryCount     int8            `json:"retry_count" pg:",use_zero"`
	Status         Status          `json:"status"`
	ImageProcessed bool            `json:"image_processed" pg:",use_zero,notnull"`
	Error          string          `json:"error,omitempty"`
}

// Table -
func (TokenMetadata) TableName() string {
	return "token_metadata"
}

// GetStatus -
func (tm TokenMetadata) GetStatus() Status {
	return tm.Status
}

// GetRetryCount -
func (tm TokenMetadata) GetRetryCount() int8 {
	return tm.RetryCount
}

// GetID -
func (tm TokenMetadata) GetID() uint64 {
	return tm.ID
}

// GetLink -
func (tm TokenMetadata) GetLink() string {
	return tm.Link
}

// IncrementRetryCount -
func (tm *TokenMetadata) IncrementRetryCount() {
	tm.RetryCount += 1
}

// SetMetadata -
func (tm *TokenMetadata) SetMetadata(data JSONB) {
	tm.Metadata = data
}

// SetStatus -
func (tm *TokenMetadata) SetStatus(status Status) {
	tm.Status = status
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
	ModelRepository[*TokenMetadata]

	SetImageProcessed(token TokenMetadata) error
	GetUnprocessedImage(from uint64, limit int) ([]TokenMetadata, error)
}

// Tokens -
type Tokens struct {
	db *database.PgGo

	mx sync.Mutex
}

// NewTokens -
func NewTokens(db *database.PgGo) *Tokens {
	return &Tokens{db: db}
}

// Get -
func (tokens *Tokens) Get(network string, status Status, limit, offset, retryCount, delay int) (all []*TokenMetadata, err error) {
	subQuery := tokens.db.DB().Model((*TokenMetadata)(nil)).Column("id").
		Where("status = ?", status).
		Where("network = ?", network).
		Where("created_at < (extract(epoch from current_timestamp) - ? * retry_count)", delay).
		OrderExpr("retry_count desc, updated_at desc")

	if retryCount > 0 {
		subQuery.Where("retry_count < ?", retryCount)
	}

	query := tokens.db.DB().Model(&all).Where("id IN (?)", subQuery)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	err = query.Select()
	return
}

// Retry -
func (tokens *Tokens) Retry(network string, retryCount int, window time.Duration) error {
	_, err := tokens.db.DB().Model((*TokenMetadata)(nil)).
		Set("retry_count = 0").
		Set("status = ?", StatusNew).
		Where("status = ?", StatusFailed).
		Where("network = ?", network).
		Where("retry_count >= ?", retryCount).
		Where("error LIKE '%%context deadline exceeded%%'").
		Where("link LIKE 'ipfs://%%'").
		Where("created_at > (extract(epoch from current_timestamp) - ?)", window).
		Update()
	return err
}

// Update -
func (tokens *Tokens) Update(metadata []*TokenMetadata) error {
	if len(metadata) == 0 {
		return nil
	}

	tokens.mx.Lock()
	defer tokens.mx.Unlock()

	_, err := tokens.db.DB().Model(&metadata).Column("metadata", "update_id", "status", "retry_count", "error").WherePK().Update()
	return err
}

// Save -
func (tokens *Tokens) Save(metadata []*TokenMetadata) error {
	if len(metadata) == 0 {
		return nil
	}

	savings := make([]*TokenMetadata, 0)
	has := make(map[string]struct{})
	for i := len(metadata) - 1; i >= 0; i-- {
		id := fmt.Sprintf("%s_%s", metadata[i].Contract, metadata[i].TokenID.String())
		if _, ok := has[id]; !ok {
			has[id] = struct{}{}
			savings = append(savings, metadata[i])
		}
	}

	if len(savings) == 0 {
		return nil
	}

	tokens.mx.Lock()
	defer tokens.mx.Unlock()

	_, err := tokens.db.DB().Model(&savings).
		OnConflict("(network, contract, token_id) DO UPDATE").
		Set("metadata = excluded.metadata, link = excluded.link, update_id = excluded.update_id, status = excluded.status").
		Insert()
	return err
}

// LastUpdateID -
func (tokens *Tokens) LastUpdateID() (updateID int64, err error) {
	err = tokens.db.DB().Model(&TokenMetadata{}).ColumnExpr("max(update_id)").Select(&updateID)
	return
}

// CountByStatus -
func (tokens *Tokens) CountByStatus(network string, status Status) (int, error) {
	return tokens.db.DB().Model(&TokenMetadata{}).Where("status = ?", status).Where("network = ?", network).Count()
}

// SetImageProcessed -
func (tokens *Tokens) SetImageProcessed(token TokenMetadata) error {
	_, err := tokens.db.DB().Model(&token).Set("image_processed = true").WherePK().Update()
	return err
}

// GetUnprocessedImage -
func (tokens *Tokens) GetUnprocessedImage(from uint64, limit int) (all []TokenMetadata, err error) {
	query := tokens.db.DB().Model(&all).Where("status = 3 AND image_processed = false")
	if from > 0 {
		query.Where("id > ?", from)
	}
	err = query.Limit(limit).Order("id asc").Select()
	return
}
