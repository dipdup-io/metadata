package resolver

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/dipdup-net/metadata/cmd/metadata/config"
	internalContext "github.com/dipdup-net/metadata/cmd/metadata/context"
	"github.com/pkg/errors"
)

// ErrorType -
type ErrorType string

const (
	ErrorTypeHttpRequest     ErrorType = "http_request"
	ErrorTypeTooBig          ErrorType = "too_big"
	ErrorTypeReceiving       ErrorType = "receiving"
	ErrorTypeKeyTezosNotFond ErrorType = "tezos_key_not_found"
	ErrorTypeTezosURIParsing ErrorType = "tezos_uri_parsing"
	ErrorTypeInvalidJSON     ErrorType = "invalid_json"
)

// ResolvingError -
type ResolvingError struct {
	Code int
	Type ErrorType
	Err  error
}

// Error -
func (err ResolvingError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return string(err.Type)
}

func newResolvingError(code int, typ ErrorType, err error) ResolvingError {
	return ResolvingError{code, typ, err}
}

// Resolver -
type Resolver interface {
	Resolve(ctx context.Context, network, address, link string) ([]byte, error)
	Is(link string) bool
}

// Receiver -
type Receiver struct {
	resolvers []Resolver
}

// New -
func New(settings config.Settings, ctx *internalContext.Context) Receiver {
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
func (r Receiver) Resolve(ctx context.Context, network, address, link string) ([]byte, error) {
	if len(link) < 7 { // the shortest prefix is http://
		return nil, errors.Wrap(ErrUnknownStorageType, link)
	}

	for i := range r.resolvers {
		if r.resolvers[i].Is(link) {
			data, err := r.resolvers[i].Resolve(ctx, network, address, link)
			if err != nil {
				return nil, err
			}
			if !json.Valid(data) {
				return nil, newResolvingError(0, ErrorTypeInvalidJSON, errors.New("invalid json"))
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
