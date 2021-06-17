package models

import (
	"fmt"

	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/state"
	"gorm.io/gorm"
)

// index type
const (
	IndexTypeMetadata = "metadata"
)

// IndexName -
func IndexName(network string) string {
	return fmt.Sprintf("%s_%s", IndexTypeMetadata, network)
}

// RelativeDatabase -
type RelativeDatabase struct {
	*gorm.DB
}

// NewRelativeDatabase -
func NewRelativeDatabase(cfg config.Database) (*RelativeDatabase, error) {
	db, err := state.OpenConnection(cfg)
	if err != nil {
		return nil, err
	}

	sql, err := db.DB()
	if err != nil {
		return nil, err
	}

	if cfg.Kind == config.DBKindSqlite {
		sql.SetMaxOpenConns(1)
	}

	if err := db.AutoMigrate(&state.State{}, &ContractMetadata{}, &TokenMetadata{}, &ContextItem{}); err != nil {
		if err := sql.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}
	return &RelativeDatabase{db}, nil
}

// GetContractMetadata -
func (db *RelativeDatabase) GetContractMetadata(status Status, limit, offset int) (all []ContractMetadata, err error) {
	query := db.Model(&ContractMetadata{}).Where("status = ?", status)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	err = query.Order("retry_count asc").Find(&all).Error
	return
}

// UpdateContractMetadata -
func (db *RelativeDatabase) UpdateContractMetadata(metadata *ContractMetadata, fields map[string]interface{}) error {
	return db.Model(metadata).Updates(fields).Error
}

// SaveContractMetadata -
func (db *RelativeDatabase) SaveContractMetadata(metadata []*ContractMetadata) error {
	if len(metadata) == 0 {
		return nil
	}
	return db.CreateInBatches(metadata, 100).Error
}

// GetTokenMetadata -
func (db *RelativeDatabase) GetTokenMetadata(status Status, limit, offset int) (all []TokenMetadata, err error) {
	query := db.Model(&TokenMetadata{}).Where("status = ?", status)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	err = query.Order("retry_count asc").Find(&all).Error
	return
}

// UpdateTokenMetadata -
func (db *RelativeDatabase) UpdateTokenMetadata(metadata *TokenMetadata, fields map[string]interface{}) error {
	return db.Model(metadata).Updates(fields).Error
}

// SaveTokenMetadata -
func (db *RelativeDatabase) SaveTokenMetadata(metadata []*TokenMetadata) error {
	if len(metadata) == 0 {
		return nil
	}
	return db.CreateInBatches(metadata, 100).Error
}

// SetImageProcessed -
func (db *RelativeDatabase) SetImageProcessed(token TokenMetadata) error {
	return db.Model(&token).Update("image_processed", true).Error
}

// GetUnprocessedImage -
func (db *RelativeDatabase) GetUnprocessedImage(from uint64, limit int) (all []TokenMetadata, err error) {
	query := db.Model(&TokenMetadata{}).Where("status = 3 AND image_processed = false")
	if from > 0 {
		query.Where("id > ?", from)
	}
	err = query.Limit(limit).Order("id asc").Find(&all).Error
	return
}

// CurrentContext -
func (db *RelativeDatabase) CurrentContext() (updates []ContextItem, err error) {
	err = db.Model(&ContextItem{}).Find(&updates).Error
	return
}

// DumpContext -
func (db *RelativeDatabase) DumpContext(action Action, item ContextItem) error {
	switch action {
	case ActionCreate, ActionUpdate:
		if err := db.Save(&item).Error; err != nil {
			return err
		}
	case ActionDelete:
		if err := db.Delete(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetState -
func (db *RelativeDatabase) GetState(indexName string) (s state.State, err error) {
	err = db.Where("index_name = ?", indexName).First(&s).Error
	return
}

// UpdateState -
func (db *RelativeDatabase) UpdateState(s state.State) error {
	return s.Update(db.DB)
}

// Close -
func (db *RelativeDatabase) Close() error {
	sql, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sql.Close()
}
