package resolver

import (
	"bytes"
	"context"
	stdJSON "encoding/json"

	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/metadata/tezoskeys"
	"github.com/dipdup-net/metadata/internal/ipfs"
	"github.com/dipdup-net/metadata/internal/tezos"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

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
	ErrorInvalidCID          ErrorType = "invalid_ipfs_cid"
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
	ipfs  IpfsNode
	sha   Sha256
	tezos TezosStorage
}

// New -
func New(ctx context.Context, settings config.Settings, tezosKeys *tezoskeys.TezosKeys, node *ipfs.Node) (Receiver, error) {
	ipfs, err := NewIPFSNode(node,
		WithTimeoutIpfsNode(settings.IPFS.Timeout),
	)
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
func (r Receiver) Resolve(ctx context.Context, network, address, link string, attempt int8) (resolved Resolved, err error) {
	if len(link) < 7 { // the shortest prefix is http://
		return resolved, errors.Wrap(ErrUnknownStorageType, link)
	}

	switch {
	case r.ipfs.Is(link):
		resolved.By = ResolverTypeIPFS
		if attempt == 3 && network == "mainnet" {
			if err := r.ipfs.FindPeers(ctx, link); err != nil {
				log.Err(err).Str("link", link).Str("network", network).Msg("can't find peers for CID")
			}
		}

		data, err := r.ipfs.Resolve(ctx, network, address, link)
		if err != nil {
			if errors.Is(err, ipfs.ErrInvalidCID) {
				return resolved, newResolvingError(0, ErrorInvalidCID, err)
			}
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

	resolved.Data = bytes.TrimLeft(resolved.Data, " ")
	if len(resolved.Data) == 0 || resolved.Data[0] != '{' || !json.Valid(resolved.Data) {
		return resolved, newResolvingError(0, ErrorTypeInvalidJSON, errors.New("invalid json"))
	}

	var buf bytes.Buffer
	if err := stdJSON.Compact(&buf, resolved.Data); err != nil {
		return resolved, err
	}
	resolved.Data = buf.Bytes()
	return
}
