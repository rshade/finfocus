package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/plugins/kubernetes/workload"
)

func envOf(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestLoadConfig_Unset(t *testing.T) {
	t.Parallel()

	cfg := LoadConfig(envOf(nil))

	assert.False(t, cfg.CPUHourlyRate.Set)
	assert.False(t, cfg.MemoryGiBHourlyRate.Set)
	assert.False(t, cfg.DaemonSetNodeCount.Set)
	assert.False(t, cfg.JobHoursPerMonth.Set)
	require.NoError(t, cfg.CPUHourlyRate.Err)
	require.NoError(t, cfg.MemoryGiBHourlyRate.Err)
	require.NoError(t, cfg.DaemonSetNodeCount.Err)
	require.NoError(t, cfg.JobHoursPerMonth.Err)
}

func TestLoadConfig_Valid(t *testing.T) {
	t.Parallel()

	cfg := LoadConfig(envOf(map[string]string{
		workload.EnvCPUHourlyRate:       "0.04",
		workload.EnvMemoryGiBHourlyRate: " 0 ",
		workload.EnvDaemonSetNodeCount:  "4",
		workload.EnvJobHoursPerMonth:    "744",
	}))

	assert.True(t, cfg.CPUHourlyRate.Set)
	require.NoError(t, cfg.CPUHourlyRate.Err)
	assert.InDelta(t, 0.04, cfg.CPUHourlyRate.Value, 1e-12)
	assert.True(t, cfg.MemoryGiBHourlyRate.Set)
	require.NoError(t, cfg.MemoryGiBHourlyRate.Err)
	assert.InDelta(t, 0.0, cfg.MemoryGiBHourlyRate.Value, 1e-12, "a zero rate is an explicit free price")
	require.NoError(t, cfg.DaemonSetNodeCount.Err)
	assert.Equal(t, 4, cfg.DaemonSetNodeCount.Value)
	require.NoError(t, cfg.JobHoursPerMonth.Err)
	assert.InDelta(t, 744.0, cfg.JobHoursPerMonth.Value, 1e-12)
}

func TestLoadConfig_InvalidRates(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"-0.01", "NaN", "Inf", "-Inf", "cheap"} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			cfg := LoadConfig(envOf(map[string]string{
				workload.EnvCPUHourlyRate:       raw,
				workload.EnvMemoryGiBHourlyRate: raw,
			}))
			assert.True(t, cfg.CPUHourlyRate.Set)
			require.Error(t, cfg.CPUHourlyRate.Err)
			assert.Contains(t, cfg.CPUHourlyRate.Err.Error(), workload.EnvCPUHourlyRate)
			require.Error(t, cfg.MemoryGiBHourlyRate.Err)
			assert.Contains(t, cfg.MemoryGiBHourlyRate.Err.Error(), workload.EnvMemoryGiBHourlyRate)
		})
	}
}

func TestLoadConfig_InvalidNodeCount(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"0", "-1", "1.5", "four"} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			cfg := LoadConfig(envOf(map[string]string{workload.EnvDaemonSetNodeCount: raw}))
			assert.True(t, cfg.DaemonSetNodeCount.Set)
			require.Error(t, cfg.DaemonSetNodeCount.Err)
			assert.Contains(t, cfg.DaemonSetNodeCount.Err.Error(), workload.EnvDaemonSetNodeCount)
		})
	}
}

func TestLoadConfig_JobHours(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw   string
		valid bool
	}{
		{"0", false},
		{"745", false},
		{"-3", false},
		{"NaN", false},
		{"744", true},
		{"0.5", true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			cfg := LoadConfig(envOf(map[string]string{workload.EnvJobHoursPerMonth: tt.raw}))
			if tt.valid {
				require.NoError(t, cfg.JobHoursPerMonth.Err)
				return
			}
			require.Error(t, cfg.JobHoursPerMonth.Err)
			assert.Contains(t, cfg.JobHoursPerMonth.Err.Error(), workload.EnvJobHoursPerMonth)
		})
	}
}
