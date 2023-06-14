package config

import (
	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/metadata/internal/ipfs"
	"github.com/pkg/errors"
)

// Config -
type Config struct {
	config.Config `yaml:",inline"`
	Metadata      Metadata `yaml:"metadata"`
}

// Substitute -
func (c *Config) Substitute() error {
	if err := c.Config.Substitute(); err != nil {
		return err
	}

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
		contract, ok := c.Contracts[address.Name()]
		if !ok {
			continue
		}
		filters.Accounts[i].SetStruct(contract)
	}
	return nil
}

func substituteDataSources(c *Config, dataSource *MetadataDataSource) error {
	if source, ok := c.DataSources[dataSource.Tzkt.Name()]; ok {
		if source.Kind != "tzkt" {
			return errors.Errorf("Invalid tzkt data source kind. Expected `tzkt`, got `%s`", source.Kind)
		}
		dataSource.Tzkt.SetStruct(source)
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
	Accounts   []config.Alias[config.Contract] `yaml:"accounts" validate:"max=50"`
	FirstLevel uint64                          `yaml:"first_level" validate:"min=0"`
	LastLevel  uint64                          `yaml:"last_level" validate:"min=0"`
}

// Addresses -
func (f Filters) Addresses() []string {
	addresses := make([]string, 0)
	for i := range f.Accounts {
		addresses = append(addresses, f.Accounts[i].Struct().Address)
	}
	return addresses
}

// MetadataDataSource -
type MetadataDataSource struct {
	Tzkt config.Alias[config.DataSource] `yaml:"tzkt" validate:"url"`
}

// Settings -
type Settings struct {
	IPFS                   IPFS      `yaml:"ipfs"`
	HTTPTimeout            uint64    `yaml:"http_timeout" validate:"min=1"`
	MaxRetryCountOnError   int       `yaml:"max_retry_count_on_error" validate:"min=1"`
	ContractServiceWorkers int       `yaml:"contract_service_workers" validate:"min=1"`
	TokenServiceWorkers    int       `yaml:"token_service_workers" validate:"min=1"`
	Thumbnail              Thumbnail `yaml:"thumbnail"`
	AWS                    AWS       `yaml:"aws"`
	MaxCPU                 int       `yaml:"max_cpu,omitempty" validate:"omitempty,min=1"`
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

// IPFS -
type IPFS struct {
	Dir       string          `yaml:"dir"`
	Bootstrap []string        `yaml:"bootstrap"`
	Gateways  []string        `yaml:"gateways" validate:"min=1,dive,url"`
	Blacklist []string        `yaml:"blacklist"`
	Timeout   uint64          `yaml:"timeout" validate:"min=1"`
	Fallback  string          `yaml:"fallback" validate:"url"`
	Delay     int             `yaml:"delay" validate:"min=1"`
	Providers []ipfs.Provider `yaml:"providers" validate:"omitempty"`
}
