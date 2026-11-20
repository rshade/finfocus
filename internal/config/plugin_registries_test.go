package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestPluginRegistryLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", dir)
	ctx := t.Context()
	path := filepath.Join(dir, "config.hujson")
	require.NoError(t, os.WriteFile(path, []byte("{\n\t// keep this comment\n\t\"unrelated\": 1,\n}\n"), 0o600))

	source := config.PluginRegistrySource{Name: "partner", URL: "https://plugins.example.com/index.json"}
	require.NoError(t, config.AddPluginRegistry(ctx, source))
	require.ErrorIs(t, config.AddPluginRegistry(ctx, source), config.ErrPluginRegistryExists)
	sources, err := config.GlobalPluginRegistries(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "community", sources[0].Trust)
	source.Trust = "community"
	assert.Equal(t, source, sources[0])

	confirmed, err := config.PluginRegistryConfirmed(ctx, source)
	require.NoError(t, err)
	assert.False(t, confirmed)
	require.NoError(t, config.ConfirmPluginRegistry(ctx, source))
	confirmed, err = config.PluginRegistryConfirmed(ctx, source)
	require.NoError(t, err)
	assert.True(t, confirmed)

	moved := source
	moved.URL = "https://other.example.com/index.json"
	require.ErrorIs(t, config.ConfirmPluginRegistry(ctx, moved), config.ErrPluginRegistryChanged)
	confirmed, err = config.PluginRegistryConfirmed(ctx, moved)
	require.ErrorIs(t, err, config.ErrPluginRegistryChanged)
	assert.False(t, confirmed)
	require.NoError(t, config.RemovePluginRegistry(ctx, source.Name))
	require.ErrorIs(t, config.RemovePluginRegistry(ctx, source.Name), config.ErrPluginRegistryNotFound)
	require.NoError(t, config.AddPluginRegistry(ctx, source))
	confirmed, err = config.PluginRegistryConfirmed(ctx, source)
	require.NoError(t, err)
	assert.False(t, confirmed)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "// keep this comment")
	assert.Contains(t, string(data), `"unrelated": 1`)
}

func TestPluginRegistrySourceValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		source  config.PluginRegistrySource
		message string
	}{
		{"valid", config.PluginRegistrySource{Name: "partner", URL: "https://example.com/index.json"}, ""},
		{"pinned", config.PluginRegistrySource{
			Name: "partner-1", URL: "https://example.com/index.json", Trust: "experimental", SHA256: strings.Repeat("a", 64),
		}, ""},
		{"name", config.PluginRegistrySource{Name: "../other", URL: "https://example.com"}, "name"},
		{"HTTP", config.PluginRegistrySource{Name: "partner", URL: "http://example.com"}, "HTTPS"},
		{"credentials", config.PluginRegistrySource{Name: "partner", URL: "https://user:secret@example.com"}, "HTTPS"},
		{"query", config.PluginRegistrySource{Name: "partner", URL: "https://example.com?token=secret"}, "HTTPS"},
		{"fragment", config.PluginRegistrySource{Name: "partner", URL: "https://example.com#secret"}, "HTTPS"},
		{"trust", config.PluginRegistrySource{Name: "partner", URL: "https://example.com", Trust: "official"}, "trust"},
		{"digest", config.PluginRegistrySource{Name: "partner", URL: "https://example.com", SHA256: "secret"}, "sha256"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := config.ValidatePluginRegistrySource(tt.source)
			if tt.message == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.message)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestPluginRegistryProjectOverlayRejected(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		data string
		key  string
	}{
		{"config.hujson", `{"plugin_registries": []}`, "plugin_registries"},
		{"config.yaml", "plugin_registries: []\n", "plugin_registries"},
		{"config.hujson", `{"PLUGIN_REGISTRIES": []}`, "PLUGIN_REGISTRIES"},
		{"config.hujson", `{"plugin_registry_confirmations": null}`, "plugin_registry_confirmations"},
		{"config.yaml", "plugin_registry_confirmations: {}\n", "plugin_registry_confirmations"},
	} {
		t.Run(tt.key+"/"+tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), tt.name)
			require.NoError(t, os.WriteFile(path, []byte(tt.data), 0o600))
			cfg := &config.Config{
				PluginRegistries: []config.PluginRegistrySource{{Name: "global", URL: "https://example.com"}},
			}
			require.ErrorIs(t, config.ShallowMergeYAML(cfg, path), config.ErrPluginRegistriesGlobalOnly)
			require.Len(t, cfg.PluginRegistries, 1)
			assert.Equal(t, "global", cfg.PluginRegistries[0].Name)
			result, _ := config.ValidateProjectConfigSource(path, []byte(tt.data))
			require.False(t, result.Valid)
			require.NotEmpty(t, result.Errors)
			assert.Equal(t, tt.key, result.Errors[len(result.Errors)-1].Path)
			assert.Contains(t, result.Errors[len(result.Errors)-1].Message, "global config")
		})
	}
}

func TestPluginRegistryAmbiguousGlobalKeysRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", dir)
	path := filepath.Join(dir, "config.hujson")
	for _, raw := range []string{
		`{"plugin_registries":[],"plugin_registries":[]}`,
		`{"PLUGIN_REGISTRIES":[]}`,
		`{"plugin_registry_confirmations":{},"plugin_registry_confirmations":{}}`,
	} {
		require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))
		_, err := config.GlobalPluginRegistries(t.Context())
		require.Error(t, err)
		result, _ := config.ValidateConfigSource(path, []byte(raw))
		assert.False(t, result.Valid, "config validation accepted %s", raw)
	}
}

func TestPluginRegistryWritesPreserveConcurrentAdds(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	ctx := t.Context()
	names := []string{"one", "two", "three"}
	results := make(chan error, len(names))
	for _, name := range names {
		go func() {
			results <- config.AddPluginRegistry(ctx, config.PluginRegistrySource{
				Name: name,
				URL:  "https://example.com/index.json",
			})
		}()
	}
	for range names {
		require.NoError(t, <-results)
	}
	sources, err := config.GlobalPluginRegistries(ctx)
	require.NoError(t, err)
	var added []string
	for _, source := range sources {
		added = append(added, source.Name)
	}
	assert.ElementsMatch(t, names, added)
}

func TestPluginRegistryTrustInvalidatesAfterConfigEdit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", dir)
	source := config.PluginRegistrySource{Name: "partner", URL: "https://example.com/index.json"}
	require.NoError(t, config.AddPluginRegistry(t.Context(), source))
	require.NoError(t, config.ConfirmPluginRegistry(t.Context(), source))

	path := filepath.Join(dir, "config.hujson")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	edited := strings.Replace(string(raw), "https://example.com", "https://new.example.com", 1)
	require.NoError(t, os.WriteFile(path, []byte(edited), 0o600))
	sources, err := config.GlobalPluginRegistries(t.Context())
	require.NoError(t, err)
	require.Len(t, sources, 1)
	_, err = config.PluginRegistryConfirmed(t.Context(), source)
	require.ErrorIs(t, err, config.ErrPluginRegistryChanged)
	approved, err := config.PluginRegistryConfirmed(t.Context(), sources[0])
	require.NoError(t, err)
	assert.False(t, approved)
}

func TestPluginRegistryWorkingDirectoryFallbackIsUntrusted(t *testing.T) {
	for _, key := range []string{"FINFOCUS_HOME", "PULUMI_HOME", "HOME", "USERPROFILE", "home"} {
		t.Setenv(key, "")
	}
	t.Chdir(t.TempDir())
	dir := filepath.Join(".", ".finfocus")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	raw := `{"plugin_registries":[{"name":"untrusted","url":"https://example.com"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.hujson"), []byte(raw), 0o600))

	require.True(t, config.UsesWorkingDirFallback())
	sources, err := config.GlobalPluginRegistries(t.Context())
	require.NoError(t, err)
	assert.Empty(t, sources)
	assert.Empty(t, config.New().PluginRegistries)
	require.ErrorIs(t, config.AddPluginRegistry(t.Context(),
		config.PluginRegistrySource{Name: "example", URL: "https://example.com"}), config.ErrPluginRegistriesGlobalOnly)
}
