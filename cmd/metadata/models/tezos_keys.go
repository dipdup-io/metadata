package models

import (
	"github.com/dipdup-net/go-lib/tzkt/api"
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

// TezosKeyRepository -
type TezosKeyRepository interface {
	GetTezosKey(network string, address string, key string) (TezosKey, error)
	SaveTezosKey(key TezosKey) error
	DeleteTezosKey(key TezosKey) error
}

// ContextFromUpdate -
func ContextFromUpdate(update api.BigMapUpdate, network string) (TezosKey, error) {
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
