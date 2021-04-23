package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ContractMetadata -
type ContractMetadata struct {
	Network    string `gorm:"primaryKey"`
	Contract   string `gorm:"primaryKey"`
	RetryCount int
	Link       string
	Status     Status
	UpdatedAt  int
	Metadata   datatypes.JSON
}

// Status - metadata status
type Status string

const (
	StatusNew     Status = "new"
	StatusFailed  Status = "failed"
	StatusApplied Status = "applied"
)

// GetContractMetadata -
func GetContractMetadata(tx *gorm.DB, status Status, limit, offset int) (all []ContractMetadata, err error) {
	query := tx.Model(&ContractMetadata{}).Where("status = ?", status)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	err = query.Order("retry_count asc").Find(&all).Error
	return
}
