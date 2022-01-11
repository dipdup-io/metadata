package resolver

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/dipdup-net/metadata/cmd/metadata/config"
	internalContext "github.com/dipdup-net/metadata/cmd/metadata/context"
	"github.com/karlseguin/ccache"
	"github.com/pkg/errors"
)

// ResolverType -
type ResolverType int

// resolver types
const (
	ResolverTypeIPFS ResolverType = iota + 1
	ResolverTypeHTTP
	ResolverTypeTezos
	ResolverTypeSha256
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

// Resolved -
type Resolved struct {
	By   ResolverType
	Node string
	Data []byte
}

// Receiver -
type Receiver struct {
	http  Http
	ipfs  Ipfs
	sha   Sha256
	tezos TezosStorage
}

// New -
func New(settings config.Settings, ctx *internalContext.Context) (Receiver, error) {
	ipfs, err := NewIPFS(settings.IPFSGateways, WithTimeoutIpfs(settings.IPFSTimeout), WithPinningIpfs(settings.IPFSPinning))
	if err != nil {
		return Receiver{}, err
	}
	return Receiver{
		ipfs:  ipfs,
		tezos: NewTezosStorage(ctx),
		http:  NewHttp(WithTimeoutHttp(settings.HTTPTimeout)),
		sha:   NewSha256(WithTimeoutSha256(settings.HTTPTimeout)),
	}, nil
}

// Init -
func (r Receiver) Init(initFunc func(*ccache.Cache) error) error {
	return initFunc(r.ipfs.cache)
}

// Resolve -
func (r Receiver) Resolve(ctx context.Context, network, address, link string) (resolved Resolved, err error) {
	if len(link) < 7 { // the shortest prefix is http://
		return resolved, errors.Wrap(ErrUnknownStorageType, link)
	}

	switch {
	case r.ipfs.Is(link):
		resolved.By = ResolverTypeIPFS
		data, err := r.ipfs.Resolve(ctx, network, address, link)
		if err != nil {
			return resolved, err
		}
		resolved.Data = data.Raw
		resolved.Node = data.Node

	case r.tezos.Is(link):
		resolved.By = ResolverTypeTezos
		resolved.Data, err = r.tezos.Resolve(ctx, network, address, link)

	case r.http.Is(link):
		resolved.By = ResolverTypeHTTP
		resolved.Data, err = r.http.Resolve(ctx, network, address, link)

	case r.sha.Is(link):
		resolved.By = ResolverTypeSha256
		resolved.Data, err = r.sha.Resolve(ctx, network, address, link)

	default:
		return resolved, errors.Wrap(ErrUnknownStorageType, link)
	}

	if err != nil {
		return
	}

	if !json.Valid(resolved.Data) {
		return resolved, newResolvingError(0, ErrorTypeInvalidJSON, errors.New("invalid json"))
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, resolved.Data); err != nil {
		return resolved, err
	}
	resolved.Data = buf.Bytes()
	return
}
