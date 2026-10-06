package registry_test

import (
	"testing"

	"github.com/rshade/finfocus/internal/registry"
)

func TestGetEmbeddedRegistry(t *testing.T) {
	t.Parallel()

	reg, err := registry.GetEmbeddedRegistry()
	if err != nil {
		t.Fatalf("GetEmbeddedRegistry() error = %v", err)
	}

	if reg.SchemaVersion != "1.0.0" {
		t.Errorf("SchemaVersion = %v, want 1.0.0", reg.SchemaVersion)
	}

	if len(reg.Plugins) == 0 {
		t.Error("Plugins is empty, expected at least one plugin")
	}
}

func TestGetPlugin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		plugin    string
		wantFound bool
	}{
		{"existing plugin", "opencost", true},
		{"aws-public plugin", "aws-public", true},
		{"non-existent plugin", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry, err := registry.GetPlugin(tt.plugin)
			if tt.wantFound {
				if err != nil {
					t.Errorf("GetPlugin(%q) error = %v", tt.plugin, err)
				}
				if entry == nil {
					t.Errorf("GetPlugin(%q) returned nil", tt.plugin)
				}
			} else if err == nil {
				t.Errorf("GetPlugin(%q) expected error, got nil", tt.plugin)
			}
		})
	}
}

func TestListRegistryPlugins(t *testing.T) {
	t.Parallel()

	plugins, err := registry.ListPluginsFromRegistry()
	if err != nil {
		t.Fatalf("ListPluginsFromRegistry() error = %v", err)
	}

	if len(plugins) < 2 {
		t.Errorf("ListPluginsFromRegistry() returned %d plugins, want at least 2", len(plugins))
	}

	// Check that expected plugins are in the list
	found := make(map[string]bool)
	for _, p := range plugins {
		found[p] = true
	}

	for _, expected := range []string{"opencost", "aws-public"} {
		if !found[expected] {
			t.Errorf("ListPluginsFromRegistry() missing expected plugin %q", expected)
		}
	}
}
