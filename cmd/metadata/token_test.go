package main

import (
	stdJSON "encoding/json"
	"testing"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestIndexer_processTokenMetadata(t *testing.T) {
	tests := []struct {
		name    string
		update  api.BigMapUpdate
		want    *models.TokenMetadata
		wantErr bool
	}{
		{
			name: "KT1G1cCRNBgQ48mVDjopHjEmTN5Sbtar8nn9",
			update: api.BigMapUpdate{
				ID:     4163559,
				Level:  1477522,
				Bigmap: 3688,
				Path:   "token_metadata",
				Action: "add_key",
				Contract: api.Address{
					Alias:   "Hedgehoge",
					Address: "KT1G1cCRNBgQ48mVDjopHjEmTN5Sbtar8nn9",
				},
				Content: &api.BigMapUpdateContent{
					Hash: "exprtZBwZUeYYYfUs9B9Rg2ywHezVHnCCnmF9WsDQVrs582dSK63dC",
					Key:  stdJSON.RawMessage("0"),
					Value: stdJSON.RawMessage(`{
					  "int": "0",
					  "map": {
						"icon": "697066733a2f2f516d584c33465a356b63775843386d64776b5331694348533271566f796736397567426855326170387a317a6373",
						"name": "4865646765686f6765",
						"symbol": "484548",
						"decimals": "36",
						"test_object": "7b7d"
					  }
					}`),
				},
			},
			want: &models.TokenMetadata{
				TokenID:    decimal.NewFromInt(0),
				Contract:   "KT1G1cCRNBgQ48mVDjopHjEmTN5Sbtar8nn9",
				Metadata:   models.JSONB(`{"decimals":"6","icon":"ipfs://QmXL3FZ5kcwXC8mdwkS1iCHS2qVoyg69ugBhU2ap8z1zcs","name":"Hedgehoge","symbol":"HEH","test_object":"{}"}`),
				Status:     models.StatusApplied,
				RetryCount: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexer := &Indexer{}
			got, err := indexer.processTokenMetadata(tt.update)
			if (err != nil) != tt.wantErr {
				t.Errorf("Indexer.processTokenMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
