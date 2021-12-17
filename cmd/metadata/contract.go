package main

import (
	"bytes"
	"context"
	stdJSON "encoding/json"
	"fmt"
	"unicode/utf8"

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
	indexer.incrementCounter("contract", metadata.Status)

	return &metadata, nil
}

func (indexer *Indexer) logContractMetadata(cm models.ContractMetadata, str, level string) {
	entry := indexer.log().Str("contract", cm.Contract).Str("link", cm.Link)
	switch level {
	case "info":
		entry.Msg(str)
	case "warn":
		entry.Msg(str)
	case "error":
		entry.Msg(str)
	}
}

func (indexer *Indexer) resolveContractMetadata(ctx context.Context, cm *models.ContractMetadata) error {
	indexer.logContractMetadata(*cm, "Trying to resolve", "info")
	data, err := indexer.resolver.Resolve(ctx, cm.Network, cm.Contract, cm.Link)
	if err != nil {
		if e, ok := err.(resolver.ResolvingError); ok {
			indexer.incrementErrorCounter(e)
			err = e.Err
		}

		switch {
		case errors.Is(err, resolver.ErrNoIPFSResponse) || errors.Is(err, resolver.ErrTezosStorageKeyNotFound):
			cm.RetryCount += 1
			if cm.RetryCount < int8(indexer.settings.MaxRetryCountOnError) {
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
		escaped := helpers.Escape(data)

		if utf8.Valid(escaped) {
			cm.Status = models.StatusApplied

			var dst bytes.Buffer
			if err := stdJSON.Compact(&dst, escaped); err != nil {
				cm.Metadata = escaped
			} else {
				cm.Metadata = dst.Bytes()
			}
		} else {
			cm.Metadata = escaped
			cm.Status = models.StatusFailed
		}
	}
	cm.UpdateID = indexer.contractActionsCounter.Increment()
	indexer.incrementCounter("contract", cm.Status)
	return nil
}
