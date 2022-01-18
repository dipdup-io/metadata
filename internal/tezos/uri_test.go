package tezos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTezosStorageURI_Parse(t *testing.T) {
	type fields struct {
		Address string
		Network string
		Key     string
	}
	tests := []struct {
		name    string
		fields  fields
		value   string
		wantErr bool
	}{
		{
			name:  "test 1",
			value: "tezos-storage:hello",
			fields: fields{
				Address: "",
				Network: "",
				Key:     "hello",
			},
		}, {
			name:  "test 2",
			value: "tezos-storage://KT1QDFEu8JijYbsJqzoXq7mKvfaQQamHD1kX/foo",
			fields: fields{
				Address: "KT1QDFEu8JijYbsJqzoXq7mKvfaQQamHD1kX",
				Network: "",
				Key:     "foo",
			},
		}, {
			name:  "test 3",
			value: "tezos-storage://KT1QDFEu8JijYbsJqzoXq7mKvfaQQamHD1kX/%2Ffoo",
			fields: fields{
				Address: "KT1QDFEu8JijYbsJqzoXq7mKvfaQQamHD1kX",
				Network: "",
				Key:     "/foo",
			},
		}, {
			name:  "test 4",
			value: "tezos-storage://KT1QDFEu8JijYbsJqzoXq7mKvfaQQamHD1kX.mainnet/%2Ffoo",
			fields: fields{
				Address: "KT1QDFEu8JijYbsJqzoXq7mKvfaQQamHD1kX",
				Network: "mainnet",
				Key:     "/foo",
			},
		}, {
			name:  "test 5",
			value: "tezos-storage:metadata",
			fields: fields{
				Address: "",
				Network: "",
				Key:     "metadata",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := &URI{}
			if err := uri.Parse(tt.value); (err != nil) != tt.wantErr {
				t.Errorf("TezosStorageURI.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.Equal(t, tt.fields.Address, uri.Address) {
				t.Errorf("Sha256URI.Parse() address = %v, want %v", uri.Address, tt.fields.Address)
				return
			}
			if !assert.Equal(t, tt.fields.Network, uri.Network) {
				t.Errorf("Sha256URI.Parse() network = %v, want %v", uri.Network, tt.fields.Network)
				return
			}
			if !assert.Equal(t, tt.fields.Key, uri.Key) {
				t.Errorf("Sha256URI.Parse() key = %v, want %v", uri.Key, tt.fields.Key)
				return
			}
		})
	}
}
