package config

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCostHistoryScalarsAreValuesNotPointers(t *testing.T) {
	t.Parallel()
	explicit := false
	days := 14
	for _, tc := range []struct {
		name        string
		history     HistoryConfig
		wantEnabled bool
		wantDays    int
	}{
		{"unset resolves to defaults", HistoryConfig{}, HistoryDefaultEnabled, HistoryDefaultRetentionDays},
		{"explicit values", HistoryConfig{Enabled: &explicit, RetentionDays: &days}, false, 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := New()
			cfg.Cost.History = tc.history
			enabled, err := cfg.Get("cost.history.enabled")
			require.NoError(t, err)
			retention, err := cfg.Get("cost.history.retention_days")
			require.NoError(t, err)
			assert.Equal(t, tc.wantEnabled, enabled)
			assert.Equal(t, tc.wantDays, retention)
			assert.Equal(t, strconv.FormatBool(tc.wantEnabled), fmt.Sprintf("%v", enabled))
		})
	}
}
