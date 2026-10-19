package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/registry"
)

func TestPluginListAvailableScorer(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_CATALOG_FILE", "")
	var output bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--available", "--output", "json"})
	require.NoError(t, cmd.Execute())

	var entries []registry.RegistryEntry
	require.NoError(t, json.Unmarshal(output.Bytes(), &entries))
	byName := make(map[string]registry.RegistryEntry, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry
	}
	require.Contains(t, byName, "scorer")
	scorer := byName["scorer"]
	assert.Equal(t, "rshade/finfocus", scorer.Repository)
	assert.Equal(t, "official", scorer.SecurityLevel)
	assert.Equal(t, []string{"*"}, scorer.SupportedProviders)
	assert.Equal(t, []string{"recommendation_scoring"}, scorer.Capabilities)
	require.Contains(t, byName, "jev", "legacy installs remain available until scorer assets are published")
	assert.NotEqual(t, byName["jev"].TagPrefix, scorer.TagPrefix)
}
