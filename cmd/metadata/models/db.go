package models

import (
	"context"
	"fmt"
	"time"

	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/database"
	pg "github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/rs/zerolog/log"
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
	*database.PgGo
}

// NewRelativeDatabase -
func NewRelativeDatabase(ctx context.Context, cfg config.Database) (*RelativeDatabase, error) {
	db := database.NewPgGo()
	if err := db.Connect(ctx, cfg); err != nil {
		return nil, err
	}

	database.Wait(ctx, db, 5*time.Second)

	for _, data := range []interface{}{
		&database.State{}, &ContractMetadata{}, &TokenMetadata{}, &ContextItem{},
	} {
		if err := db.DB().WithContext(ctx).Model(data).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
		}); err != nil {
			if err := db.Close(); err != nil {
				return nil, err
			}
			return nil, err
		}
	}
	db.DB().AddQueryHook(dbLogger{})

	return &RelativeDatabase{db}, nil
}

type dbLogger struct{}

func (d dbLogger) BeforeQuery(c context.Context, q *pg.QueryEvent) (context.Context, error) {
	q.StartTime = time.Now()
	return c, nil
}

func (d dbLogger) AfterQuery(c context.Context, q *pg.QueryEvent) error {
	duration := time.Since(q.StartTime).Milliseconds()
	raw, err := q.FormattedQuery()
	if err != nil {
		return err
	}
	sql := string(raw)
	log.Debug().Msgf("[%d ms] %+v", duration, sql)

	return nil
}

// GetContractMetadata -
func (db *RelativeDatabase) GetContractMetadata(status Status, limit, offset, retryCount int) (all []ContractMetadata, err error) {
	query := db.DB().Model(&all).Where("status = ?", status)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	if retryCount > 0 {
		query.Where("retry_count < ?", retryCount)
	}
	err = query.Order("retry_count asc").Select()
	return
}

// UpdateContractMetadata -
func (db *RelativeDatabase) UpdateContractMetadata(ctx context.Context, metadata []*ContractMetadata) error {
	if len(metadata) == 0 {
		return nil
	}

	_, err := db.DB().Model(&metadata).Column("metadata", "update_id", "status", "retry_count").WherePK().Update()
	return err
}

// SaveContractMetadata -
func (db *RelativeDatabase) SaveContractMetadata(ctx context.Context, metadata []*ContractMetadata) error {
	if len(metadata) == 0 {
		return nil
	}
	_, err := db.DB().Model(&metadata).
		OnConflict("(network, contract) DO UPDATE").
		Set("metadata = excluded.metadata, link = excluded.link, update_id = excluded.update_id, status = excluded.status").
		Insert()
	return err
}

// LastTokenUpdateID -
func (db *RelativeDatabase) LastContractUpdateID() (updateID int64, err error) {
	err = db.DB().Model(&ContractMetadata{}).ColumnExpr("max(update_id)").Select(&updateID)
	return
}

// GetTokenMetadata -
func (db *RelativeDatabase) GetTokenMetadata(status Status, limit, offset, retryCount int) (all []TokenMetadata, err error) {
	query := db.DB().Model(&all).Where("status = ?", status)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	if retryCount > 0 {
		query.Where("retry_count < ?", retryCount)
	}
	err = query.Order("retry_count asc").Select()
	return
}

// UpdateTokenMetadata -
func (db *RelativeDatabase) UpdateTokenMetadata(ctx context.Context, metadata []*TokenMetadata) error {
	if len(metadata) == 0 {
		return nil
	}

	_, err := db.DB().Model(&metadata).Column("metadata", "update_id", "status", "retry_count", "link").WherePK().Update()
	return err
}

// SaveTokenMetadata -
func (db *RelativeDatabase) SaveTokenMetadata(ctx context.Context, metadata []*TokenMetadata) error {
	if len(metadata) == 0 {
		return nil
	}

	_, err := db.DB().Model(&metadata).
		OnConflict("(network, contract, token_id) DO UPDATE").
		Set("metadata = excluded.metadata, link = excluded.link, update_id = excluded.update_id, status = excluded.status").
		Insert()
	return err
}

// SetImageProcessed -
func (db *RelativeDatabase) SetImageProcessed(token TokenMetadata) error {
	_, err := db.DB().Model(&token).Set("image_processed", true).WherePK().Update()
	return err
}

// GetUnprocessedImage -
func (db *RelativeDatabase) GetUnprocessedImage(from uint64, limit int) (all []TokenMetadata, err error) {
	query := db.DB().Model(&all).Where("status = 3 AND image_processed = false")
	if from > 0 {
		query.Where("id > ?", from)
	}
	err = query.Limit(limit).Order("id asc").Select()
	return
}

// CurrentContext -
func (db *RelativeDatabase) CurrentContext() (updates []ContextItem, err error) {
	err = db.DB().Model(&updates).Select()
	return
}

// LastTokenUpdateID -
func (db *RelativeDatabase) LastTokenUpdateID() (updateID int64, err error) {
	err = db.DB().Model(&TokenMetadata{}).ColumnExpr("max(update_id)").Select(&updateID)
	return
}

// DumpContext -
func (db *RelativeDatabase) DumpContext(action Action, item ContextItem) error {
	switch action {
	case ActionUpdate:
		_, err := db.DB().Model(&item).WherePK().Update()
		return err

	case ActionCreate:
		_, err := db.DB().Model(&item).Insert()
		return err
	case ActionDelete:
		_, err := db.DB().Model(&item).Delete()
		return err
	}
	return nil
}

// Close -
func (db *RelativeDatabase) Close() error {
	return db.PgGo.Close()
}
