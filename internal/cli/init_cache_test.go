package cli_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine/cache"
)

func newTestCmdWithCacheTTL(ttl int) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("cache-ttl", ttl, "cache TTL in seconds")
	return cmd
}

func TestInitCache(t *testing.T) {
	tests := []struct {
		name     string
		flagTTL  int
		flagSet  bool // simulate user explicitly passing --cache-ttl
		envTTL   string
		wantNil  bool
		wantDesc string
	}{
		{
			name:     "explicit --cache-ttl 0 disables cache",
			flagTTL:  0,
			flagSet:  true,
			wantNil:  true,
			wantDesc: "explicit --cache-ttl 0 disables caching",
		},
		{
			name:     "no flag and no env uses config default (non-nil)",
			flagTTL:  0,
			flagSet:  false,
			wantNil:  false,
			wantDesc: "config default enables caching",
		},
		{
			name:     "positive --cache-ttl returns non-nil Cache",
			flagTTL:  3600,
			flagSet:  true,
			wantNil:  false,
			wantDesc: "non-zero TTL enables caching",
		},
		{
			name:     "env var override provides TTL",
			flagTTL:  0,
			flagSet:  false,
			envTTL:   "1800",
			wantNil:  false,
			wantDesc: "env var overrides config",
		},
		{
			name:     "env var 0 disables cache",
			flagTTL:  0,
			flagSet:  false,
			envTTL:   "0",
			wantNil:  true,
			wantDesc: "env var 0 disables caching",
		},
		{
			name:     "CLI flag overrides env var",
			flagTTL:  7200,
			flagSet:  true,
			envTTL:   "1800",
			wantNil:  false,
			wantDesc: "flag takes precedence over env var",
		},
		{
			name:     "invalid env var falls back to config",
			flagTTL:  0,
			flagSet:  false,
			envTTL:   "not-a-number",
			wantNil:  false,
			wantDesc: "invalid env var logs warning, falls back to config default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup env vars (always use t.Setenv for proper restore)
			if tt.envTTL != "" {
				t.Setenv(cache.EnvTTLSeconds, tt.envTTL)
			} else {
				t.Setenv(cache.EnvTTLSeconds, "")
			}
			t.Setenv(cache.EnvTTLSecondsLegacy, "")

			cmd := newTestCmdWithCacheTTL(tt.flagTTL)
			// Simulate user explicitly setting the flag
			if tt.flagSet {
				require.NoError(t, cmd.Flags().Set("cache-ttl", toString(tt.flagTTL)))
			}
			ctx := context.Background()

			result := cli.InitCache(ctx, cmd)

			if tt.wantNil {
				assert.Nil(t, result, tt.wantDesc)
			} else {
				require.NotNil(t, result, tt.wantDesc)
				assert.True(t, result.IsEnabled(), "returned cache should be enabled")
			}
		})
	}
}

func TestInitCache_InitFailureReturnsNilGracefully(t *testing.T) {
	// Set FINFOCUS_CACHE_DIR to an unwritable path to trigger init failure
	t.Setenv("FINFOCUS_CACHE_DIR", "/proc/nonexistent/cache")
	t.Setenv(cache.EnvTTLSeconds, "3600")

	cmd := newTestCmdWithCacheTTL(0)
	ctx := context.Background()

	result := cli.InitCache(ctx, cmd)
	assert.Nil(t, result, "init failure should return nil gracefully")
}

func toString(i int) string {
	return strconv.Itoa(i)
}

func TestInitCache_LegacyTTLSecondsEnvVar(t *testing.T) {
	cmd := newTestCmdWithCacheTTL(0)
	ctx := context.Background()

	// New env var unset; legacy env var disables caching.
	t.Setenv(cache.EnvTTLSeconds, "")
	t.Setenv(cache.EnvTTLSecondsLegacy, "0")

	result := cli.InitCache(ctx, cmd)
	assert.Nil(t, result, "legacy TTL env var should be honored")
}
