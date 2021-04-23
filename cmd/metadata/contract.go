package main

import (
	"fmt"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (indexer *Indexer) processContractMetadata(update api.BigMapUpdate, tx *gorm.DB) error {
	if update.Content.Hash != emptyHash {
		return indexer.ctx.Add(update, indexer.network)
	}

	link, err := helpers.Decode(update.Content.Value)
	if err != nil {
		return err
	}

	metadata := &models.ContractMetadata{
		Network:  indexer.network,
		Contract: update.Contract.Address,
		Status:   models.StatusNew,
		Link:     string(link),
	}

	return tx.Save(metadata).Error
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
			if cm.RetryCount < int(indexer.maxRetryCount) {
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
		cm.Metadata = data
		cm.Status = models.StatusApplied
	}
}

func (indexer *Indexer) onContractFlush(tx *gorm.DB, flushed []interface{}) error {
	if len(flushed) == 0 {
		return nil
	}

	return indexer.db.Transaction(func(tx *gorm.DB) error {
		for i := range flushed {
			cm, ok := flushed[i].(*models.ContractMetadata)
			if !ok {
				return errors.Errorf("Invalid contract's queue type: %T", flushed[i])
			}
			if err := tx.Clauses(clause.OnConflict{
				UpdateAll: true,
			}).Create(cm).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (indexer *Indexer) onContractTick(tx *gorm.DB) error {
	uresolved, err := models.GetContractMetadata(indexer.db, models.StatusNew, 15, 0)
	if err != nil {
		return err
	}
	for i := range uresolved {
		indexer.resolveContractMetadata(&uresolved[i])
		indexer.contracts.Add(&uresolved[i])
	}
	return nil
}
