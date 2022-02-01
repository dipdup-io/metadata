package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSha256URI_Parse(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		hash    string
		link    string
		wantErr bool
	}{
		{
			name:  "test 1",
			value: "sha256://0xeaa42ea06b95d7917d22135a630e65352cfd0a721ae88155a1512468a95cb750/https:%2F%2Ftezos.com",
			hash:  "0xeaa42ea06b95d7917d22135a630e65352cfd0a721ae88155a1512468a95cb750",
			link:  "https://tezos.com",
		}, {
			name:  "test 2",
			value: "sha256://0xeaa42ea06b95d7917d22135a630e65352cfd0a721ae88155a1512468a95cb750/https://tezos.com",
			hash:  "0xeaa42ea06b95d7917d22135a630e65352cfd0a721ae88155a1512468a95cb750",
			link:  "https://tezos.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := &Sha256URI{}
			if err := uri.Parse(tt.value); (err != nil) != tt.wantErr {
				t.Errorf("Sha256URI.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.Equal(t, tt.hash, uri.Hash) {
				t.Errorf("Sha256URI.Parse() hash = %v, want %v", uri.Hash, tt.hash)
				return
			}
			if !assert.Equal(t, tt.link, uri.Link) {
				t.Errorf("Sha256URI.Parse() link = %v, want %v", uri.Hash, tt.hash)
				return
			}
		})
	}
}
