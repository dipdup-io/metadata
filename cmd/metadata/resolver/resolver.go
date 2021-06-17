package resolver

import (
	"bytes"
	"encoding/json"

	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/context"
	"github.com/pkg/errors"
)

// Resolver -
type Resolver interface {
	Resolve(network, address, link string) ([]byte, error)
	Is(link string) bool
}

// Receiver -
type Receiver struct {
	resolvers []Resolver
}

// New -
func New(settings config.Settings, ctx *context.Context) Receiver {
	return Receiver{
		[]Resolver{
			NewIPFS(settings.IPFSGateways, WithTimeoutIpfs(settings.IPFSTimeout), WithPinningIpfs(settings.IPFSPinning)),
			NewTezosStorage(ctx),
			NewHttp(WithTimeoutHttp(settings.HTTPTimeout)),
			NewSha256(WithTimeoutSha256(settings.HTTPTimeout)),
		},
	}
}

// Resolve -
func (r Receiver) Resolve(network, address, link string) ([]byte, error) {
	if len(link) < 7 { // the shortest prefix is http://
		return nil, errors.Wrap(ErrUnknownStorageType, link)
	}

	for i := range r.resolvers {
		if r.resolvers[i].Is(link) {
			data, err := r.resolvers[i].Resolve(network, address, link)
			if err != nil {
				return nil, err
			}
			if !json.Valid(data) {
				return nil, errors.New("Invalid JSON")
			}

			var buf bytes.Buffer
			if err := json.Compact(&buf, data); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}

	return nil, errors.Wrap(ErrUnknownStorageType, link)
}
