package main

import (
	"context"
	"fmt"
	"unicode/utf8"

	api "github.com/dipdup-net/go-lib/tzkt/data"
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

func (indexer *Indexer) logContractMetadata(cm models.ContractMetadata, str string) {
	indexer.log().Str("contract", cm.Contract).Str("link", cm.Link).Msg(str)
}

func (indexer *Indexer) resolveContractMetadata(ctx context.Context, cm *models.ContractMetadata) error {
	indexer.logContractMetadata(*cm, "trying to resolve")
	cm.RetryCount += 1

	resolved, err := indexer.resolver.Resolve(ctx, cm.Network, cm.Contract, cm.Link, cm.RetryCount)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		cm.Error = err.Error()
		if e, ok := err.(resolver.ResolvingError); ok {
			indexer.prom.IncrementErrorCounter(indexer.network, e)
			err = e.Err

			if e.Type == resolver.ErrorInvalidHTTPURI ||
				e.Type == resolver.ErrorTypeInvalidJSON ||
				e.Type == resolver.ErrorInvalidCID {
				cm.RetryCount = int8(indexer.settings.MaxRetryCountOnError)
			}
		}

		if cm.RetryCount < int8(indexer.settings.MaxRetryCountOnError) {
			indexer.logContractMetadata(*cm, fmt.Sprintf("retry: %s", err.Error()))
		} else {
			cm.Status = models.StatusFailed
			indexer.logContractMetadata(*cm, "failed")
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
