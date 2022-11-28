package main

import (
	"context"
	"fmt"
	"unicode/utf8"

	api "github.com/dipdup-net/go-lib/tzkt/data"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
)

func (indexer *Indexer) processContractMetadata(update api.BigMapUpdate) (*models.ContractMetadata, error) {
	if update.Content == nil {
		return nil, nil
	}
	if update.Content.Hash != emptyHash {
		return nil, indexer.tezosKeys.Add(update, indexer.network)
	}

	link, err := helpers.Decode(update.Content.Value)
	if err != nil {
		return nil, err
	}

	return &models.ContractMetadata{
		Network:  indexer.network,
		Contract: update.Contract.Address,
		Status:   models.StatusNew,
		Link:     string(link),
	}, nil
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
	cm.RetryCount += 1

	resolved, err := indexer.resolver.Resolve(ctx, cm.Network, cm.Contract, cm.Link)
	if err != nil {
		cm.Error = err.Error()
		if e, ok := err.(resolver.ResolvingError); ok {
			indexer.prom.IncrementErrorCounter(indexer.network, e)
			err = e.Err
		}

		if cm.RetryCount < int8(indexer.settings.MaxRetryCountOnError) {
			indexer.logContractMetadata(*cm, fmt.Sprintf("Retry: %s", err.Error()), "warn")
		} else {
			cm.Status = models.StatusFailed
			indexer.logContractMetadata(*cm, "Failed", "warn")
		}
	} else {
		cm.Metadata = resolved.Data
		if utf8.Valid(resolved.Data) {
			cm.Status = models.StatusApplied
			cm.Error = ""
			indexer.log().Int64("response_time", resolved.ResponseTime).Str("contract", cm.Contract).Msg("resolved contract metadata")
		} else {
			cm.Error = "invalid json"
			cm.Status = models.StatusFailed
		}
	}

	if resolved.By == resolver.ResolverTypeIPFS && cm.Status == models.StatusApplied {
		if resolved.ResponseTime > 0 {
			indexer.prom.AddHistogramResponseTime(indexer.network, resolved)
		}
	}
	return nil
}
