package models

import (
	"context"
	"sync"
	"time"

	"github.com/dipdup-net/go-lib/database"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
)

// ContractUpdateID - incremental counter
var ContractUpdateID = helpers.NewCounter(0)

// ContractMetadata -
type ContractMetadata struct {
	//nolint
	tableName struct{} `pg:"contract_metadata"`

	ID         uint64 `json:"-" pg:",notnull"`
	CreatedAt  int64  `json:"created_at" pg:",use_zero"`
	UpdatedAt  int64  `json:"updated_at" pg:",use_zero"`
	UpdateID   int64  `json:"-" pg:",use_zero,notnull"`
	Network    string `json:"network" pg:",unique:contract"`
	Contract   string `json:"contract" pg:",unique:contract"`
	Link       string `json:"link"`
	Status     Status `json:"status"`
	RetryCount int8   `json:"retry_count" pg:",use_zero"`
	Metadata   JSONB  `json:"metadata,omitempty" pg:",type:json,use_zero"`
	Error      string `json:"error,omitempty"`
}

// TableName -
func (ContractMetadata) TableName() string {
	return "contract_metadata"
}

// GetStatus -
func (cm *ContractMetadata) GetStatus() Status {
	return cm.Status
}

// GetRetryCount -
func (cm *ContractMetadata) GetRetryCount() int8 {
	return cm.RetryCount
}

// GetID -
func (cm *ContractMetadata) GetID() uint64 {
	return cm.ID
}

// GetLink -
func (cm *ContractMetadata) GetLink() string {
	return cm.Link
}

// IncrementRetryCount -
func (cm *ContractMetadata) IncrementRetryCount() {
	cm.RetryCount += 1
}

// SetMetadata -
func (cm *ContractMetadata) SetMetadata(data JSONB) {
	cm.Metadata = data
}

// SetStatus -
func (cm *ContractMetadata) SetStatus(status Status) {
	cm.Status = status
}

// BeforeInsert -
func (cm *ContractMetadata) BeforeInsert(ctx context.Context) (context.Context, error) {
	cm.UpdatedAt = time.Now().Unix()
	cm.CreatedAt = cm.UpdatedAt
	cm.UpdateID = ContractUpdateID.Increment()
	return ctx, nil
}

// BeforeUpdate -
func (cm *ContractMetadata) BeforeUpdate(ctx context.Context) (context.Context, error) {
	cm.UpdatedAt = time.Now().Unix()
	cm.UpdateID = ContractUpdateID.Increment()
	return ctx, nil
}

// Contracts -
type Contracts struct {
	db *database.PgGo

	mx sync.Mutex
}

// NewContracts -
func NewContracts(db *database.PgGo) *Contracts {
	return &Contracts{db: db}
}

// Get -
func (contracts *Contracts) Get(network string, status Status, limit, offset, retryCount, delay int) (all []*ContractMetadata, err error) {
	query := contracts.db.DB().Model(&all).Where("status = ?", status).Where("network = ?", network).Where("created_at < (extract(epoch from current_timestamp) - ? * retry_count)", delay)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	if retryCount > 0 {
		query.Where("retry_count < ?", retryCount)
	}
	err = query.OrderExpr("retry_count desc, updated_at desc").Select()
	return
}

// FailedByTimeout -
func (contracts *Contracts) FailedByTimeout(network string, limit, offset, retryCount, delay int) (all []*ContractMetadata, err error) {
	subQuery := contracts.db.DB().Model((*ContractMetadata)(nil)).Column("id").
		Where("status = ?", StatusFailed).
		Where("network = ?", network).
		Where("retry_count = ?", retryCount).
		Where("error LIKE '%%Client.Timeout%%'").
		Where("created_at < (extract(epoch from current_timestamp) - ?)", delay).
		OrderExpr("updated_at desc")

	query := contracts.db.DB().Model(&all).Where("id IN (?)", subQuery)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	err = query.Select()
	return
}

// Update -
func (contracts *Contracts) Update(metadata []*ContractMetadata) error {
	if len(metadata) == 0 {
		return nil
	}

	contracts.mx.Lock()
	defer contracts.mx.Unlock()

	_, err := contracts.db.DB().Model(&metadata).Column("metadata", "update_id", "status", "retry_count", "error").WherePK().Update()
	return err
}

// Save -
func (contracts *Contracts) Save(metadata []*ContractMetadata) error {
	if len(metadata) == 0 {
		return nil
	}

	savings := make([]*ContractMetadata, 0)
	has := make(map[string]struct{})
	for i := len(metadata) - 1; i >= 0; i-- {
		if _, ok := has[metadata[i].Contract]; !ok {
			has[metadata[i].Contract] = struct{}{}
			savings = append(savings, metadata[i])
		}
	}
	if len(savings) == 0 {
		return nil
	}

	contracts.mx.Lock()
	defer contracts.mx.Unlock()

	_, err := contracts.db.DB().Model(&savings).
		OnConflict("(network, contract) DO UPDATE").
		Set("metadata = excluded.metadata, link = excluded.link, update_id = excluded.update_id, status = excluded.status").
		Insert()
	return err
}

// LastUpdateID -
func (contracts *Contracts) LastUpdateID() (updateID int64, err error) {
	err = contracts.db.DB().Model(&ContractMetadata{}).ColumnExpr("max(update_id)").Select(&updateID)
	return
}

// CountByStatus -
func (contracts *Contracts) CountByStatus(network string, status Status) (int, error) {
	return contracts.db.DB().Model(&ContractMetadata{}).Where("status = ?", status).Where("network = ?", network).Count()
}
