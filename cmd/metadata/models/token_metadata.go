package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TokenMetadata -
type TokenMetadata struct {
	Network    string `gorm:"primaryKey"`
	Contract   string `gorm:"primaryKey"`
	TokenID    uint64 `gorm:"primaryKey"`
	Link       string
	RetryCount int
	Status     Status
	UpdatedAt  int
	Metadata   datatypes.JSON
}

// GetTokenMetadata -
func GetTokenMetadata(tx *gorm.DB, status Status, limit, offset int) (all []TokenMetadata, err error) {
	query := tx.Model(&TokenMetadata{}).Where("status = ?", status)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	err = query.Order("retry_count asc").Find(&all).Error
	return
}
