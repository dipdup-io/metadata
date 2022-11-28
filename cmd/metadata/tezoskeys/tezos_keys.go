package tezoskeys

import (
	"github.com/dipdup-net/go-lib/tzkt/data"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
)

// TezosKeysAction -
type TezosKeysAction struct {
	Action models.Action
	Update models.TezosKey
}

// TezosKeys -
type TezosKeys struct {
	repo *models.TezosKeys
}

// NewTezosKeys -
func NewTezosKeys(repo *models.TezosKeys) *TezosKeys {
	return &TezosKeys{repo}
}

// Add -
func (tk *TezosKeys) Add(update data.BigMapUpdate, network string) error {
	val := string(update.Content.Value)
	if !helpers.IsJSON(val) { // wait only JSON
		return nil
	}
	item, err := models.ContextFromUpdate(update, network)
	if err != nil {
		return err
	}

	switch update.Action {
	case "add_key", "update_key":
		return tk.repo.Save(item)
	case "remove_key":
		return tk.repo.Delete(item)
	}
	return nil
}

// Get -
func (tk *TezosKeys) Get(network, address, key string) (models.TezosKey, error) {
	return tk.repo.Get(network, address, key)
}
