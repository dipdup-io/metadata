package context

import (
	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"gorm.io/gorm"
)

// Action -
type Action string

// Actions
const (
	ActionDelete Action = "delete"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
)

// ContextAction -
type ContextAction struct {
	Action Action
	Update models.ContextItem
}

// Context -
type Context struct {
	cache map[string]models.ContextItem

	journal map[string]ContextAction
}

// NewContext -
func NewContext() *Context {
	return &Context{
		cache:   make(map[string]models.ContextItem),
		journal: make(map[string]ContextAction),
	}
}

// Add -
func (ctx *Context) Add(update api.BigMapUpdate, network string) error {
	val := string(update.Content.Value)
	if !helpers.IsJSON(val) { // wait only JSON
		return nil
	}
	item, err := models.ContextFromUpdate(update, network)
	if err != nil {
		return err
	}
	key := item.Path()
	ctx.cache[key] = item

	switch update.Action {
	case "add_key":
		ctx.journal[key] = ContextAction{
			Action: ActionCreate,
			Update: item,
		}
	case "update_key":
		ctx.journal[key] = ContextAction{
			Action: ActionUpdate,
			Update: item,
		}
	}
	return nil
}

// Remove -
func (ctx *Context) Remove(update models.ContextItem) {
	key := update.Path()
	if _, ok := ctx.journal[key]; ok {
		delete(ctx.journal, key)
	} else {
		ctx.journal[key] = ContextAction{
			Action: ActionDelete,
			Update: update,
		}
	}
}

// Get -
func (ctx *Context) Get(network, address, key string) (models.ContextItem, bool) {
	item := models.ContextItem{
		Network: network,
		Address: address,
		Key:     key,
	}
	current, ok := ctx.cache[item.Path()]
	return current, ok
}

// Dump -
func (ctx *Context) Dump(tx *gorm.DB) error {
	for key, action := range ctx.journal {
		switch action.Action {
		case ActionCreate, ActionUpdate:
			if err := tx.Save(&action.Update).Error; err != nil {
				return err
			}
		case ActionDelete:
			if err := tx.Delete(&action.Update).Error; err != nil {
				return err
			}
		}
		delete(ctx.journal, key)
	}
	return nil
}

// Load -
func (ctx *Context) Load(db *gorm.DB) error {
	items, err := models.CurrentContext(db)
	if err != nil {
		return err
	}

	for i := range items {
		ctx.cache[items[i].Path()] = items[i]
	}
	return nil
}
