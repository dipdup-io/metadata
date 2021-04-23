package resolver

import (
	"fmt"
	"strings"
	"time"

	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/ipfs/go-cid"
	"github.com/karlseguin/ccache"
	"gorm.io/gorm"
)

const (
	prefixIpfs = "ipfs://"
)

// Ipfs -
type Ipfs struct {
	Http

	cache    *ccache.Cache
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

// NewIPFS -
func NewIPFS(gateways []string, opts ...IpfsOption) Ipfs {
	s := Ipfs{
		Http:     NewHttp(),
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

	hash := strings.TrimPrefix(link, prefixIpfs)
	if _, err := cid.Decode(hash); err != nil {
		return nil, err
	}

	for i := range s.gateways {
		url := fmt.Sprintf("%s/ipfs/%s", s.gateways[i], hash)

		data, err := s.cache.Fetch(hash, time.Hour, func() (interface{}, error) {
			return s.Http.Resolve(network, address, url)
		})
		if err == nil {
			return data.Value().([]byte), nil
		}

	}

	return nil, ErrNoIPFSResponse
}

// Is -
func (s Ipfs) Is(link string) bool {
	return strings.HasPrefix(link, prefixIpfs)
}
