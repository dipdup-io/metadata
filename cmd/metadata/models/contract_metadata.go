package models

import (
	"gorm.io/datatypes"
)

// ContractMetadata -
type ContractMetadata struct {
	ID         uint64         `gorm:"autoIncrement;not null;" json:"-"`
	CreatedAt  int64          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  int64          `gorm:"autoUpdateTime" json:"updated_at"`
	Network    string         `gorm:"primaryKey" json:"network"`
	Contract   string         `gorm:"primaryKey" json:"contract"`
	RetryCount int            `gorm:"type:SMALLINT" json:"retry_count"`
	Link       string         `json:"link"`
	Status     Status         `gorm:"type:SMALLINT" json:"status"`
	Metadata   datatypes.JSON `json:"metadata,omitempty"`
	UpdateID   int64          `gorm:"type:int4;uniqueIndex:contract_metadata_update_id_key;autoIncrement:false;not null;" json:"-"`
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
	LastContractUpdateID() (int64, error)
}
