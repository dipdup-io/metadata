package models

import (
	"gorm.io/datatypes"
)

// TokenMetadata -
type TokenMetadata struct {
	ID             uint64         `gorm:"autoIncrement;not null;" json:"-"`
	CreatedAt      int64          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      int64          `gorm:"autoUpdateTime" json:"updated_at"`
	Network        string         `gorm:"primaryKey" json:"network"`
	Contract       string         `gorm:"primaryKey" json:"contract"`
	TokenID        uint64         `gorm:"primaryKey" json:"token_id"`
	Link           string         `json:"link"`
	RetryCount     int            `gorm:"type:SMALLINT;default:0" json:"retry_count"`
	Status         Status         `gorm:"type:SMALLINT" json:"status"`
	Metadata       datatypes.JSON `json:"metadata,omitempty"`
	ImageProcessed bool           `json:"image_processed"`
	UpdateID       int64          `gorm:"type:int4;uniqueIndex:token_metadata_update_id_key;autoIncrement:false;not null;" json:"-"`
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
	LastTokenUpdateID() (int64, error)
}
