package models

import (
	"time"

	"gorm.io/datatypes"
)

// TokenMetadata -
type TokenMetadata struct {
	ID             uint64         `gorm:"autoIncrement;not null;" json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Network        string         `gorm:"primaryKey" json:"network"`
	Contract       string         `gorm:"primaryKey" json:"contract"`
	TokenID        uint64         `gorm:"primaryKey" json:"token_id"`
	Link           string         `json:"link"`
	RetryCount     int            `gorm:"type:SMALLINT;default:0" json:"retry_count"`
	Status         Status         `gorm:"type:SMALLINT" json:"status"`
	Metadata       datatypes.JSON `json:"metadata,omitempty"`
	ImageProcessed bool           `json:"image_processed"`
}

// Table -
func (TokenMetadata) TableName() string {
	return "token_metadata"
}

// TokenRepository -
type TokenRepository interface {
	GetTokenMetadata(status Status, limit, offset int) ([]TokenMetadata, error)
	SetImageProcessed(token TokenMetadata) error
	GetUnprocessedImage(from uint64, limit int) ([]TokenMetadata, error)
	UpdateTokenMetadata(metadata *TokenMetadata, fields map[string]interface{}) error
	SaveTokenMetadata(metadata []*TokenMetadata) error
}
