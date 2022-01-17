package models

import (
	"fmt"

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

// ContextItem -
type ContextItem struct {
	//nolint
	tableName struct{} `pg:"dipdup_metadata_context"`

	ID      uint64 `json:"-"  pg:",notnull"`
	Network string `pg:",unique:context_item"`
	Address string `pg:",unique:context_item"`
	Key     string `pg:",unique:context_item"`
	Value   []byte
}

// TableName -
func (ContextItem) TableName() string {
	return "dipdup_metadata_context"
}

// Path -
func (ci ContextItem) Path() string {
	return fmt.Sprintf("%s:%s:%s", ci.Network, ci.Address, ci.Key)
}

// ContextRepository -
type ContextRepository interface {
	CurrentContext() ([]ContextItem, error)
	DumpContext(action Action, item ContextItem) error
}

// ContextFromUpdate -
func ContextFromUpdate(update api.BigMapUpdate, network string) (ContextItem, error) {
	var ctx ContextItem
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
