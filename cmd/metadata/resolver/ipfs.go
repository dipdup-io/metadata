package resolver

import (
	"strings"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/karlseguin/ccache"
	"gorm.io/gorm"

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

// Init -
func (s Ipfs) Init(db *gorm.DB) error {
	pageSize := 1000

	var offset int
	var end bool
	for !end {
		tokens, err := models.GetTokenMetadata(db, models.StatusApplied, pageSize, offset)
		if err != nil {
			return err
		}

		for i := range tokens {
			if !strings.HasPrefix(tokens[i].Link, prefixIpfs) {
				continue
			}

			hash := strings.TrimPrefix(tokens[i].Link, prefixIpfs)
			s.cache.Set(hash, []byte(tokens[i].Metadata), time.Hour)
		}

		end = len(tokens) < pageSize
		offset += pageSize
	}

	offset = 0
	end = false
	for !end {
		contracts, err := models.GetContractMetadata(db, models.StatusApplied, pageSize, offset)
		if err != nil {
			return err
		}

		for i := range contracts {
			if !strings.HasPrefix(contracts[i].Link, prefixIpfs) {
				continue
			}

			hash := strings.TrimPrefix(contracts[i].Link, prefixIpfs)
			s.cache.Set(hash, []byte(contracts[i].Metadata), time.Hour)
		}

		end = len(contracts) < pageSize
		offset += pageSize
	}
	return nil
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

	for i := range s.gateways {
		url := helpers.IPFSLink(s.gateways[i], hash)
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
