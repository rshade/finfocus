package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestParseUsageProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    pbc.UsageProfile
		wantErr bool
	}{
		{name: "absent", value: "", want: pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED},
		{name: "default", value: "default", want: pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED},
		{name: "unspecified", value: "UNSPECIFIED", want: pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED},
		{name: "dev", value: "DEV", want: pbc.UsageProfile_USAGE_PROFILE_DEV},
		{name: "development", value: "development", want: pbc.UsageProfile_USAGE_PROFILE_DEV},
		{name: "prod", value: "prod", want: pbc.UsageProfile_USAGE_PROFILE_PROD},
		{name: "production", value: " Production ", want: pbc.UsageProfile_USAGE_PROFILE_PROD},
		{name: "burst", value: "burst", want: pbc.UsageProfile_USAGE_PROFILE_BURST},
		{name: "unknown", value: "staging", wantErr: true},
		{name: "spaces", value: "  ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseUsageProfile(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, UsageProfileLabel(got), labelFor(tt.want))
		})
	}
}

func TestParseRequiredUsageProfileRejectsEmpty(t *testing.T) {
	t.Parallel()
	_, err := ParseRequiredUsageProfile("")
	require.Error(t, err)
	_, err = ParseRequiredUsageProfile("   ")
	require.Error(t, err)
}

func TestCostConfigValidateProfile(t *testing.T) {
	t.Parallel()

	require.NoError(t, (CostConfig{}).Validate())
	require.NoError(t, (CostConfig{Profile: "dev"}).Validate())
	require.NoError(t, (CostConfig{Profile: "default"}).Validate())
	err := (CostConfig{Profile: "staging"}).Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "profile")
	err = (CostConfig{Profile: " "}).Validate()
	require.Error(t, err)
}

func TestCostConfigRejectsExplicitEmptyProfile(t *testing.T) {
	t.Parallel()

	var present CostConfig
	require.NoError(t, json.Unmarshal([]byte(`{"profile":""}`), &present))
	assert.True(t, present.HasProfile())
	require.ErrorContains(t, present.Validate(), "profile")

	var omitted CostConfig
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	assert.False(t, omitted.HasProfile())
	require.NoError(t, omitted.Validate())

	var fromYAML CostConfig
	require.NoError(t, yaml.Unmarshal([]byte("profile: \"\"\n"), &fromYAML))
	assert.True(t, fromYAML.HasProfile())
	require.ErrorContains(t, fromYAML.Validate(), "profile")
}

func TestLoadConfigRejectsExplicitEmptyProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.hujson")
	require.NoError(t, os.WriteFile(path, []byte(`{"cost":{"profile":""}}`), 0o600))
	cfg := &Config{}
	require.NoError(t, loadConfig(path, cfg))
	assert.True(t, cfg.Cost.HasProfile())
	require.ErrorContains(t, cfg.Cost.Validate(), "profile")

	require.NoError(t, os.WriteFile(path, []byte(`{"cost":{"pricing_spec_fallback":false}}`), 0o600))
	omitted := &Config{}
	require.NoError(t, loadConfig(path, omitted))
	assert.False(t, omitted.Cost.HasProfile())
	require.NoError(t, omitted.Cost.Validate())

	overlay := filepath.Join(dir, "project.yaml")
	require.NoError(t, os.WriteFile(overlay, []byte("cost:\n  profile: \"\"\n"), 0o600))
	merged := &Config{}
	require.NoError(t, ShallowMergeYAML(merged, overlay))
	assert.True(t, merged.Cost.HasProfile())
	require.ErrorContains(t, merged.Cost.Validate(), "profile")
}

func labelFor(profile pbc.UsageProfile) string {
	switch profile {
	case pbc.UsageProfile_USAGE_PROFILE_DEV:
		return "dev"
	case pbc.UsageProfile_USAGE_PROFILE_PROD:
		return "prod"
	case pbc.UsageProfile_USAGE_PROFILE_BURST:
		return "burst"
	case pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED:
		return "default"
	default:
		return "default"
	}
}
