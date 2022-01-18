package tezoskeys

import (
	"github.com/dipdup-net/go-lib/tzkt/api"
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
	repo models.TezosKeyRepository
}

// NewTezosKeys -
func NewTezosKeys(repo models.TezosKeyRepository) *TezosKeys {
	return &TezosKeys{repo}
}

// Add -
func (tk *TezosKeys) Add(update api.BigMapUpdate, network string) error {
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
		return tk.repo.SaveTezosKey(item)
	case "remove_key":
		return tk.repo.DeleteTezosKey(item)
	}
	return nil
}

// Get -
func (tk *TezosKeys) Get(network, address, key string) (models.TezosKey, error) {
	return tk.repo.GetTezosKey(network, address, key)
}
