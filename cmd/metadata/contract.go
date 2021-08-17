package main

import (
	"fmt"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
	"github.com/pkg/errors"
)

func (indexer *Indexer) processContractMetadata(update api.BigMapUpdate) (*models.ContractMetadata, error) {
	if update.Content == nil {
		return nil, nil
	}
	if update.Content.Hash != emptyHash {
		return nil, indexer.ctx.Add(update, indexer.network)
	}

	link, err := helpers.Decode(update.Content.Value)
	if err != nil {
		return nil, err
	}

	metadata := models.ContractMetadata{
		Network:  indexer.network,
		Contract: update.Contract.Address,
		Status:   models.StatusNew,
		Link:     string(link),
		UpdateID: indexer.contractActionsCounter.Increment(),
	}

	return &metadata, nil
}

func (indexer *Indexer) logContractMetadata(cm models.ContractMetadata, str, level string) {
	entry := indexer.log().WithField("contract", cm.Contract).WithField("link", cm.Link)
	switch level {
	case "info":
		entry.Info(str)
	case "warn":
		entry.Warn(str)
	case "error":
		entry.Error(str)
	}
}

func (indexer *Indexer) resolveContractMetadata(cm *models.ContractMetadata) {
	indexer.logContractMetadata(*cm, "Trying to resolve", "info")
	data, err := indexer.resolver.Resolve(cm.Network, cm.Contract, cm.Link)
	if err != nil {
		switch {
		case errors.Is(err, resolver.ErrNoIPFSResponse) || errors.Is(err, resolver.ErrTezosStorageKeyNotFound):
			cm.RetryCount += 1
			if cm.RetryCount < int(indexer.settings.MaxRetryCountOnError) {
				indexer.logContractMetadata(*cm, fmt.Sprintf("Retry: %s", err.Error()), "warn")
			} else {
				cm.Status = models.StatusFailed
				indexer.logContractMetadata(*cm, "Failed", "warn")
			}
		default:
			cm.Status = models.StatusFailed
			indexer.logContractMetadata(*cm, "Failed", "warn")
		}
	} else {
		cm.Metadata = helpers.Escape(data)
		cm.Status = models.StatusApplied
	}
	cm.UpdateID = indexer.contractActionsCounter.Increment()
}

func (indexer *Indexer) onContractTick() error {
	uresolved, err := indexer.db.GetContractMetadata(models.StatusNew, 15, 0)
	if err != nil {
		return err
	}
	for i := range uresolved {
		indexer.resolveContractMetadata(&uresolved[i])
		if err := indexer.db.UpdateContractMetadata(&uresolved[i], map[string]interface{}{
			"status":      uresolved[i].Status,
			"metadata":    uresolved[i].Metadata,
			"retry_count": uresolved[i].RetryCount,
			"update_id":   uresolved[i].UpdateID,
		}); err != nil {
			return err
		}
	}
	return nil
}
