package resolver

import (
	"strings"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/karlseguin/ccache"

	shell "github.com/ipfs/go-ipfs-api"
)

const (
	prefixIpfs = "ipfs://"
)

// Ipfs -
type Ipfs struct {
	Http

	cache    *ccache.Cache
	pinning  []*shell.Shell
	gateways []string
}

// IpfsOption -
type IpfsOption func(*Ipfs)

// WithTimeoutIpfs -
func WithTimeoutIpfs(timeout uint64) IpfsOption {
	return func(s *Ipfs) {
		WithTimeoutHttp(timeout)(&s.Http)
	}
}

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

// NewIPFS -
func NewIPFS(gateways []string, opts ...IpfsOption) Ipfs {
	s := Ipfs{
		Http:     NewHttp(),
		pinning:  make([]*shell.Shell, 0),
		gateways: gateways,
		cache:    ccache.New(ccache.Configure().MaxSize(30000)),
	}

	for i := range opts {
		opts[i](&s)
	}

	return s
}

// Resolve -
func (s Ipfs) Resolve(network, address, link string) ([]byte, error) {
	if len(s.gateways) == 0 {
		return nil, ErrEmptyIPFSGatewayList
	}

	hash, err := helpers.IPFSHash(link)
	if err != nil {
		return nil, err
	}

	for _, sh := range s.pinning {
		_ = sh.Pin(hash)
	}

	gateways := helpers.ShuffleGateways(s.gateways)
	for i := range gateways {
		url := helpers.IPFSLink(gateways[i], hash)
		data, err := s.cache.Fetch(hash, time.Hour, func() (interface{}, error) {
			return s.Http.Resolve(network, address, url)
		})
		if err == nil {
			contents := data.Value().([]byte)
			if len(s.pinning) > 0 {
				s.pinContents(contents)
			}
			return contents, nil
		}
	}

	return nil, ErrNoIPFSResponse
}

// Is -
func (s Ipfs) Is(link string) bool {
	return strings.HasPrefix(link, prefixIpfs)
}

func (s Ipfs) pinContents(data []byte) {
	hash := helpers.FindAllIPFSLinks(data)

	for i := range hash {
		for _, sh := range s.pinning {
			_ = sh.Pin(hash[i])
		}
	}
}
