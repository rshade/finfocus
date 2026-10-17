package registry

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

//go:embed registry.json
var registryData []byte

// EmbeddedRegistry represents the embedded plugin registry catalog.
type EmbeddedRegistry struct {
	SchemaVersion string                   `json:"schema_version"`
	Plugins       map[string]RegistryEntry `json:"plugins"`
}

//nolint:gochecknoglobals // sync.Once pattern for lazy loading
var (
	embeddedRegistry     *EmbeddedRegistry
	embeddedRegistryOnce sync.Once
	errEmbeddedRegistry  error
)

// GetEmbeddedRegistry returns the parsed embedded registry catalog.
// It initializes and parses the embedded registry data on first call in a thread-safe manner.
// GetEmbeddedRegistry parses and returns the embedded plugin registry, initializing it once on first use.
// It returns the parsed *EmbeddedRegistry and a non-nil error if parsing the embedded registry failed; the initialization is performed exactly once and is safe for concurrent use.
func GetEmbeddedRegistry() (*EmbeddedRegistry, error) {
	embeddedRegistryOnce.Do(func() {
		embeddedRegistry = &EmbeddedRegistry{}
		errEmbeddedRegistry = json.Unmarshal(registryData, embeddedRegistry)
		if errEmbeddedRegistry != nil {
			errEmbeddedRegistry = fmt.Errorf(
				"failed to parse embedded registry: %w",
				errEmbeddedRegistry,
			)
		}
	})
	return embeddedRegistry, errEmbeddedRegistry
}

// GetPlugin looks up a plugin by name in the embedded registry and returns its RegistryEntry.
// The name parameter is the plugin identifier to look up.
// It returns a pointer to the RegistryEntry if the plugin exists.
// If parsing the embedded registry fails, an error from GetEmbeddedRegistry is returned.
// If the plugin is not present in the registry, an error indicating the plugin was not found is returned.
func GetPlugin(name string) (*RegistryEntry, error) {
	extras, err := overlayEntries()
	if err != nil {
		return nil, err
	}
	for i := range extras {
		if extras[i].Name == name {
			entry := extras[i]
			return &entry, nil
		}
	}
	reg, err := GetEmbeddedRegistry()
	if err != nil {
		return nil, err
	}
	entry, ok := reg.Plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found in registry", name)
	}
	return &entry, nil
}

// ListPluginsFromRegistry returns the names of all plugins contained in the embedded registry.
// It loads the embedded registry and collects each plugin key into a slice.
// The returned error is non-nil if the embedded registry cannot be loaded or parsed.
func ListPluginsFromRegistry() ([]string, error) {
	reg, err := GetEmbeddedRegistry()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(reg.Plugins))
	for name := range reg.Plugins {
		names = append(names, name)
	}
	return names, nil
}

// GetAllPluginEntries retrieves all plugin entries present in the embedded registry catalog.
// It returns a slice containing every RegistryEntry found in the embedded catalog.
// If the embedded registry cannot be loaded or parsed, an error is returned.
func GetAllPluginEntries() ([]RegistryEntry, error) {
	reg, err := GetEmbeddedRegistry()
	if err != nil {
		return nil, err
	}
	byName := make(map[string]RegistryEntry, len(reg.Plugins))
	for _, entry := range reg.Plugins {
		byName[entry.Name] = entry
	}
	extras, err := overlayEntries()
	if err != nil {
		return nil, err
	}
	for _, entry := range extras {
		byName[entry.Name] = entry
	}
	plugins := make([]RegistryEntry, 0, len(byName))
	for _, entry := range byName {
		plugins = append(plugins, entry)
	}
	return plugins, nil
}

// overlayEntries reads FINFOCUS_CATALOG_FILE when it is set. The file uses the
// embedded registry document shape. An unset variable adds nothing. The
// dashboard tests use it to publish a stand-in plugin; it is not a supported
// custom catalog.
func overlayEntries() ([]RegistryEntry, error) {
	path := os.Getenv("FINFOCUS_CATALOG_FILE")
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G703: operator-set FINFOCUS_CATALOG_FILE seam, not request input
	if err != nil {
		return nil, fmt.Errorf("reading catalog overlay: %w", err)
	}
	var doc EmbeddedRegistry
	if unmarshalErr := json.Unmarshal(data, &doc); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing catalog overlay: %w", unmarshalErr)
	}
	extras := make([]RegistryEntry, 0, len(doc.Plugins))
	for _, entry := range doc.Plugins {
		extras = append(extras, entry)
	}
	return extras, nil
}
