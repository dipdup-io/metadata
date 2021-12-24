package main

import (
	"bytes"
	"context"
	"encoding/hex"
	stdJSON "encoding/json"
	"fmt"
	"net/url"
	"unicode/utf8"

	jsoniter "github.com/json-iterator/go"
	"github.com/shopspring/decimal"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// TokenInfo -
type TokenInfo struct {
	TokenID   decimal.Decimal   `json:"token_id"`
	TokenInfo map[string]string `json:"token_info"`
	Link      string            `json:"-"`
}

type tokenMetadataBigMap struct {
	TokenID   string            `json:"token_id"`
	TokenInfo map[string]string `json:"token_info"`
}

// UnmarshalJSON -
func (tokenInfo *TokenInfo) UnmarshalJSON(data []byte) error {
	var ti tokenMetadataBigMap
	if err := json.Unmarshal(data, &ti); err != nil {
		return err
	}

	tokenID, err := decimal.NewFromString(ti.TokenID)
	if err != nil {
		return err
	}
	tokenInfo.TokenID = tokenID
	tokenInfo.TokenInfo = ti.TokenInfo

	if link, ok := tokenInfo.TokenInfo[""]; ok {
		b, err := hex.DecodeString(link)
		if err != nil {
			return err
		}
		if utf8.Valid(b) {
			tokenInfo.Link = string(b)
		}
		delete(tokenInfo.TokenInfo, "")
	}

	decodeMap(tokenInfo.TokenInfo)

	return nil
}

func decodeMap(m map[string]string) {
	for key, value := range m {
		if b, err := hex.DecodeString(value); err == nil && utf8.Valid(b) {
			m[key] = string(b)
		}
	}
}

func (indexer *Indexer) processTokenMetadata(update api.BigMapUpdate) (*models.TokenMetadata, error) {
	if update.Content == nil {
		return nil, nil
	}

	var tokenInfo TokenInfo
	if err := json.Unmarshal(update.Content.Value, &tokenInfo); err != nil {
		return nil, err
	}

	metadata, err := json.Marshal(tokenInfo.TokenInfo)
	if err != nil {
		return nil, err
	}

	token := models.TokenMetadata{
		Network:  indexer.network,
		Contract: update.Contract.Address,
		TokenID:  tokenInfo.TokenID,
		Status:   models.StatusNew,
		Metadata: helpers.Escape(metadata),
		UpdateID: indexer.tokenActionsCounter.Increment(),
	}

	if _, err := url.ParseRequestURI(tokenInfo.Link); err != nil {
		token.Status = models.StatusApplied
	} else {
		token.Link = tokenInfo.Link
	}

	indexer.incrementCounter("token", token.Status)

	return &token, nil
}

func (indexer *Indexer) logTokenMetadata(tm models.TokenMetadata, str, level string) {
	entry := indexer.log().Str("contract", tm.Contract).Str("token_id", tm.TokenID.String()).Str("link", tm.Link)
	switch level {
	case "info":
		entry.Msg(str)
	case "warn":
		entry.Msg(str)
	case "error":
		entry.Msg(str)
	}
}

func (indexer *Indexer) resolveTokenMetadata(ctx context.Context, tm *models.TokenMetadata) error {
	indexer.logTokenMetadata(*tm, "Trying to resolve", "info")
	tm.RetryCount += 1

	data, err := indexer.resolver.Resolve(ctx, tm.Network, tm.Contract, tm.Link)
	if err != nil {
		if e, ok := err.(resolver.ResolvingError); ok {
			indexer.incrementErrorCounter(e)
			err = e.Err
		}

		if tm.RetryCount < int8(indexer.settings.MaxRetryCountOnError) {
			indexer.logTokenMetadata(*tm, fmt.Sprintf("Retry: %s", err.Error()), "warn")
		} else {
			tm.Status = models.StatusFailed
			indexer.logTokenMetadata(*tm, "Failed", "warn")
		}
	} else {
		metadata, err := mergeTokenMetadata(tm.Metadata, data)
		if err != nil {
			return err
		}

		escaped := helpers.Escape(data)
		if utf8.Valid(metadata) {
			tm.Status = models.StatusApplied

			var dst bytes.Buffer
			if err := stdJSON.Compact(&dst, escaped); err != nil {
				tm.Metadata = escaped
			} else {
				tm.Metadata = dst.Bytes()
			}
		} else {
			tm.Status = models.StatusFailed
			tm.Metadata = escaped
		}
	}

	indexer.incrementCounter("token", tm.Status)
	tm.UpdateID = indexer.tokenActionsCounter.Increment()
	return nil
}

func mergeTokenMetadata(src, got []byte) ([]byte, error) {
	if len(src) == 0 {
		return got, nil
	}

	if len(got) == 0 {
		return src, nil
	}

	srcMap := make(map[string]interface{})
	if err := json.Unmarshal(src, &srcMap); err != nil {
		return nil, err
	}
	gotMap := make(map[string]interface{})
	if err := json.Unmarshal(got, &gotMap); err != nil {
		return nil, err
	}

	for key, value := range gotMap {
		if _, ok := srcMap[key]; !ok {
			srcMap[key] = value
		}
	}
	return json.Marshal(srcMap)
}
