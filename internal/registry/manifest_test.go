package registry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

// TestLoadManifest_ValidManifest tests loading a valid manifest file.
func TestLoadManifest_ValidManifest(t *testing.T) {
	manifestData := registry.Manifest{
		Name:        "aws-plugin",
		Version:     "v1.0.0",
		Description: "AWS cost calculation plugin",
		Author:      "FinFocus Team",
		Providers:   []string{"aws"},
		Metadata: map[string]string{
			"supportedRegions": "us-east-1,us-west-2",
		},
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, "aws-plugin", manifest.Name)
	assert.Equal(t, "v1.0.0", manifest.Version)
	assert.Equal(t, "AWS cost calculation plugin", manifest.Description)
	assert.Equal(t, "FinFocus Team", manifest.Author)
	assert.Equal(t, []string{"aws"}, manifest.Providers)
	assert.Equal(t, "us-east-1,us-west-2", manifest.Metadata["supportedRegions"])
}

// TestLoadManifest_MinimalManifest tests loading a manifest with only required fields.
func TestLoadManifest_MinimalManifest(t *testing.T) {
	manifestData := registry.Manifest{
		Name:    "minimal-plugin",
		Version: "v0.1.0",
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.Equal(t, "minimal-plugin", manifest.Name)
	assert.Equal(t, "v0.1.0", manifest.Version)
	assert.Empty(t, manifest.Description)
	assert.Empty(t, manifest.Author)
	assert.Empty(t, manifest.Providers)
	assert.Nil(t, manifest.Metadata)
}

// TestLoadManifest_MultipleProviders tests manifest with multiple providers.
func TestLoadManifest_MultipleProviders(t *testing.T) {
	manifestData := registry.Manifest{
		Name:      "multi-cloud-plugin",
		Version:   "v2.0.0",
		Providers: []string{"aws", "azure", "gcp"},
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.Len(t, manifest.Providers, 3)
	assert.Contains(t, manifest.Providers, "aws")
	assert.Contains(t, manifest.Providers, "azure")
	assert.Contains(t, manifest.Providers, "gcp")
}

// TestLoadManifest_WithMetadata tests manifest with custom metadata.
func TestLoadManifest_WithMetadata(t *testing.T) {
	manifestData := registry.Manifest{
		Name:    "metadata-plugin",
		Version: "v1.5.0",
		Metadata: map[string]string{
			"apiEndpoint":  "https://api.example.com",
			"refreshRate":  "300",
			"cacheEnabled": "true",
		},
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.Len(t, manifest.Metadata, 3)
	assert.Equal(t, "https://api.example.com", manifest.Metadata["apiEndpoint"])
	assert.Equal(t, "300", manifest.Metadata["refreshRate"])
	assert.Equal(t, "true", manifest.Metadata["cacheEnabled"])
}

// TestLoadManifest_NonExistentFile tests error handling for missing file.
func TestLoadManifest_NonExistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	manifest, err := registry.LoadManifest(path)

	assert.Error(t, err)
	assert.Nil(t, manifest)
}

// TestLoadManifest_InvalidJSON tests error handling for malformed JSON.
func TestLoadManifest_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(path, []byte("{ invalid json }"), 0644)
	require.NoError(t, err)

	manifest, err := registry.LoadManifest(path)

	assert.Error(t, err)
	assert.Nil(t, manifest)
}

// TestLoadManifest_EmptyFile tests handling of empty file.
func TestLoadManifest_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "empty.json")

	// Write empty file
	err := os.WriteFile(path, []byte(""), 0644)
	require.NoError(t, err)

	manifest, err := registry.LoadManifest(path)

	assert.Error(t, err)
	assert.Nil(t, manifest)
}

// TestLoadManifest_EmptyJSONObject tests handling of empty JSON object.
func TestLoadManifest_EmptyJSONObject(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "empty-object.json")

	// Write empty JSON object
	err := os.WriteFile(path, []byte("{}"), 0644)
	require.NoError(t, err)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.NotNil(t, manifest)
	assert.Empty(t, manifest.Name)
	assert.Empty(t, manifest.Version)
}

// TestLoadManifest_InvalidFieldTypes tests handling of incorrect field types.
func TestLoadManifest_InvalidFieldTypes(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "invalid-types.json")

	// Write JSON with invalid field types
	invalidJSON := `{
		"name": "test-plugin",
		"version": 123,
		"providers": "should-be-array"
	}`
	err := os.WriteFile(path, []byte(invalidJSON), 0644)
	require.NoError(t, err)

	manifest, err := registry.LoadManifest(path)

	// JSON unmarshaling should fail due to type mismatch
	assert.Error(t, err)
	assert.Nil(t, manifest)
}

// TestLoadManifest_ExtraFields tests that extra unknown fields are ignored.
func TestLoadManifest_ExtraFields(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "extra-fields.json")

	// Write JSON with extra fields not in Manifest struct
	jsonWithExtras := `{
		"name": "test-plugin",
		"version": "v1.0.0",
		"unknownField": "value",
		"anotherExtra": 123
	}`
	err := os.WriteFile(path, []byte(jsonWithExtras), 0644)
	require.NoError(t, err)

	manifest, err := registry.LoadManifest(path)

	// Should succeed - extra fields are ignored
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", manifest.Name)
	assert.Equal(t, "v1.0.0", manifest.Version)
}

// TestLoadManifest_UnicodeContent tests handling of Unicode characters.
func TestLoadManifest_UnicodeContent(t *testing.T) {
	manifestData := registry.Manifest{
		Name:        "unicode-plugin",
		Version:     "v1.0.0",
		Description: "Plugin with unicode: 日本語, emoji: 🚀",
		Author:      "Café ☕",
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.Contains(t, manifest.Description, "日本語")
	assert.Contains(t, manifest.Description, "🚀")
	assert.Contains(t, manifest.Author, "☕")
}

// TestLoadManifest_EscapedCharacters tests handling of escaped JSON characters.
func TestLoadManifest_EscapedCharacters(t *testing.T) {
	manifestData := registry.Manifest{
		Name:        "escaped-plugin",
		Version:     "v1.0.0",
		Description: "Plugin with \"quotes\" and \\ backslashes",
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.Contains(t, manifest.Description, "\"quotes\"")
	assert.Contains(t, manifest.Description, "\\")
}

// TestLoadManifest_LargeMetadata tests handling of large metadata maps.
func TestLoadManifest_LargeMetadata(t *testing.T) {
	metadata := make(map[string]string)
	for i := 0; i < 100; i++ {
		metadata[string(rune('a'+i%26))+string(rune('0'+i/26))] = string(rune('A'+i%26)) + string(rune('0'+i/26))
	}

	manifestData := registry.Manifest{
		Name:     "large-metadata-plugin",
		Version:  "v1.0.0",
		Metadata: metadata,
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.Len(t, manifest.Metadata, 100)
}

// TestLoadManifest_NilMetadata tests that nil metadata is preserved.
func TestLoadManifest_NilMetadata(t *testing.T) {
	manifestData := registry.Manifest{
		Name:     "nil-metadata-plugin",
		Version:  "v1.0.0",
		Metadata: nil,
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	assert.Nil(t, manifest.Metadata)
}

// TestLoadManifest_EmptyMetadata tests that empty metadata map is preserved.
func TestLoadManifest_EmptyMetadata(t *testing.T) {
	manifestData := registry.Manifest{
		Name:     "empty-metadata-plugin",
		Version:  "v1.0.0",
		Metadata: map[string]string{},
	}

	path := createManifestFile(t, manifestData)

	manifest, err := registry.LoadManifest(path)

	require.NoError(t, err)
	// Empty map might be nil or empty depending on JSON marshaling
	assert.True(t, manifest.Metadata == nil || len(manifest.Metadata) == 0)
}

// Helper functions

// createManifestFile creates a temporary manifest JSON file.
func createManifestFile(t *testing.T, manifest registry.Manifest) string {
	t.Helper()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "plugin.manifest.json")

	data, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)

	return path
}
