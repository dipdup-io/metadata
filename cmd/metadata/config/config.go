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

// Validate -
func (c *Config) Validate() error {
	for network, mempool := range c.Metadata.Indexers {
		if err := mempool.DataSource.Validate(); err != nil {
			return errors.Wrap(err, network)
		}
		if err := mempool.Filters.Validate(); err != nil {
			return errors.Wrap(err, network)
		}
	}
	return c.Metadata.Settings.Validate()
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
	Indexers map[string]*Indexer `yaml:"indexers"`
}

// indexers -
type Indexer struct {
	Filters    Filters            `yaml:"filters"`
	DataSource MetadataDataSource `yaml:"datasources"`
}

// Filters -
type Filters struct {
	Accounts []string `yaml:"accounts"`
}

// Validate -
func (cfg Filters) Validate() error {
	if len(cfg.Accounts) > tzktMaxSubscriptions {
		return errors.Errorf("Maximum accounts in config is %d. You added %d accounts", tzktMaxSubscriptions, len(cfg.Accounts))
	}

	return nil
}

// MetadataDataSource -
type MetadataDataSource struct {
	Tzkt string `yaml:"tzkt"`
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
	IPFSGateways         []string `yaml:"ipfs_gateways"`
	IPFSPinning          []string `yaml:"ipfs_pinning"`
	IPFSTimeout          uint64   `yaml:"ipfs_timeout"`
	HTTPTimeout          uint64   `yaml:"http_timeout"`
	MaxRetryCountOnError uint64   `yaml:"max_retry_count_on_error"`
	Index                []string `yaml:"index"`
	AWS                  AWS      `yaml:"aws"`
	MaxCPU               uint64   `yaml:"max_cpu,omitempty"`
}

// Validate -
func (cfg *Settings) Validate() error {
	if cfg.IPFSTimeout == 0 {
		cfg.IPFSTimeout = 10
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 10
	}
	if cfg.MaxCPU == 0 {
		cfg.MaxCPU = 4
	}
	if cfg.MaxRetryCountOnError == 0 {
		cfg.MaxRetryCountOnError = 3
	}
	if len(cfg.Index) == 0 {
		cfg.Index = []string{"token_metadata", "metadata"}
	}

	if len(cfg.IPFSGateways) == 0 {
		return errors.New("Empty IPFS gateway list")
	}

	for i := range cfg.IPFSGateways {
		if _, err := url.ParseRequestURI(cfg.IPFSGateways[i]); err != nil {
			return errors.Wrapf(err, "Invalid IPFS gateway url %s", cfg.IPFSGateways[i])
		}
	}

	return nil
}

// AWS -
type AWS struct {
	BucketName string `yaml:"bucket_name"`
	Region     string `yaml:"region"`
	AccessKey  string `yaml:"access_key_id"`
	Secret     string `yaml:"secret_access_key"`
}
