package models

import (
	"github.com/dipdup-net/go-lib/database"
	"github.com/dipdup-net/go-lib/tzkt/data"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
)

// Action -
type Action string

// Actions
const (
	ActionDelete Action = "delete"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
)

// TezosKey -
type TezosKey struct {
	//nolint
	tableName struct{} `pg:"tezos_keys"`

	ID      uint64 `json:"-"  pg:",notnull"`
	Network string `pg:",unique:tezos_key"`
	Address string `pg:",unique:tezos_key"`
	Key     string `pg:",unique:tezos_key"`
	Value   []byte
}

// TableName -
func (TezosKey) TableName() string {
	return "tezos_keys"
}

// ContextFromUpdate -
func ContextFromUpdate(update data.BigMapUpdate, network string) (TezosKey, error) {
	var ctx TezosKey
	ctx.Address = update.Contract.Address
	ctx.Network = network
	ctx.Key = helpers.Trim(string(update.Content.Key))

	data, err := helpers.Decode(update.Content.Value)
	if err != nil {
		return ctx, err
	}
	ctx.Value = data
	return ctx, nil
}

// TezosKeys -
type TezosKeys struct {
	db *database.PgGo
}

// NewTezosKeys -
func NewTezosKeys(db *database.PgGo) *TezosKeys {
	return &TezosKeys{db}
}

// Get -
func (keys *TezosKeys) Get(network, address, key string) (tk TezosKey, err error) {
	query := keys.db.DB().Model(&tk)

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

// Save -
func (keys *TezosKeys) Save(tk TezosKey) error {
	_, err := keys.db.DB().Model(&tk).OnConflict("(network, address, key) DO UPDATE").Set("value = excluded.value").Insert()
	return err
}

// Delete -
func (keys *TezosKeys) Delete(tk TezosKey) error {
	query := keys.db.DB().Model(&tk)

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
