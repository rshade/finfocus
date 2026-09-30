package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/registry"
)

var allowedRegistryCapabilities = map[string]bool{ //nolint:gochecknoglobals // test fixture
	"projected":              true,
	"actual":                 true,
	"cost_retrieval":         true,
	"cost_projection":        true,
	"pricing_specs":          true,
	"recommendations":        true,
	"usage_stats":            true,
	"allocation":             true,
	"recommendation_scoring": true,
}

func isAllowedRegistryCapability(c string) bool { return allowedRegistryCapabilities[c] }

// TestRegistryJSONValid ensures the embedded registry.json is always valid.
// This test prevents invalid registry entries from passing CI.
func TestRegistryJSONValid(t *testing.T) {
	t.Parallel()

	reg, err := registry.GetEmbeddedRegistry()
	if err != nil {
		t.Fatalf("Failed to load registry.json: %v", err)
	}

	// Validate schema version
	if reg.SchemaVersion == "" {
		t.Error("registry.json missing schema_version")
	}
	if reg.SchemaVersion != "1.0.0" {
		t.Errorf("unexpected schema_version: %s (expected 1.0.0)", reg.SchemaVersion)
	}

	// Must have at least one plugin
	if len(reg.Plugins) == 0 {
		t.Fatal("registry.json has no plugins")
	}

	// Validate each plugin entry
	for name, entry := range reg.Plugins {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			validateRegistryEntryComplete(t, name, entry)
		})
	}
}

func validateRegistryEntryComplete(t *testing.T, name string, entry registry.RegistryEntry) {
	t.Helper()

	// Name must match key
	if entry.Name != name {
		t.Errorf("entry name %q doesn't match key %q", entry.Name, name)
	}

	// Required fields
	if entry.Name == "" {
		t.Error("missing required field: name")
	}
	if entry.Repository == "" {
		t.Error("missing required field: repository")
	}
	if entry.Description == "" {
		t.Error("missing required field: description")
	}

	// Validate repository format (owner/repo)
	if err := registry.ValidateRegistryEntry(entry); err != nil {
		t.Errorf("invalid entry: %v", err)
	}

	// Security level must be valid if specified
	if entry.SecurityLevel != "" {
		validLevels := map[string]bool{
			"official":     true,
			"community":    true,
			"experimental": true,
		}
		if !validLevels[entry.SecurityLevel] {
			t.Errorf("invalid security_level: %s", entry.SecurityLevel)
		}
	}

	// Capabilities must be valid if specified
	for _, cap := range entry.Capabilities {
		if !isAllowedRegistryCapability(cap) {
			t.Errorf("invalid capability: %s (add to allowedRegistryCapabilities if intentional)", cap)
		}
	}

	// Supported providers should be known values
	validProviders := map[string]bool{
		"aws":        true,
		"gcp":        true,
		"azure":      true,
		"kubernetes": true,
		"*":          true,
	}
	for _, provider := range entry.SupportedProviders {
		if !validProviders[provider] {
			t.Errorf("unknown provider: %s (add to validProviders if intentional)", provider)
		}
	}

	// Min spec version should be valid semver if specified
	if entry.MinSpecVersion != "" && !registry.IsValidVersion(entry.MinSpecVersion) {
		t.Errorf("invalid min_spec_version: %s", entry.MinSpecVersion)
	}
}

// TestRegistryCapabilities_AllowUsageAndAllocation verifies usage_stats and allocation are allowed capabilities.
func TestRegistryCapabilities_AllowUsageAndAllocation(t *testing.T) {
	t.Parallel()

	for _, c := range []string{"usage_stats", "allocation"} {
		assert.True(t, isAllowedRegistryCapability(c), c)
	}
	assert.False(t, isAllowedRegistryCapability("teleport"))
}

// TestRegistryJSONPluginNames ensures plugin names follow conventions.
func TestRegistryJSONPluginNames(t *testing.T) {
	t.Parallel()

	reg, err := registry.GetEmbeddedRegistry()
	if err != nil {
		t.Fatalf("Failed to load registry.json: %v", err)
	}

	for name := range reg.Plugins {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Names should be lowercase
			for _, c := range name {
				if c >= 'A' && c <= 'Z' {
					t.Errorf("plugin name %q contains uppercase characters", name)
					break
				}
			}

			// Names should not be empty
			if name == "" {
				t.Error("empty plugin name")
			}

			// Names should not contain spaces
			for _, c := range name {
				if c == ' ' {
					t.Errorf("plugin name %q contains spaces", name)
					break
				}
			}
		})
	}
}

// TestRegistryJSONNoDuplicates ensures no duplicate entries exist. Plugins
// released from one monorepo share a repository, so the identity of an entry is
// its repository together with its tag prefix.
func TestRegistryJSONNoDuplicates(t *testing.T) {
	t.Parallel()

	reg, err := registry.GetEmbeddedRegistry()
	if err != nil {
		t.Fatalf("Failed to load registry.json: %v", err)
	}

	sources := make(map[string]string)
	for name, entry := range reg.Plugins {
		key := entry.Repository + "|" + entry.TagPrefix
		if existing, ok := sources[key]; ok {
			t.Errorf(
				"duplicate repository %s with tag prefix %q: used by both %q and %q",
				entry.Repository,
				entry.TagPrefix,
				existing,
				name,
			)
		}
		sources[key] = name
	}
}
