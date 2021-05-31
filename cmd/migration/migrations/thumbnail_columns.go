package migrations

import (
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
)

// ThumbnailColumns -
type ThumbnailColumns struct {
}

// Name -
func (m *ThumbnailColumns) Name() string {
	return "thumbnail_columns"
}

// Do -
func (m *ThumbnailColumns) Do(cfg config.Config) error {
	db, err := models.OpenDatabaseConnection(cfg.Database)
	if err != nil {
		return err
	}

	if db.Migrator().HasColumn(&models.TokenMetadata{}, "image_retry_count") {
		if err := db.Migrator().DropColumn(&models.TokenMetadata{}, "image_retry_count"); err != nil {
			return err
		}
	}

	return db.AutoMigrate(&models.TokenMetadata{})
}
