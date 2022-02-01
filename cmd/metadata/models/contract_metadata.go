package models

import (
	"context"
	"time"

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
	cm.UpdateID = ContractUpdateID.Increment()
	return ctx, nil
}

// BeforeUpdate -
func (cm *ContractMetadata) BeforeUpdate(ctx context.Context) (context.Context, error) {
	cm.UpdatedAt = time.Now().Unix()
	cm.UpdateID = ContractUpdateID.Increment()
	return ctx, nil
}

// ContractRepository -
type ContractRepository interface {
	GetContractMetadata(network string, status Status, limit, offset, retryCount int) ([]ContractMetadata, error)
	UpdateContractMetadata(ctx context.Context, metadata []*ContractMetadata) error
	SaveContractMetadata(ctx context.Context, metadata []*ContractMetadata) error
	LastContractUpdateID() (int64, error)
	CountContractsByStatus(network string, status Status) (int, error)
}
