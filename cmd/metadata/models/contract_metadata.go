package models

import (
	"context"
	"time"
)

// ContractMetadata -
type ContractMetadata struct {
	//nolint
	tableName struct{} `pg:"contract_metadata"`

	ID         uint64 `gorm:"autoIncrement;not null;" json:"-" pg:",notnull"`
	CreatedAt  int64  `gorm:"autoCreateTime" json:"created_at" pg:",use_zero"`
	UpdatedAt  int64  `gorm:"autoUpdateTime" json:"updated_at" pg:",use_zero"`
	UpdateID   int64  `gorm:"type:int4;uniqueIndex:contract_metadata_update_id_key;autoIncrement:false;not null;" json:"-" pg:",use_zero,notnull"`
	Network    string `gorm:"primaryKey" json:"network" pg:",unique:contract"`
	Contract   string `gorm:"primaryKey" json:"contract" pg:",unique:contract"`
	Link       string `json:"link"`
	Status     Status `gorm:"type:SMALLINT" json:"status"`
	RetryCount int8   `gorm:"type:SMALLINT" json:"retry_count" pg:",use_zero"`
	Metadata   JSONB  `json:"metadata,omitempty" pg:",type:jsonb,use_zero"`
}

// TableName -
func (ContractMetadata) TableName() string {
	return "contract_metadata"
}

// BeforeInsert -
func (cm *ContractMetadata) BeforeInsert(ctx context.Context) (context.Context, error) {
	cm.UpdatedAt = time.Now().Unix()
	cm.CreatedAt = cm.UpdatedAt
	return ctx, nil
}

// BeforeUpdate -
func (cm *ContractMetadata) BeforeUpdate(ctx context.Context) (context.Context, error) {
	cm.UpdatedAt = time.Now().Unix()
	return ctx, nil
}

// ContractRepository -
type ContractRepository interface {
	GetContractMetadata(network string, status Status, limit, offset, retryCount int) ([]ContractMetadata, error)
	UpdateContractMetadata(ctx context.Context, metadata []*ContractMetadata) error
	SaveContractMetadata(ctx context.Context, metadata []*ContractMetadata) error
	LastContractUpdateID() (int64, error)
}
