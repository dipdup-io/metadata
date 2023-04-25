package main

import (
	"github.com/dipdup-net/go-lib/hasura"
	"os"
	"testing"
)

func TestIntegration_HasuraMetadata(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("Skipping testing in CI environment")
	}

	configPath := "../../build/dipdup.yml"                      // todo: Fix paths
	expectedMetadataPath := "../../build/expected_metadata.yml" // todo: Fix paths

	hasura.TestExpectedMetadataWithActual(t, configPath, expectedMetadataPath)
}
