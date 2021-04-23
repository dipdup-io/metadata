package resolver

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

// TezosURI -
type TezosURI struct {
	Address string
	Network string
	Key     string
}

// Parse -
func (uri *TezosURI) Parse(value string) (err error) {
	if !strings.HasPrefix(value, PrefixTezosStorage) {
		return errors.Wrap(ErrInvalidTezosStoragePrefix, value)
	}

	uri.Key = strings.TrimPrefix(value, PrefixTezosStorage)
	if strings.HasPrefix(uri.Key, "//") {
		uri.Key = strings.TrimPrefix(uri.Key, "//")
		parts := strings.Split(uri.Key, "/")
		if len(parts) > 1 {
			uri.parseHost(parts[0])

			if len(parts) == 2 {
				uri.Key, err = url.QueryUnescape(parts[1])
				if err != nil {
					return
				}
			}
		}
	}
	return
}

func (uri *TezosURI) parseHost(host string) {
	parts := strings.Split(host, ".")
	if isAddress(parts[0]) {
		uri.Address = parts[0]
	}

	if len(parts) == 2 {
		uri.Network = parts[1]
	}
}

// Sha256URI -
type Sha256URI struct {
	Hash string
	Link string
}

// Parse -
func (uri *Sha256URI) Parse(value string) error {
	if !strings.HasPrefix(value, prefixSha256) {
		return errors.Wrap(ErrInvalidSha256Prefix, value)
	}

	key := strings.TrimPrefix(value, prefixSha256)
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return errors.Wrap(ErrInvalidURI, value)
	}

	uri.Hash = parts[0]
	link, err := url.QueryUnescape(parts[1])
	if err != nil {
		return err
	}
	uri.Link = link
	return nil
}

func isAddress(str string) bool {
	return len(str) == 36 && (strings.HasPrefix(str, "KT") || strings.HasPrefix(str, "tz1") || strings.HasPrefix(str, "tz2") || strings.HasPrefix(str, "tz3"))
}
