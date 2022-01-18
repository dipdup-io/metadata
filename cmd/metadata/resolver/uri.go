package resolver

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

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
