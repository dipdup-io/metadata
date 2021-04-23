package resolver

import (
	"strings"

	"github.com/dipdup-net/metadata/cmd/metadata/context"
)

// prefixes
const (
	PrefixTezosStorage = "tezos-storage:"
)

// TezosStorage -
type TezosStorage struct {
	ctx *context.Context
}

// NewTezosStorage -
func NewTezosStorage(ctx *context.Context) TezosStorage {
	return TezosStorage{ctx}
}

// Resolve -
func (s TezosStorage) Resolve(network, address, value string) ([]byte, error) {
	var uri TezosURI
	if err := uri.Parse(value); err != nil {
		return nil, err
	}

	if uri.Network == "" {
		uri.Network = network
	}

	if uri.Address == "" {
		uri.Address = address
	}

	item, ok := s.ctx.Get(uri.Network, uri.Address, uri.Key)
	if !ok {
		return nil, ErrTezosStorageKeyNotFound
	}

	return item.Value, nil
}

// Is -
func (s TezosStorage) Is(link string) bool {
	return strings.HasPrefix(link, PrefixTezosStorage)
}
