package main

import (
	"fmt"
	"strconv"

	"github.com/dipdup-net/go-lib/cmdline"
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/dipdup-net/metadata/cmd/migration/migrations"
	"github.com/pkg/errors"
)

var migrationsList = []migrations.Migration{
	&migrations.ThumbnailColumns{},
}

func main() {
	args := cmdline.Parse()
	if args.Help {
		return
	}

	cfg, err := config.Load(args.Config)
	if err != nil {
		panic(err)
	}

	m, err := chooseMigration()
	if err != nil {
		panic(err)
	}

	if err := m.Do(cfg); err != nil {
		panic(err)
	}
}

func chooseMigration() (migrations.Migration, error) {
	fmt.Println("Available migrations:")
	for i, migration := range migrationsList {
		fmt.Printf("[%d] %s\n", i, migration.Name())
	}

	fmt.Print("\nEnter migration #:")
	var input string
	fmt.Scanln(&input)

	index, err := strconv.Atoi(input)
	if err != nil {
		return nil, err
	}

	if index < 0 || index > len(migrationsList)-1 {
		return nil, errors.Errorf("Invalid # of migration: %s", input)
	}

	return migrationsList[index], nil
}
