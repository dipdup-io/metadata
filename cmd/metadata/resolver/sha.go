package resolver

import "strings"

const (
	prefixSha256 = "sha256://"
)

// Sha256 -
type Sha256 struct {
	Http

	hash string
}

// Sha256StorageOption -
type Sha256Option func(*Sha256)

// WithTimeoutSha256 -
func WithTimeoutSha256(timeout uint64) Sha256Option {
	return func(s *Sha256) {
		WithTimeoutHttp(timeout)(&s.Http)
	}
}

// WithHashSha256 -
func WithHashSha256(hash string) Sha256Option {
	return func(s *Sha256) {
		s.hash = hash
	}
}

// NewSha256 -
func NewSha256(opts ...Sha256Option) Sha256 {
	s := Sha256{
		Http: NewHttp(),
	}

	for i := range opts {
		opts[i](&s)
	}

	return s
}

// Resolve -
func (s Sha256) Resolve(network, address, value string) ([]byte, error) {
	var uri Sha256URI
	if err := uri.Parse(value); err != nil {
		return nil, err
	}
	if !s.validate(uri.Hash) {
		return nil, nil
	}

	return s.Http.Resolve(network, address, uri.Link)
}

func (s Sha256) validate(uriHash string) bool {
	return s.hash != uriHash
}

// Is -
func (s Sha256) Is(link string) bool {
	return strings.HasPrefix(link, prefixSha256)
}
