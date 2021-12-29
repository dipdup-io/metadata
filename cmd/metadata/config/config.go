package config

import (
	"net/url"

	"github.com/dipdup-net/go-lib/config"
	"github.com/pkg/errors"
)

// Config -
type Config struct {
	config.Config `yaml:",inline"`
	Metadata      Metadata `yaml:"metadata"`
}

// Substitute -
func (c *Config) Substitute() error {
	for _, indexer := range c.Metadata.Indexers {
		if err := substituteContracts(c, &indexer.Filters); err != nil {
			return err
		}
		if err := substituteDataSources(c, &indexer.DataSource); err != nil {
			return err
		}
	}
	return nil
}

func substituteContracts(c *Config, filters *Filters) error {
	for i, address := range filters.Accounts {
		contract, ok := c.Contracts[address]
		if !ok {
			continue
		}
		filters.Accounts[i] = contract.Address
	}
	return nil
}

func substituteDataSources(c *Config, dataSource *MetadataDataSource) error {
	if source, ok := c.DataSources[dataSource.Tzkt]; ok {
		if source.Kind != "tzkt" {
			return errors.Errorf("Invalid tzkt data source kind. Expected `tzkt`, got `%s`", source.Kind)
		}
		dataSource.Tzkt = source.URL
	}
	return nil
}

// Load -
func Load(filename string) (cfg Config, err error) {
	err = config.Parse(filename, &cfg)
	return
}

// Metadata -
type Metadata struct {
	Settings Settings            `yaml:"settings"`
	Indexers map[string]*Indexer `yaml:"indexers" validate:"min=1"`
}

// indexers -
type Indexer struct {
	Filters    Filters            `yaml:"filters"`
	DataSource MetadataDataSource `yaml:"datasources"`
}

// Filters -
type Filters struct {
	Accounts []string `yaml:"accounts" validate:"max=50"`
}

// MetadataDataSource -
type MetadataDataSource struct {
	Tzkt string `yaml:"tzkt" validate:"url"`
}

// Validate -
func (cfg MetadataDataSource) Validate() error {
	if _, err := url.ParseRequestURI(cfg.Tzkt); err != nil {
		return errors.Wrapf(err, "Invalid TzKT url %s", cfg.Tzkt)
	}
	return nil
}

// Settings -
type Settings struct {
	IPFSGateways         []string  `yaml:"ipfs_gateways" validate:"min=1,dive,url"`
	IPFSPinning          []string  `yaml:"ipfs_pinning"`
	IPFSTimeout          uint64    `yaml:"ipfs_timeout" validate:"min=1"`
	HTTPTimeout          uint64    `yaml:"http_timeout" validate:"min=1"`
	MaxRetryCountOnError uint64    `yaml:"max_retry_count_on_error" validate:"min=1"`
	Thumbnail            Thumbnail `yaml:"thumbnail"`
	AWS                  AWS       `yaml:"aws"`
	MaxCPU               uint64    `yaml:"max_cpu,omitempty" validate:"omitempty,min=1"`
}

// AWS -
type AWS struct {
	Endpoint   string `yaml:"endpoint" validate:"omitempty,url"`
	BucketName string `yaml:"bucket_name" validate:"omitempty"`
	Region     string `yaml:"region" validate:"omitempty"`
	AccessKey  string `yaml:"access_key_id" validate:"omitempty"`
	Secret     string `yaml:"secret_access_key" validate:"omitempty"`
}

// Thumbnail -
type Thumbnail struct {
	MaxFileSize int64 `yaml:"max_file_size_mb" validate:"min=1"`
	Size        int   `yaml:"size" validate:"min=1"`
	Workers     int   `yaml:"workers" validate:"min=1"`
	Timeout     int   `yaml:"timeout" validate:"min=1"`
}
