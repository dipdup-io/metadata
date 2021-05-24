package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TokenMetadata -
type TokenMetadata struct {
	Network         string `gorm:"primaryKey"`
	Contract        string `gorm:"primaryKey"`
	TokenID         uint64 `gorm:"primaryKey"`
	Link            string
	RetryCount      int `gorm:"default:0"`
	Status          Status
	UpdatedAt       int
	Metadata        datatypes.JSON
	ImageProcessed  bool
	ImageRetryCount int `gorm:"default:0"`
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

// GetTokenMetadataWithUnprocessedImages -
func GetTokenMetadataWithUnprocessedImages(tx *gorm.DB) (all []TokenMetadata, err error) {
	err = tx.Model(&TokenMetadata{}).Where("status = 3 AND image_processed = false AND image_retry_count < 3").Limit(5).Order("image_retry_count desc").Find(&all).Error
	return
}

// SetImageProcessed -
func (tm *TokenMetadata) SetImageProcessed(tx *gorm.DB) error {
	tm.ImageRetryCount += 1
	updates := map[string]interface{}{
		"image_retry_count": tm.ImageRetryCount,
	}

	if tm.ImageProcessed {
		updates["image_processed"] = true
	}
	return tx.Model(&tm).Updates(updates).Error
}
