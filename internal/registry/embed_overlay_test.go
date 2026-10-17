package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlugin_CatalogOverlay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	body := []byte(`{
		"schema_version": "1.0.0",
		"plugins": {
			"recorder": {
				"name": "recorder",
				"description": "records requests",
				"repository": "rshade/finfocus",
				"security_level": "official",
				"supported_providers": ["test"],
				"capabilities": ["projected"]
			}
		}
	}`)
	require.NoError(t, os.WriteFile(path, body, 0o644))
	t.Setenv("FINFOCUS_CATALOG_FILE", path)

	entry, err := GetPlugin("recorder")
	require.NoError(t, err)
	assert.Equal(t, "rshade/finfocus", entry.Repository)
	assert.Equal(t, "official", entry.SecurityLevel)

	all, err := GetAllPluginEntries()
	require.NoError(t, err)
	found := false
	for _, item := range all {
		if item.Name == "recorder" {
			found = true
		}
	}
	assert.True(t, found)

	embedded, err := GetPlugin("aws-public")
	require.NoError(t, err)
	assert.NotEmpty(t, embedded.Repository)
}
