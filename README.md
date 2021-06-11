# DipDup metadata indexer

DipDup service for indexing contract and token metadata. Based on TzKT indexer. For start synchronization of DipDup state TzKT API is uising. When head of DipDup is equal TzKT head DipDup will connect to TzKT by SignalR protocol.

## Config

Base DipDup config is supported. To index metadata you should add `metadata` section in your config YAML-file. For example:

```yaml
version: 0.0.1

metadata:
  settings:
    ipfs_gateways:
      - https://cloudflare-ipfs.com
      - https://ipfs.io
      - https://dweb.link
    ipfs_timeout: 10
    http_timeout: 10
    max_retry_count_on_error: 3
  indexers:
    mainnet:
      filters:
        accounts:
          - KT1RJ6PbjHpwc3M5rw5s2Nbmefwbuwbdxton
      datasources:
          tzkt: https://api.tzkt.io

database:
  kind: sqlite
  path: metadata.db

```

### Metadata config in details

Metadata config consists of two parts: `settings` ans `indexers`.
In `settings` section you can set general properties for all indexers.

You can set following properties:

* `ipfs_gateways` - list of IPFS gateways
* `ipfs_timeout` - how long DipDup will be wait response from IPFS gateway
* `http_timeout` - how long DipDup will be wait response from HTTP servers
* `max_retry_count_on_error` - retry counts. If DipDup can't get response from IPFS or HTTP server it will try `max_retry_count_on_error` times again. 

Example of `settings` section:

```yaml
metadata:
  settings:
    ipfs_gateways:
      - https://cloudflare-ipfs.com
      - https://ipfs.io
      - https://dweb.link
    ipfs_timeout: 10
    http_timeout: 10
    max_retry_count_on_error: 3
    # any indexers here
```

`indexers` section contains specific setting for every indexer, such as: filters and datasources.

Now only `accounts` filter availiable. You can set contracts which metadata you want to index. `accounts` is list of contract`s addresses.

`datasources` contains only one field - `tzkt`. It's link to TzKT API indexer.

Example of `indexers` section:

```yaml
metadata:
    # any settings and other indexers here
    indexers:
        mainnet:
        filters:
            accounts:
            - KT1RJ6PbjHpwc3M5rw5s2Nbmefwbuwbdxton
        datasources:
            tzkt: https://api.tzkt.io
```

## Usage

Now you can build metadata indexer from source or build docker container.

To build from source you need to install golang 1.15 and run `go build .`

Command-line args are following
```
Usage of dipdup-metadata:
  -f string
        Path to YAML config file (default "config.yaml")
  -h    Show usage
```

To build docker container

```bash
CONFIG=<yaml_config> docker build -f build/Dockerfile -t dipdup-net/metadata:latest .
docker run --name metadata dipdup-net/metadata
```

## Requirements

Now postgres, mysql and sqlite are supported

## Models

Metadata creates two models: `token_metadata` and `contract_metadata`


```go
// ContractMetadata -
type ContractMetadata struct {
	gorm.Model
	Network    string `gorm:"primaryKey"`
	Contract   string `gorm:"primaryKey"`
	RetryCount int
	Link       string
	Status     Status
	Metadata   datatypes.JSON // postgres: JSONB, mysql and sqlite: JSON
}

// TokenMetadata -
type TokenMetadata struct {
  gorm.Model
	Network        string `gorm:"primaryKey"`
	Contract       string `gorm:"primaryKey"`
	TokenID        uint64 `gorm:"primaryKey"`
	Link           string
	RetryCount     int `gorm:"default:0"`
	Status         Status
	Metadata       datatypes.JSON // postgres: JSONB, mysql and sqlite: JSON
	ImageProcessed bool 
}

// Status - metadata status
type Status int

const (
	StatusNew Status = iota + 1
	StatusFailed
	StatusApplied
)
```
