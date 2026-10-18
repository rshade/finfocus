package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

//nolint:paralleltest // t.Setenv in subtests changes the process-wide environment
func TestResolveUsageProfilePrecedence(t *testing.T) {
	if _, ok := os.LookupEnv(config.UsageProfileEnv); ok {
		t.Skip("FINFOCUS_PROFILE is set in the environment")
	}
	cmd := &cobra.Command{Use: "cost"}
	cmd.Flags().String("profile", "", usageProfileFlagHelp)
	cfg := &config.Config{}
	cfg.Cost.Profile = "prod"

	t.Run("config", func(t *testing.T) {
		profile, err := resolveUsageProfile(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_PROD, profile)
	})

	t.Run("env overrides config", func(t *testing.T) {
		t.Setenv(config.UsageProfileEnv, "burst")
		profile, err := resolveUsageProfile(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_BURST, profile)
	})

	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv(config.UsageProfileEnv, "burst")
		require.NoError(t, cmd.ParseFlags([]string{"--profile=dev"}))
		profile, err := resolveUsageProfile(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_DEV, profile)
	})
}

func TestResolveUsageProfileRejectsPresentEmpty(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "cost"}
	cmd.Flags().String("profile", "", usageProfileFlagHelp)
	require.NoError(t, cmd.ParseFlags([]string{"--profile="}))
	_, err := resolveUsageProfile(cmd, nil)
	require.Error(t, err)

	cfg := &config.Config{}
	cfg.Cost.Profile = "staging"
	_, err = resolveUsageProfile(&cobra.Command{Use: "cfg"}, cfg)
	require.Error(t, err)
}

func TestResolveUsageProfileRejectsEmptyEnv(t *testing.T) {
	t.Setenv(config.UsageProfileEnv, "   ")
	_, err := resolveUsageProfile(&cobra.Command{Use: "plain"}, nil)
	require.Error(t, err)
}

func TestResolveUsageProfileDefaultIsNoPreference(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "cost"}
	cmd.Flags().String("profile", "", usageProfileFlagHelp)
	require.NoError(t, cmd.ParseFlags([]string{"--profile=default"}))
	profile, err := resolveUsageProfile(cmd, &config.Config{Cost: config.CostConfig{Profile: "prod"}})
	require.NoError(t, err)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, profile)
}

func TestResolveUsageProfileRejectsExplicitEmptyConfig(t *testing.T) {
	t.Parallel()
	var cfg config.Config
	require.NoError(t, json.Unmarshal([]byte(`{"cost":{"profile":""}}`), &cfg))
	_, err := resolveUsageProfile(nil, &cfg)
	require.ErrorContains(t, err, "usage profile")

	var omitted config.Config
	require.NoError(t, json.Unmarshal([]byte(`{"cost":{}}`), &omitted))
	profile, err := resolveUsageProfile(nil, &omitted)
	require.NoError(t, err)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, profile)
}

func TestApplyUsageProfile(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "projected"}
	cmd.Flags().String("profile", "", usageProfileFlagHelp)
	require.NoError(t, cmd.ParseFlags([]string{"--profile=development"}))
	eng := engine.New(nil, nil)
	require.NoError(t, applyUsageProfile(cmd, eng, nil))
	assert.NoError(t, applyUsageProfile(cmd, nil, nil))
}

//nolint:paralleltest // SetGlobalConfig replaces the process-wide config singleton
func TestUsageProfileSourcePrefersMergedConfig(t *testing.T) {
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })

	config.SetGlobalConfig(&config.Config{Cost: config.CostConfig{Profile: "dev"}})
	fallback := &config.Config{Cost: config.CostConfig{Profile: "prod"}}
	profile, err := resolveUsageProfile(nil, usageProfileSource(fallback))
	require.NoError(t, err)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_DEV, profile)

	config.ResetGlobalConfigForTest()
	profile, err = resolveUsageProfile(nil, usageProfileSource(fallback))
	require.NoError(t, err)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_PROD, profile)
}

//nolint:paralleltest // SetGlobalConfig replaces the process-wide config singleton
func TestCostPreRunRejectsUnknownProfile(t *testing.T) {
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })
	config.SetGlobalConfig(nil)

	cmd := newCostCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--profile=staging"}))
	err := cmd.PersistentPreRunE(cmd, []string{})
	require.Error(t, err)
	require.ErrorContains(t, err, "usage profile")

	require.NoError(t, cmd.ParseFlags([]string{"--profile=prod"}))
	require.NoError(t, cmd.PersistentPreRunE(cmd, []string{}))
}

func TestRecommendationsOutputNamesProfile(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{Profile: "dev", Currency: "USD"}
	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsJSON(&buf, result, nil))
	assert.Contains(t, buf.String(), `"profile": "dev"`)

	buf.Reset()
	require.NoError(t, renderRecommendationsNDJSON(&buf, result, nil))
	assert.Contains(t, buf.String(), `"profile":"dev"`)

	buf.Reset()
	require.NoError(t, renderRecommendationsTableWithVerbose(&buf, result, false))
	assert.Contains(t, buf.String(), "Profile: dev")
	assert.Contains(t, buf.String(), "RECOMMENDATIONS SUMMARY")
}
