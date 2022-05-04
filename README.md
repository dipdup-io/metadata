# Metadata indexer

[![Tests](https://github.com/dipdup-net/metadata/workflows/Tests/badge.svg?)](https://github.com/dipdup-net/metadata/actions?query=workflow%3ATests)
[![Docker images](https://github.com/dipdup-net/metadata/workflows/Release/badge.svg?)](https://hub.docker.com/r/dipdup/metadata)
[![Made With](https://img.shields.io/badge/made%20with-dipdup-blue.svg?)](https://dipdup.net)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Selective Tezos metadata indexer based on DipDup framework.  
Supported features:
- [TZIP-16](https://gitlab.com/tzip/tzip/-/blob/master/proposals/tzip-16/tzip-16.md) contract metadata
- [TZIP-12](https://gitlab.com/tezos/tzip/-/blob/master/proposals/tzip-12/tzip-12.md#token-metadata) token metadata
- IPFS file pinning
- Token thumbnails generating (and uploading to AWS)
- Elasicsearch mode

## Configuration

Fully compatible with DipDup YAML configuration file format.
Metadata indexer reuses `datasources`, `contracts`, `database`, `hasura` sections, and reads its own settings from `metadata` top-level section.

Read more [in the docs](https://docs.dipdup.net/plugins/metadata).

## GQL client

### Installation

```
npm i @dipdup/metadata
```

### Usage

First of all you need to create an instance of metadata client:
```js
import { createClient } from '@dipdup/metadata'

const client = createClient({
    url: 'http://metadata.dipdup.net/v1/graphql',
    subscription: {
        url: "wss://metadata.dipdup.net/v1/graphql"
    }
});
```

#### Query

```js
import { everything } from '@dipdup/metadata'

client.chain.query
    .token_metadata({
        where: { 
            contract: { _eq: 'KT1RJ6PbjHpwc3M5rw5s2Nbmefwbuwbdxton' },
            token_id: { _eq: 100000 }
        }
    })
    .get({ ...everything })
    .then(res => console.log)
```

#### Subscription (live query)

```js
const { unsubscribe } = client.chain.subscription
    .token_metadata({
        where: { 
            contract: { _eq: 'KT1RJ6PbjHpwc3M5rw5s2Nbmefwbuwbdxton' },
            created_at: { _gt: '2021-07-06T00:00:00' }
        }
    })
    .get({ ...everything })
    .subscribe({
        next: res => console.log
    })
```

## Maintenance

### Refetch recent metadata

This is not a permanent solution, rather an ad-hoc command to fix recent fetch errors. Adjust the data accordingly or remove time condition.
```sql
UPDATE token_metadata
SET retry_count=0, status=1
WHERE created_at > 1646082000 AND metadata ISNULL
```
