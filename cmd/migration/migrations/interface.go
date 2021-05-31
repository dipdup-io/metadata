package migrations

import "github.com/dipdup-net/metadata/cmd/metadata/config"

// Migration -
type Migration interface {
	Do(cfg config.Config) error
	Name() string
}
