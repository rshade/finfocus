package carbon

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnv(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, value string
		want        float64
		wantErr     bool
	}{
		{name: "default", want: 0.5}, {name: "idle", value: "0", want: 0},
		{name: "full", value: "1", want: 1}, {name: "custom", value: "0.25", want: 0.25},
		{name: "negative", value: "-1", wantErr: true}, {name: "over", value: "1.01", wantErr: true},
		{name: "nan", value: "NaN", wantErr: true}, {name: "infinite", value: "+Inf", wantErr: true},
		{name: "invalid", value: "half", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := ConfigFromEnv(func(key string) string { assert.Equal(t, EnvUtilization, key); return tt.value })
			if tt.wantErr {
				require.ErrorContains(t, err, EnvUtilization)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, cfg.Utilization, 1e-12)
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	t.Parallel()
	for _, u := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		_, err := EstimateResource(testResource("t3.medium", "us-east-1"), Config{Utilization: u})
		require.Error(t, err)
	}
}
