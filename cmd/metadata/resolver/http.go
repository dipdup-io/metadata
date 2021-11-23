package resolver

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/pkg/errors"
)

const (
	prefixHttp  = "http://"
	prefixHttps = "https://"
)

// HTTPStorage -
type Http struct {
	timeout time.Duration
}

// HttpOption -
type HttpOption func(*Http)

// WithTimeoutHttp -
func WithTimeoutHttp(timeout uint64) HttpOption {
	return func(s *Http) {
		if timeout != 0 {
			s.timeout = time.Duration(timeout) * time.Second
		}
	}
}

// NewHttp -
func NewHttp(opts ...HttpOption) Http {
	s := Http{
		timeout: time.Duration(defaultTimeout) * time.Second,
	}

	for i := range opts {
		opts[i](&s)
	}

	return s
}

// Resolve -
func (s Http) Resolve(ctx context.Context, network, address, link string) ([]byte, error) {
	if _, err := url.ParseRequestURI(link); err != nil {
		return nil, ErrInvalidURI
	}
	client := http.Client{
		Timeout: s.timeout,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, newResolvingError(0, ErrorTypeReceiving, errors.Wrap(ErrHTTPRequest, err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newResolvingError(resp.StatusCode, ErrorTypeHttpRequest, errors.Errorf("invalid status: %s", resp.Status))
	}

	data, err := ioutil.ReadAll(io.LimitReader(resp.Body, 20971520)) // 20 MB limit for metadata
	if err != nil {
		return nil, newResolvingError(0, ErrorTypeTooBig, err)
	}

	return helpers.Escape(data), nil
}

// Is -
func (s Http) Is(link string) bool {
	return strings.HasPrefix(link, prefixHttp) || strings.HasPrefix(link, prefixHttps)
}
