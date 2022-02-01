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
		&database.State{}, &ContractMetadata{}, &TokenMetadata{}, &TezosKey{}, &IPFSLink{},
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
func (db *RelativeDatabase) GetContractMetadata(network string, status Status, limit, offset, retryCount int) (all []ContractMetadata, err error) {
	query := db.DB().Model(&all).Where("status = ?", status).Where("network = ?", network)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	if retryCount > 0 {
		query.Where("retry_count < ?", retryCount)
	}
	err = query.OrderExpr("retry_count desc, updated_at desc").Select()
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

// CountContractsByStatus -
func (db *RelativeDatabase) CountContractsByStatus(network string, status Status) (int, error) {
	return db.DB().Model(&ContractMetadata{}).Where("status = ?", status).Where("network = ?", network).Count()
}

// GetTokenMetadata -
func (db *RelativeDatabase) GetTokenMetadata(network string, status Status, limit, offset, retryCount int) (all []TokenMetadata, err error) {
	query := db.DB().Model(&all).Where("status = ?", status).Where("network = ?", network)
	if limit > 0 {
		query.Limit(limit)
	}
	if offset > 0 {
		query.Offset(offset)
	}
	if retryCount > 0 {
		query.Where("retry_count < ?", retryCount)
	}
	err = query.OrderExpr("retry_count desc, updated_at desc").Select()
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
	_, err := db.DB().Model(&token).Set("image_processed = true").WherePK().Update()
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

// CountTokensByStatus -
func (db *RelativeDatabase) CountTokensByStatus(network string, status Status) (int, error) {
	return db.DB().Model(&TokenMetadata{}).Where("status = ?", status).Where("network = ?", network).Count()
}

// GetTezosKey -
func (db *RelativeDatabase) GetTezosKey(network, address, key string) (tk TezosKey, err error) {
	query := db.DB().Model(&tk)

	if network != "" {
		query.Where("network = ?", network)
	}
	if address != "" {
		query.Where("address = ?", address)
	}
	if key != "" {
		query.Where("key = ?", key)
	}

	err = query.First()
	return
}

// SaveTezosKey -
func (db *RelativeDatabase) SaveTezosKey(tk TezosKey) error {
	_, err := db.DB().Model(&tk).OnConflict("(network, address, key) DO UPDATE").Set("value = excluded.value").Insert()
	return err
}

// DeleteTezosKey -
func (db *RelativeDatabase) DeleteTezosKey(tk TezosKey) error {
	query := db.DB().Model(&tk)

	if tk.Network != "" {
		query.Where("network = ?", tk.Network)
	}
	if tk.Address != "" {
		query.Where("address = ?", tk.Address)
	}
	if tk.Key != "" {
		query.Where("key = ?", tk.Key)
	}

	_, err := query.Delete()
	return err
}

// LastTokenUpdateID -
func (db *RelativeDatabase) LastTokenUpdateID() (updateID int64, err error) {
	err = db.DB().Model(&TokenMetadata{}).ColumnExpr("max(update_id)").Select(&updateID)
	return
}

// Close -
func (db *RelativeDatabase) Close() error {
	return db.PgGo.Close()
}

// IPFSLink -
func (db *RelativeDatabase) IPFSLink(id int64) (link IPFSLink, err error) {
	err = db.DB().Model(&link).Where("id = ?", id).First()
	return
}

// IPFSLinks -
func (db *RelativeDatabase) IPFSLinks(limit, offset int) (links []IPFSLink, err error) {
	err = db.DB().Model(&links).Limit(limit).Offset(offset).Order("id desc").Select(&links)
	return
}

// SaveIPFSLink -
func (db *RelativeDatabase) SaveIPFSLink(link IPFSLink) error {
	_, err := db.DB().Model(&link).Where("link = ?link").SelectOrInsert(&link)
	return err
}

// UpdateIPFSLink -
func (db *RelativeDatabase) UpdateIPFSLink(link IPFSLink) error {
	_, err := db.DB().Model(&link).WherePK().Column("data", "node").Update()
	return err
}

// IPFSLinkByURL -
func (db *RelativeDatabase) IPFSLinkByURL(url string) (link IPFSLink, err error) {
	err = db.DB().Model(&link).Where("link = ?", url).First()
	return
}

// CreateIndices -
func (db *RelativeDatabase) CreateIndices() error {
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_network_status_idx ON contract_metadata (network, status)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_idx ON contract_metadata (network, contract)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_sort_idx ON contract_metadata (retry_count, updated_at)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS contract_metadata_update_id_idx ON contract_metadata (update_id)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_network_status_idx ON token_metadata (network, status)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_sort_idx ON token_metadata (retry_count, updated_at)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_idx ON token_metadata (network, contract, token_id)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS token_metadata_update_id_idx ON token_metadata (update_id)
	`); err != nil {
		return err
	}
	if _, err := db.DB().Exec(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS tezos_key_idx ON tezos_keys (network, address, key)
	`); err != nil {
		return err
	}
	return nil
}

// Exec -
func (db *RelativeDatabase) Exec(sql string) error {
	_, err := db.DB().Exec(sql)
	return err
}
