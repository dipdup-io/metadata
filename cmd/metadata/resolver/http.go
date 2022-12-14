package resolver

import (
	"context"
	"fmt"
	"io"
	"net"
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
	client  http.Client
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

	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 10
	t.MaxConnsPerHost = 10
	t.MaxIdleConnsPerHost = 10

	s.client = http.Client{
		Timeout:   s.timeout,
		Transport: t,
	}

	return s
}

// Resolve -
func (s Http) Resolve(ctx context.Context, network, address, link string) ([]byte, error) {
	parsed, err := url.ParseRequestURI(link)
	if err != nil {
		return nil, ErrInvalidURI
	}

	if err := s.ValidateURL(parsed); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, newResolvingError(0, ErrorTypeReceiving, errors.Wrap(ErrHTTPRequest, err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newResolvingError(resp.StatusCode, ErrorTypeHttpRequest, errors.Errorf("invalid status: %s", resp.Status))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 20971520)) // 20 MB limit for metadata
	if err != nil {
		return nil, newResolvingError(0, ErrorTypeTooBig, err)
	}

	return helpers.Escape(data), nil
}

// Is -
func (s Http) Is(link string) bool {
	return strings.HasPrefix(link, prefixHttp) || strings.HasPrefix(link, prefixHttps)
}

// ValidateURL -
func (s Http) ValidateURL(link *url.URL) error {
	host := link.Host
	if strings.Contains(host, ":") {
		newHost, _, err := net.SplitHostPort(link.Host)
		if err != nil {
			return err
		}
		host = newHost
	}
	if host == "localhost" || host == "127.0.0.1" {
		return errors.Wrap(ErrInvalidURI, fmt.Sprintf("invalid host: %s", host))
	}

	for _, mask := range []string{
		"10.0.0.0/8",
		"100.64.0.0/10",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"240.0.0.0/4",
	} {
		_, cidr, err := net.ParseCIDR(mask)
		if err != nil {
			return err
		}

		ip := net.ParseIP(host)
		if ip != nil && cidr.Contains(ip) {
			return errors.Wrap(ErrInvalidURI, fmt.Sprintf("restricted subnet: %s", mask))
		}
	}
	return nil
}
