package resolver

import (
	"context"
	"strings"
	"time"

	"github.com/dipdup-net/metadata/internal/ipfs"
	"github.com/karlseguin/ccache"

	shell "github.com/ipfs/go-ipfs-api"
)

const (
	prefixIpfs = "ipfs://"
)

// Ipfs -
type Ipfs struct {
	cache   *ccache.Cache
	pinning []*shell.Shell
	pool    *ipfs.Pool
	timeout time.Duration
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

// NewIPFS -
func NewIPFS(gateways []string, opts ...IpfsOption) (Ipfs, error) {
	pool, err := ipfs.NewPool(gateways, 1024*1024)
	if err != nil {
		return Ipfs{}, err
	}
	s := Ipfs{
		pinning: make([]*shell.Shell, 0),
		pool:    pool,
		cache:   ccache.New(ccache.Configure().MaxSize(1000)),
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

	data, err := s.cache.Fetch(link, time.Hour, func() (interface{}, error) {
		requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
		defer cancel()

		return s.pool.Get(requestCtx, link)
	})
	if err != nil {
		return ipfs.Data{}, newResolvingError(0, ErrorTypeHttpRequest, err)
	}
	content := data.Value().(ipfs.Data)
	if len(s.pinning) > 0 {
		s.pinContent(content.Raw)
	}
	return content, nil
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
