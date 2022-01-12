package resolver

import (
	"context"
	"strings"
	"time"

	"github.com/dipdup-net/metadata/internal/ipfs"

	shell "github.com/ipfs/go-ipfs-api"
)

const (
	prefixIpfs = "ipfs://"
)

// Ipfs -
type Ipfs struct {
	pinning  []*shell.Shell
	pool     *ipfs.Pool
	timeout  time.Duration
	fallback string
}

// IpfsOption -
type IpfsOption func(*Ipfs)

// WithPinningIpfs -
func WithPinningIpfs(urls []string) IpfsOption {
	return func(s *Ipfs) {
		if s.pinning == nil {
			s.pinning = make([]*shell.Shell, 0)
		}
		for _, url := range urls {
			sh := shell.NewShell(url)
			sh.SetTimeout(10 * time.Second)
			s.pinning = append(s.pinning, sh)
		}
	}
}

// WithTimeoutIpfs -
func WithTimeoutIpfs(timeout uint64) IpfsOption {
	return func(s *Ipfs) {
		s.timeout = time.Duration(timeout) * time.Second
	}
}

// WithFallbackIpfs -
func WithFallbackIpfs(fallback string) IpfsOption {
	return func(s *Ipfs) {
		s.fallback = fallback
	}
}

// NewIPFS -
func NewIPFS(gateways []string, opts ...IpfsOption) (Ipfs, error) {
	pool, err := ipfs.NewPool(gateways, 1024*1024)
	if err != nil {
		return Ipfs{}, err
	}
	s := Ipfs{
		pinning: make([]*shell.Shell, 0),
		pool:    pool,
	}

	for i := range opts {
		opts[i](&s)
	}

	return s, nil
}

// Resolve -
func (s Ipfs) Resolve(ctx context.Context, network, address, link string) (ipfs.Data, error) {
	path := ipfs.Path(link)
	for _, sh := range s.pinning {
		_ = sh.Pin(path)
	}

	requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	data, err := s.pool.GetFromRandomGateway(requestCtx, link)
	if err != nil {
		if s.fallback == "" {
			return data, newResolvingError(0, ErrorTypeHttpRequest, err)
		}

		requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
		defer cancel()

		data, err = s.pool.GetFromNode(requestCtx, link, s.fallback)
		if err != nil {
			return data, err
		}
	}

	if len(s.pinning) > 0 {
		s.pinContent(data.Raw)
	}
	return data, nil
}

// Is -
func (s Ipfs) Is(link string) bool {
	return strings.HasPrefix(link, prefixIpfs)
}

func (s Ipfs) pinContent(data []byte) {
	hash := ipfs.FindAllLinks(data)

	for i := range hash {
		for _, sh := range s.pinning {
			_ = sh.Pin(hash[i])
		}
	}
}
