package models

import (
	"github.com/dipdup-net/go-lib/database"
	"github.com/go-pg/pg/v10"
)

// IPFSLink -
type IPFSLink struct {
	//nolint
	tableName struct{} `pg:"ipfs_link"`

	ID   int64
	Node string
	Link string `pg:",unique:ipfs"`
	Data []byte `pg:",type:bytea,use_zero"`
}

// TableName -
func (IPFSLink) TableName() string {
	return "ipfs_link"
}

// IPFSLinkRepository -
type IPFSLinkRepository interface {
	Get(id int64) (IPFSLink, error)
	GetByURLs(url ...string) ([]IPFSLink, error)
	All(limit, offset int) ([]IPFSLink, error)
	Save(link IPFSLink) error
	Update(link IPFSLink) error
}

// IPFS -
type IPFS struct {
	db *database.PgGo
}

// NewIPFS -
func NewIPFS(db *database.PgGo) *IPFS {
	return &IPFS{db}
}

// Get -
func (ipfs *IPFS) Get(id int64) (link IPFSLink, err error) {
	err = ipfs.db.DB().Model(&link).Where("id = ?", id).First()
	return
}

// All -
func (ipfs *IPFS) All(limit, offset int) (links []IPFSLink, err error) {
	err = ipfs.db.DB().Model(&links).Limit(limit).Offset(offset).Order("id desc").Select(&links)
	return
}

// Save -
func (ipfs *IPFS) Save(link IPFSLink) error {
	_, err := ipfs.db.DB().Model(&link).Where("link = ?link").SelectOrInsert(&link)
	return err
}

// Update -
func (ipfs *IPFS) Update(link IPFSLink) error {
	_, err := ipfs.db.DB().Model(&link).WherePK().Column("data", "node").Update()
	return err
}

// GetByURLs -
func (ipfs *IPFS) GetByURLs(url ...string) (links []IPFSLink, err error) {
	err = ipfs.db.DB().Model(&links).Where("link IN (?)", pg.In(url)).Select(&links)
	return
}
