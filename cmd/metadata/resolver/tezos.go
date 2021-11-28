package resolver

import (
	"strings"

	"context"

	internalContext "github.com/dipdup-net/metadata/cmd/metadata/context"
)

// prefixes
const (
	PrefixTezosStorage = "tezos-storage:"
)

// TezosStorage -
type TezosStorage struct {
	ctx *internalContext.Context
}

// NewTezosStorage -
func NewTezosStorage(ctx *internalContext.Context) TezosStorage {
	return TezosStorage{ctx}
}

// Resolve -
func (s TezosStorage) Resolve(ctx context.Context, network, address, value string) ([]byte, error) {
	var uri TezosURI
	if err := uri.Parse(value); err != nil {
		return nil, newResolvingError(0, ErrorTypeTezosURIParsing, err)
	}

	if uri.Network == "" {
		uri.Network = network
	}

	if uri.Address == "" {
		uri.Address = address
	}

	item, ok := s.ctx.Get(uri.Network, uri.Address, uri.Key)
	if !ok {
		return nil, newResolvingError(0, ErrorTypeKeyTezosNotFond, ErrTezosStorageKeyNotFound)
	}

	return item.Value, nil
}

// Is -
func (s TezosStorage) Is(link string) bool {
	return strings.HasPrefix(link, PrefixTezosStorage)
}
