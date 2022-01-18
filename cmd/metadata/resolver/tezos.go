package resolver

import (
	"context"

	"github.com/dipdup-net/metadata/cmd/metadata/tezoskeys"
	"github.com/dipdup-net/metadata/internal/tezos"
)

type tezosData struct {
	Data []byte
	URI  tezos.URI
}

// TezosStorage -
type TezosStorage struct {
	tk *tezoskeys.TezosKeys
}

// NewTezosStorage -
func NewTezosStorage(ctx *tezoskeys.TezosKeys) TezosStorage {
	return TezosStorage{ctx}
}

// Resolve -
func (s TezosStorage) Resolve(ctx context.Context, network, address, value string) (tezosData, error) {
	var uri tezos.URI
	if err := uri.Parse(value); err != nil {
		return tezosData{}, newResolvingError(0, ErrorTypeTezosURIParsing, err)
	}

	if uri.Network == "" {
		uri.Network = network
	}

	if uri.Address == "" {
		uri.Address = address
	}

	item, err := s.tk.Get(uri.Network, uri.Address, uri.Key)
	if err != nil {
		return tezosData{
			URI: uri,
		}, newResolvingError(0, ErrorTypeKeyTezosNotFond, ErrTezosStorageKeyNotFound)
	}

	return tezosData{
		URI:  uri,
		Data: item.Value,
	}, nil
}

// Is -
func (s TezosStorage) Is(link string) bool {
	return tezos.Is(link)
}
