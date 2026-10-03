package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine/cache"
)

func TestInitCacheFromConfig_LRUTier(t *testing.T) {
	tests := []struct {
		name       string
		envEnabled string
		cfgEnabled bool
		wantTier   bool
	}{
		{name: "env enables tier", envEnabled: "true", wantTier: true},
		{name: "off when unset", envEnabled: "", wantTier: false},
		{name: "config enables tier", envEnabled: "", cfgEnabled: true, wantTier: true},
		{name: "env false overrides config", envEnabled: "false", cfgEnabled: true, wantTier: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(cache.EnvCacheDir, t.TempDir())
			t.Setenv(cache.EnvTTLSeconds, "")
			t.Setenv(cache.EnvTTLSecondsLegacy, "")
			t.Setenv(cache.EnvLRUEnabled, tt.envEnabled)
			t.Setenv(cache.EnvLRUMaxItems, "8")

			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().Int("cache-ttl", 0, "cache TTL in seconds")
			require.NoError(t, cmd.Flags().Set("cache-ttl", "3600"))

			cfg := &config.Config{}
			cfg.Cost.Cache.LRUEnabled = tt.cfgEnabled
			store := initCacheFromConfig(context.Background(), cmd, cfg)
			require.NotNil(t, store)
			t.Cleanup(func() {
				require.NoError(t, store.Close())
			})

			tier, isTier := store.(*cache.TieredStore)
			assert.Equal(t, tt.wantTier, isTier)
			if !tt.wantTier {
				_, isBolt := store.(*cache.BoltStore)
				assert.True(t, isBolt)
				return
			}
			require.NoError(t, tier.Set("projected/a", json.RawMessage(`"v"`)))
			got, err := tier.Get("projected/a")
			require.NoError(t, err)
			payload := string(got.Data)
			assert.JSONEq(t, `"v"`, payload)
		})
	}
}
