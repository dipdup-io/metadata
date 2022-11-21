package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"unicode/utf8"

	jsoniter "github.com/json-iterator/go"
	"github.com/shopspring/decimal"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// TokenInfo -
type TokenInfo struct {
	TokenID   decimal.Decimal   `json:"token_id"`
	TokenInfo map[string]string `json:"token_info"`
	Link      string            `json:"-"`
}

// UnmarshalJSON -
func (tokenInfo *TokenInfo) UnmarshalJSON(data []byte) error {
	var generalTokenInfo map[string]interface{}
	if err := json.Unmarshal(data, &generalTokenInfo); err != nil {
		return err
	}
	for _, value := range generalTokenInfo {
		switch typedValue := value.(type) {
		case string:
			tokenID, err := decimal.NewFromString(typedValue)
			if err != nil {
				return err
			}
			tokenInfo.TokenID = tokenID
		case map[string]interface{}:
			tokenInfo.TokenInfo = make(map[string]string)
			for infoKey, infoValue := range typedValue {
				stringValue, ok := infoValue.(string)
				if !ok {
					continue
				}
				tokenInfo.TokenInfo[infoKey] = stringValue
			}
		}
	}

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
	}
	if len(metadata) > 2 {
		token.Metadata = helpers.Escape(metadata)
	}

	if _, err := url.ParseRequestURI(tokenInfo.Link); err != nil {
		token.Status = models.StatusApplied
		token.RetryCount = 1
		indexer.prom.IncrementMetadataCounter(indexer.network, prometheus.MetadataTypeToken, token.Status.String())
	} else {
		token.Link = tokenInfo.Link
		indexer.prom.IncrementMetadataNew(indexer.network, prometheus.MetadataTypeToken)
	}

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
	if _, ok := legacyContracts[tm.Contract]; ok {
		indexer.logTokenMetadata(*tm, "readonly legacy metadata", "info")
		return nil
	}

	indexer.logTokenMetadata(*tm, "trying to resolve", "info")
	tm.RetryCount += 1

	resolved, err := indexer.resolver.Resolve(ctx, tm.Network, tm.Contract, tm.Link)
	if err != nil {
		tm.Error = err.Error()
		if e, ok := err.(resolver.ResolvingError); ok {
			indexer.prom.IncrementErrorCounter(indexer.network, e)
			err = e.Err

			if e.Type == resolver.ErrorInvalidHTTPURI || e.Type == resolver.ErrorTypeInvalidJSON {
				tm.RetryCount = int8(indexer.settings.MaxRetryCountOnError)
			}
		}

		if tm.RetryCount < int8(indexer.settings.MaxRetryCountOnError) {
			indexer.logTokenMetadata(*tm, fmt.Sprintf("retry: %s", err.Error()), "warn")
		} else {
			tm.Status = models.StatusFailed
			indexer.logTokenMetadata(*tm, "failed", "warn")
		}
	} else {
		if utf8.Valid(resolved.Data) {
			tm.Status = models.StatusApplied
			tm.Error = ""
			tm.Metadata = resolved.Data
			indexer.log().Int64("response_time", resolved.ResponseTime).Str("contract", tm.Contract).Str("token_id", tm.TokenID.String()).Msg("resolved token metadata")
		} else {
			tm.Error = "invalid json"
			tm.Status = models.StatusFailed
		}
	}

	if resolved.By == resolver.ResolverTypeIPFS && tm.Status == models.StatusApplied {
		if resolved.ResponseTime > 0 {
			indexer.prom.AddHistogramResponseTime(indexer.network, resolved)
		}
	}
	return nil
}

var legacyTokens = []*models.TokenMetadata{
	{
		Network:        "mainnet",
		Contract:       "KT1PWx2mnDueood7fEmfbBDKx1D9BAnnXitn",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"tzBTC","symbol":"tzBTC","decimals":"8"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1VYsVfmobT7rsMVivvZ4J8i3bPiqz12NaH",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"wXTZ","symbol":"wXTZ","decimals":"6"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1LN4LPSqTMS7Sd2CJw4bbDGRkMv2t68Fy9",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"USDtez","symbol":"USDtz","decimals":"6"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT19at7rQUvyjxnZ2fBv7D9zc8rkyG7gAoU8",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"ETHtez","symbol":"ETHtz","decimals":"18"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1REEb5VxWRjcHm5GzDMwErMmNFftsE5Gpf",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"Stably USD","symbol":"USDS","decimals":"6"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1AEfeckNbdEYwaMKkytBwPJPycz7jdSGea",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"STKR","symbol":"STKR","decimals":"18"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1AafHA1C1vk959wvHWBispY9Y2f3fxBUUo",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"Sirius","symbol":"SIRS","decimals":"0"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1K9gCRgaLRFKTErYt1wVxA3Frb9FjasjTV",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"Kolibri USD","symbol":"kUSD","decimals":"18"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1AxaBxkFLCUi3f8rdDAAxBKHfzY8LfKDRA",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"Quipuswap Liquidating kUSD","symbol":"QLkUSD","decimals":"36"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1AFA2mwNUMNd4SsujE1YYp29vd8BZejyKW",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"Hic et nunc DAO","symbol":"hDAO","decimals":"6"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1S6t5PrHXnozytDU3vYdajmsenoBNYY8WJ",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"XTZGold","symbol":"XTZGOLD","decimals":"0"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1EqhKGcu9nztF5p9qa4c3cYVqVewQrJpi2",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"XTZSilver","symbol":"XTZSILVER","decimals":"0"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1XQZxsG4pMgcN7q7Nu3XFihsb9mEvqBmAT",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"QuipuSwap tCow","symbol":"tCOW","decimals":"0"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	}, {
		Network:        "mainnet",
		Contract:       "KT1LqEyTQxD2Dsdkk4LME5YGcBqazAwXrg4t",
		TokenID:        decimal.NewFromInt(0),
		Metadata:       models.JSONB(`{"name":"Werenode EVSE ledger","symbol":"EVSE","decimals":"0"}`),
		Status:         models.StatusApplied,
		RetryCount:     1,
		ImageProcessed: true,
	},
}

var legacyContracts = map[string]struct{}{
	"KT1K9gCRgaLRFKTErYt1wVxA3Frb9FjasjTV": {},
	"KT1AxaBxkFLCUi3f8rdDAAxBKHfzY8LfKDRA": {},
	"KT1AFA2mwNUMNd4SsujE1YYp29vd8BZejyKW": {},
}

func (indexer *Indexer) initialTokenMetadata(ctx context.Context) error {
	for i := range legacyTokens {
		legacyTokens[i].UpdateID = models.TokenUpdateID.Increment()
	}
	return indexer.db.Tokens.Save(legacyTokens)
}
