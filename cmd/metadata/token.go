package main

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"unicode/utf8"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// TokenInfo -
type TokenInfo struct {
	TokenID   uint64            `json:"token_id"`
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

	tokenID, err := strconv.ParseUint(ti.TokenID, 10, 64)
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

	return &token, nil
}

func (indexer *Indexer) logTokenMetadata(tm models.TokenMetadata, str, level string) {
	entry := indexer.log().WithField("contract", tm.Contract).WithField("token_id", tm.TokenID).WithField("link", tm.Link)
	switch level {
	case "info":
		entry.Info(str)
	case "warn":
		entry.Warn(str)
	case "error":
		entry.Error(str)
	}
}

func (indexer *Indexer) resolveTokenMetadata(tm *models.TokenMetadata) error {
	indexer.logTokenMetadata(*tm, "Trying to resolve", "info")
	data, err := indexer.resolver.Resolve(tm.Network, tm.Contract, tm.Link)
	if err != nil {
		switch {
		case errors.Is(err, resolver.ErrNoIPFSResponse) || errors.Is(err, resolver.ErrTezosStorageKeyNotFound):
			tm.RetryCount += 1
			if tm.RetryCount < int(indexer.settings.MaxRetryCountOnError) {
				indexer.logTokenMetadata(*tm, fmt.Sprintf("Retry: %s", err.Error()), "warn")
			} else {
				tm.Status = models.StatusFailed
				indexer.logTokenMetadata(*tm, "Failed", "warn")
			}
		default:
			tm.Status = models.StatusFailed
			indexer.logTokenMetadata(*tm, "Failed", "warn")
		}
	} else {
		metadata, err := mergeTokenMetadata(tm.Metadata, data)
		if err != nil {
			return err
		}

		if utf8.Valid(metadata) {
			tm.Metadata = helpers.Escape(metadata)
			tm.Status = models.StatusApplied
		} else {
			tm.Status = models.StatusFailed
		}
	}
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

func (indexer *Indexer) onTokenTick() error {
	uresolved, err := indexer.db.GetTokenMetadata(models.StatusNew, 15, 0)
	if err != nil {
		return err
	}

	for i := range uresolved {
		if err := indexer.resolveTokenMetadata(&uresolved[i]); err != nil {
			return err
		}
		if err := indexer.db.UpdateTokenMetadata(&uresolved[i], map[string]interface{}{
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
