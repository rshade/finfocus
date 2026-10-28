package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestConfigPulumiCloudSetAndGet(t *testing.T) {
	setupTestConfig(t)
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	t.Chdir(t.TempDir())
	const key = "cost.history.pulumi_cloud.enabled"
	path := filepath.Join(os.Getenv("FINFOCUS_HOME"), "config.hujson")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	for _, document := range []string{`{}`, `{"cost":{"history":{"pulumi_cloud":{"enabled":false}}}}`} {
		require.NoError(t, os.WriteFile(path, []byte(document), 0o600))
		result := axtest.Run(
			context.Background(),
			t,
			cli.NewRootCmd("test"),
			[]string{"config", "get", key},
		)
		require.Equal(t, 0, result.ExitCode, string(result.Stderr))
		assert.Equal(t, "false\n", string(result.Stdout))
	}
	require.NoError(t, os.WriteFile(path, []byte("{\n // retain me\n}\n"), 0o600))
	result := axtest.Run(
		context.Background(),
		t,
		cli.NewRootCmd("test"),
		[]string{"config", "set", key, "auto"},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "// retain me")
	result = axtest.Run(
		context.Background(),
		t,
		cli.NewRootCmd("test"),
		[]string{"config", "get", key},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Equal(t, "auto\n", string(result.Stdout))
	result = axtest.Run(
		context.Background(),
		t,
		cli.NewRootCmd("test"),
		[]string{"config", "set", key, "maybe"},
	)
	assert.Equal(t, 2, result.ExitCode)
	for _, allowed := range []string{"false", "auto", "true"} {
		assert.Contains(t, string(result.Stdout)+string(result.Stderr), allowed)
	}
}

func TestConfigPulumiCloudRejectsInvalidMode(t *testing.T) {
	setupTestConfig(t)
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Chdir(t.TempDir())
	path := filepath.Join(os.Getenv("FINFOCUS_HOME"), "config.hujson")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(
		t,
		os.WriteFile(
			path,
			[]byte(`{"cost":{"history":{"pulumi_cloud":{"enabled":"maybe"}}}}`),
			0o600,
		),
	)
	for _, args := range [][]string{{"config", "validate"}, {"cost", "actual", "--from", "2026-10-01", "--to", "2026-10-02"}} {
		result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), args)
		assert.Equal(t, 2, result.ExitCode, "%v: %s", args, result.Stderr)
		for _, allowed := range []string{"false", "auto", "true"} {
			assert.Contains(t, string(result.Stdout)+string(result.Stderr), allowed)
		}
	}
}
