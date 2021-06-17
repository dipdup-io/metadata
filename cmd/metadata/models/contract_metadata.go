package models

import (
	"time"

	"gorm.io/datatypes"
)

// ContractMetadata -
type ContractMetadata struct {
	ID         uint64         `gorm:"autoIncrement;not null;" json:"-"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Network    string         `gorm:"primaryKey" json:"network"`
	Contract   string         `gorm:"primaryKey" json:"contract"`
	RetryCount int            `gorm:"type:SMALLINT" json:"retry_count"`
	Link       string         `json:"link"`
	Status     Status         `gorm:"type:SMALLINT" json:"status"`
	Metadata   datatypes.JSON `json:"metadata,omitempty"`
}

// Table -
func (ContractMetadata) TableName() string {
	return "contract_metadata"
}

// ContractRepository -
type ContractRepository interface {
	GetContractMetadata(status Status, limit, offset int) ([]ContractMetadata, error)
	UpdateContractMetadata(metadata *ContractMetadata, fields map[string]interface{}) error
	SaveContractMetadata(metadata []*ContractMetadata) error
}
