package registry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

func aliasEntry(name string, aliases ...string) registry.RegistryEntry {
	return registry.RegistryEntry{Name: name, Repository: "rshade/finfocus-plugin-" + name, Aliases: aliases}
}

func TestValidateRegistryEntry_Aliases(t *testing.T) {
	t.Parallel()

	require.NoError(t, registry.ValidateRegistryEntry(aliasEntry("aws-public", "finfocus-plugin-aws-public")))

	err := registry.ValidateRegistryEntry(aliasEntry("aws-public", "Not_Valid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"aws-public" has invalid alias "Not_Valid"`)

	err = registry.ValidateRegistryEntry(aliasEntry("aws-public", "aws-public"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `alias "aws-public" repeats its own name`)
}

func TestValidateCatalogNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []registry.RegistryEntry
		wantErr string
	}{
		{
			name: "distinct names and aliases",
			entries: []registry.RegistryEntry{
				aliasEntry("aws-public", "finfocus-plugin-aws-public"),
				aliasEntry("cloudzero", "finfocus-plugin-cloudzero"),
			},
		},
		{
			name: "alias equals another entry's name",
			entries: []registry.RegistryEntry{
				aliasEntry("aws-public", "aws-ce"),
				aliasEntry("aws-ce"),
			},
			wantErr: `"aws-ce" is a name of both "aws-ce" and "aws-public"`,
		},
		{
			name: "two entries share an alias",
			entries: []registry.RegistryEntry{
				aliasEntry("aws-public", "old-name"),
				aliasEntry("aws-ce", "old-name"),
			},
			wantErr: `"old-name" is a name of both "aws-ce" and "aws-public"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := registry.ValidateCatalogNames(tc.entries)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRegistryJSONAliases(t *testing.T) {
	t.Parallel()

	reg, err := registry.GetEmbeddedRegistry()
	require.NoError(t, err)
	assert.Equal(t, []string{"finfocus-plugin-aws-public"}, reg.Plugins["aws-public"].Aliases)
	assert.Equal(t, []string{"finfocus-plugin-cloudzero"}, reg.Plugins["cloudzero"].Aliases)

	entries := make([]registry.RegistryEntry, 0, len(reg.Plugins))
	for _, entry := range reg.Plugins {
		require.NoError(t, registry.ValidateRegistryEntry(entry))
		entries = append(entries, entry)
	}
	require.NoError(t, registry.ValidateCatalogNames(entries))
}

func TestCatalogAliases(t *testing.T) {
	t.Setenv("FINFOCUS_CATALOG_FILE", "")

	assert.Equal(t, []string{"finfocus-plugin-aws-public"}, registry.CatalogAliases("aws-public"))
	assert.Empty(t, registry.CatalogAliases("not-in-the-catalog"))

	path := filepath.Join(t.TempDir(), "catalog.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"schema_version":"1","plugins":{"acme":{
		"name":"acme","repository":"rshade/finfocus-plugin-acme","aliases":["acme-legacy"]}}}`), 0o600))
	t.Setenv("FINFOCUS_CATALOG_FILE", path)

	assert.Equal(t, []string{"acme-legacy"}, registry.CatalogAliases("acme"))
}
