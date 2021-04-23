package models

import (
	"fmt"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"gorm.io/gorm"
)

// ContextItem -
type ContextItem struct {
	Network string `gorm:"primarykey"`
	Address string `gorm:"primarykey"`
	Key     string `gorm:"primarykey"`
	Value   []byte
}

// TableName -
func (ContextItem) TableName() string {
	return "_dipdup_metadata_context"
}

// Path -
func (ci ContextItem) Path() string {
	return fmt.Sprintf("%s:%s:%s", ci.Network, ci.Address, ci.Key)
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

// CurrentContext -
func CurrentContext(db *gorm.DB) (updates []ContextItem, err error) {
	err = db.Model(&ContextItem{}).Find(&updates).Error
	return
}
