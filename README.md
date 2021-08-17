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

Read more [in the docs](https://docs.dipdup.net/config-file-reference/plugins/metadata).