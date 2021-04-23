package resolver

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

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
func (s Http) Resolve(network, address, link string) ([]byte, error) {
	if _, err := url.ParseRequestURI(link); err != nil {
		return nil, ErrInvalidURI
	}
	client := http.Client{
		Timeout: s.timeout,
	}
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(ErrHTTPRequest, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Invalid status code: %s", resp.Status)
	}

	return ioutil.ReadAll(io.LimitReader(resp.Body, 20971520)) // 20 MB limit for metadata
}

// Is -
func (s Http) Is(link string) bool {
	return strings.HasPrefix(link, prefixHttp) || strings.HasPrefix(link, prefixHttps)
}
