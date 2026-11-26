package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/conformance"
)

func TestPluginConformance_ExpectedNameFlag(t *testing.T) {
	t.Parallel()

	cmd := NewPluginConformanceCmd()
	flag := cmd.Flags().Lookup("expected-name")
	require.NotNil(t, flag)
	assert.Empty(t, flag.DefValue)

	binary := filepath.Join(t.TempDir(), "finfocus-plugin-aws-public")
	require.NoError(
		t,
		os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755),
	)
	cfg, err := buildSuiteConfig(
		context.Background(),
		binary,
		"tcp",
		"normal",
		"1m",
		nil,
		"",
		probeFlags{},
		"aws-public",
	)
	require.NoError(t, err)
	assert.Equal(t, "aws-public", cfg.ExpectedName)
}

func TestPluginConformance_WarningsKeepExitCode(t *testing.T) {
	t.Parallel()

	report := &conformance.SuiteReport{Summary: conformance.Summary{Total: 3, Passed: 1, Warnings: 2}}
	assert.NoError(t, checkResults(report))
}
