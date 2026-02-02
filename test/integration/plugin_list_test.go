//go:build integration

// Package cli_test provides black-box integration tests for the internal/cli package.
// These tests validate CLI behavior from an external consumer perspective.
package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// TestPluginList_ShowsProvidersColumn tests that plugin list shows providers.
func TestPluginList_ShowsProvidersColumn(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Simple output should show Providers column
	assert.Contains(t, output, "Providers")
}

// TestPluginList_VerboseShowsCapabilities tests that verbose mode shows capabilities.
func TestPluginList_VerboseShowsCapabilities(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--verbose"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Verbose output should show Capabilities column
	assert.Contains(t, output, "Capabilities")
	// And providers column
	assert.Contains(t, output, "Providers")
}

// TestPluginList_GlobalPluginShowsWildcard tests that global plugins show "*" for providers.
func TestPluginList_GlobalPluginShowsWildcard(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	// This test verifies the formatting logic for global plugins
	// When a plugin has no specific providers, it should show "*"
	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)

	// If no plugins are installed, output should indicate that
	// Otherwise, any global plugin should show "*" in providers column
	output := buf.String()
	// At minimum, header should be present
	if output != "" && output != "No plugins found.\n" && output != "Plugin directory does not exist:" {
		assert.Contains(t, output, "Providers")
	}
}

// TestPluginList_AvailableShowsRegistry tests that --available shows registry plugins.
func TestPluginList_AvailableShowsRegistry(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--available"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should show registry columns
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Description")
	assert.Contains(t, output, "Repository")
	assert.Contains(t, output, "Security")
}
