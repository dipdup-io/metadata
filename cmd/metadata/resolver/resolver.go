package resolver

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/tezoskeys"
	"github.com/dipdup-net/metadata/internal/tezos"
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
	ErrorInvalidHTTPURI      ErrorType = "invalid_http_uri"
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
	By           ResolverType
	Node         string
	Data         []byte
	ResponseTime int64
	URI          tezos.URI
}

// Receiver -
type Receiver struct {
	http  Http
	ipfs  Ipfs
	sha   Sha256
	tezos TezosStorage
}

// New -
func New(settings config.Settings, tezosKeys *tezoskeys.TezosKeys) (Receiver, error) {
	ipfs, err := NewIPFS(settings.IPFS.Gateways,
		WithTimeoutIpfs(settings.IPFS.Timeout),
		WithPinningIpfs(settings.IPFS.Pinning),
		WithFallbackIpfs(settings.IPFS.Fallback))
	if err != nil {
		return Receiver{}, err
	}
	return Receiver{
		ipfs:  ipfs,
		tezos: NewTezosStorage(tezosKeys),
		http:  NewHttp(WithTimeoutHttp(settings.HTTPTimeout)),
		sha:   NewSha256(WithTimeoutSha256(settings.HTTPTimeout)),
	}, nil
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
		resolved.ResponseTime = data.ResponseTime

	case r.tezos.Is(link):
		resolved.By = ResolverTypeTezos
		data, err := r.tezos.Resolve(ctx, network, address, link)
		if err != nil {
			return resolved, err
		}
		resolved.Data = data.Data
		resolved.URI = data.URI

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
		if errors.Is(err, ErrInvalidURI) {
			return resolved, newResolvingError(0, ErrorInvalidHTTPURI, err)
		}
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
