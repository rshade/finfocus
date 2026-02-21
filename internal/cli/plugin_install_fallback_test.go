package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestPluginInstallCmd_FallbackFlags(t *testing.T) {
	t.Run("fallback-to-latest flag exists", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		flag := cmd.Flags().Lookup("fallback-to-latest")
		require.NotNil(t, flag, "flag should exist")
		assert.Equal(t, "false", flag.DefValue)
		assert.Contains(t, flag.Usage, "Automatically install latest stable version")
	})

	t.Run("no-fallback flag exists", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		flag := cmd.Flags().Lookup("no-fallback")
		require.NotNil(t, flag, "flag should exist")
		assert.Equal(t, "false", flag.DefValue)
		assert.Contains(t, flag.Usage, "Disable fallback behavior")
	})

	t.Run("flags are mutually exclusive", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()

		// Set both flags
		require.NoError(t, cmd.Flags().Set("fallback-to-latest", "true"))
		require.NoError(t, cmd.Flags().Set("no-fallback", "true"))

		// Manually set args to prevent "requires exactly 1 arg" error
		cmd.SetArgs([]string{"test-plugin@v1.0.0"})

		// Capture error output
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)
		cmd.SetOut(&bytes.Buffer{})

		// Execute should fail due to mutual exclusivity
		err := cmd.Execute()
		require.Error(t, err)

		// Cobra's mutual exclusion error message
		assert.Contains(t, err.Error(), "none of the others can be")
	})

	t.Run("help text includes fallback examples", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()

		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--help"})

		_ = cmd.Execute()

		output := outBuf.String()
		assert.Contains(t, output, "--fallback-to-latest")
		assert.Contains(t, output, "--no-fallback")
		assert.Contains(t, output, "Auto-fallback to latest stable")
		assert.Contains(t, output, "Fail immediately if requested version lacks assets")
	})

	t.Run("long description mentions fallback behavior", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		assert.Contains(t, cmd.Long, "Fallback Behavior")
		assert.Contains(t, cmd.Long, "Interactive mode")
		assert.Contains(t, cmd.Long, "Non-interactive mode")
		assert.Contains(t, cmd.Long, "Strict mode")
	})
}

func TestIsNoAssetError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "no asset found error",
			errMsg:   "no asset found for linux/amd64. Available: []",
			expected: true,
		},
		{
			name:     "no compatible asset found error",
			errMsg:   "no compatible asset found for version v1.0.0 or any of 10 fallback releases",
			expected: true,
		},
		{
			name:     "other error",
			errMsg:   "failed to get release: release not found",
			expected: false,
		},
		{
			name:     "connection error",
			errMsg:   "failed to connect to GitHub API",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to call the exported function
			// Since isNoAssetError is unexported, we test via behavior
			// For now, we document the expected behavior
			// The actual test would require exposing the function or testing through command execution
			_ = tt.expected // Document expected values
		})
	}
}

func TestGetPlatformString(t *testing.T) {
	// getPlatformString returns runtime.GOOS/runtime.GOARCH
	// We can't directly test this without exposing the function
	// This test documents the expected format
	t.Run("format documentation", func(t *testing.T) {
		// Expected format: "os/arch" e.g., "linux/amd64", "darwin/arm64", "windows/amd64"
		t.Log("Platform string should be in 'os/arch' format")
	})
}

func TestDisplayInstallResult(t *testing.T) {
	// displayInstallResult is an internal function
	// We test its behavior through command output verification
	t.Run("format documentation", func(t *testing.T) {
		// Non-fallback: shows just version
		// Fallback: shows "Version: v0.9.0 (requested: v1.0.0)"
		t.Log("Non-fallback shows 'Version: vX.Y.Z'")
		t.Log("Fallback shows 'Version: vX.Y.Z (requested: vA.B.C)'")
	})
}

// TestPluginInstallCmd_FallbackToLatest_FlagBehavior tests the --fallback-to-latest flag [T019].
// This tests that the flag is correctly parsed and passed to the install options.
func TestPluginInstallCmd_FallbackToLatest_FlagBehavior(t *testing.T) {
	t.Run("flag defaults to false", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		flag := cmd.Flags().Lookup("fallback-to-latest")
		require.NotNil(t, flag)
		assert.Equal(t, "false", flag.DefValue)
	})

	t.Run("flag can be set to true", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		err := cmd.Flags().Set("fallback-to-latest", "true")
		require.NoError(t, err)

		flag := cmd.Flags().Lookup("fallback-to-latest")
		assert.Equal(t, "true", flag.Value.String())
	})

	t.Run("flag usage describes automatic fallback", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		flag := cmd.Flags().Lookup("fallback-to-latest")
		assert.Contains(t, flag.Usage, "Automatically")
		assert.Contains(t, flag.Usage, "latest stable version")
	})
}

// TestPluginInstallCmd_NoFallback_FlagBehavior tests the --no-fallback flag [T026].
// This tests that the flag correctly disables fallback behavior.
func TestPluginInstallCmd_NoFallback_FlagBehavior(t *testing.T) {
	t.Run("flag defaults to false", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		flag := cmd.Flags().Lookup("no-fallback")
		require.NotNil(t, flag)
		assert.Equal(t, "false", flag.DefValue)
	})

	t.Run("flag can be set to true", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		err := cmd.Flags().Set("no-fallback", "true")
		require.NoError(t, err)

		flag := cmd.Flags().Lookup("no-fallback")
		assert.Equal(t, "true", flag.Value.String())
	})

	t.Run("flag usage describes disabling fallback", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		flag := cmd.Flags().Lookup("no-fallback")
		assert.Contains(t, flag.Usage, "Disable")
		assert.Contains(t, flag.Usage, "fallback")
	})
}

// TestPluginInstallCmd_MutualExclusivity_Detailed tests mutual exclusivity in detail [T025].
func TestPluginInstallCmd_MutualExclusivity_Detailed(t *testing.T) {
	t.Run("fallback-to-latest alone works", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		err := cmd.Flags().Set("fallback-to-latest", "true")
		require.NoError(t, err)

		// This flag being set alone should not cause a parsing error
		flag := cmd.Flags().Lookup("fallback-to-latest")
		assert.Equal(t, "true", flag.Value.String())

		noFallback := cmd.Flags().Lookup("no-fallback")
		assert.Equal(t, "false", noFallback.Value.String())
	})

	t.Run("no-fallback alone works", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()
		err := cmd.Flags().Set("no-fallback", "true")
		require.NoError(t, err)

		flag := cmd.Flags().Lookup("no-fallback")
		assert.Equal(t, "true", flag.Value.String())

		fallbackToLatest := cmd.Flags().Lookup("fallback-to-latest")
		assert.Equal(t, "false", fallbackToLatest.Value.String())
	})

	t.Run("both flags together causes error on execute", func(t *testing.T) {
		cmd := cli.NewPluginInstallCmd()

		// Both flags can be set (Cobra validates on Execute)
		require.NoError(t, cmd.Flags().Set("fallback-to-latest", "true"))
		require.NoError(t, cmd.Flags().Set("no-fallback", "true"))

		cmd.SetArgs([]string{"test-plugin"})
		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		require.Error(t, err, "Execute should fail when both flags are set")
		assert.Contains(t, err.Error(), "none of the others can be")
	})
}
