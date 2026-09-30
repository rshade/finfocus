package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoringConfig_ResolveDefaults(t *testing.T) {
	var s *ScoringConfig
	got := s.Resolve()

	assert.False(t, got.Enabled)
	assert.Equal(t, ScoringIdentifierPseudonymized, got.IdentifierMode)
	assert.Equal(t, DefaultScoringTimeoutSeconds, got.TimeoutSeconds)
	assert.InDelta(t, DefaultScoringReviewRisk, got.RiskThreshold, 1e-9)
	assert.InDelta(t, DefaultScoringReviewDeadBand, got.DeadBand, 1e-9)
}

func TestScoringConfig_ResolveOverrides(t *testing.T) {
	risk := 0.5
	band := 0.2
	s := &ScoringConfig{
		Enabled:        true,
		Plugin:         "scorer",
		IdentifierMode: " RAW ",
		TimeoutSeconds: 5,
		FieldAllowlist: []string{"impact"},
		NeedsReview:    &ScoringReviewConfig{Risk: &risk, DeadBand: &band},
	}
	got := s.Resolve()

	assert.True(t, got.Enabled)
	assert.Equal(t, "scorer", got.Plugin)
	assert.Equal(t, ScoringIdentifierRaw, got.IdentifierMode)
	assert.Equal(t, 5, got.TimeoutSeconds)
	assert.InDelta(t, 0.5, got.RiskThreshold, 1e-9)
	assert.InDelta(t, DefaultScoringReviewFalsePositive, got.FalsePositiveLimit, 1e-9)
	assert.InDelta(t, 0.2, got.DeadBand, 1e-9)
	assert.Equal(t, []string{"impact"}, got.FieldAllowlist)
}

func TestScoringConfig_Validate(t *testing.T) {
	bad := 1.5
	tests := []struct {
		name    string
		cfg     *ScoringConfig
		wantErr string
	}{
		{"nil is valid", nil, ""},
		{"disabled empty is valid", &ScoringConfig{}, ""},
		{"enabled requires plugin", &ScoringConfig{Enabled: true}, "scoring.plugin is required"},
		{"bad identifier mode", &ScoringConfig{IdentifierMode: "hashed"}, "identifier_mode"},
		{"negative timeout", &ScoringConfig{TimeoutSeconds: -1}, "timeout_seconds"},
		{"unknown allowlist field", &ScoringConfig{FieldAllowlist: []string{"nope"}}, "field_allowlist"},
		{
			"all allowlist fields valid",
			&ScoringConfig{FieldAllowlist: []string{
				"action_detail", "primary_reason", "secondary_reasons",
			}},
			"",
		},
		{
			"threshold out of range",
			&ScoringConfig{NeedsReview: &ScoringReviewConfig{Risk: &bad}},
			"needs_review.risk",
		},
		{"valid full", &ScoringConfig{Enabled: true, Plugin: "p", IdentifierMode: "omitted"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestConfig_ScoringSetGet(t *testing.T) {
	c := &Config{}

	require.NoError(t, c.Set("scoring.plugin", "scorer"))
	require.NoError(t, c.Set("scoring.enabled", "true"))
	require.NoError(t, c.Set("scoring.identifier_mode", "omitted"))
	require.NoError(t, c.Set("scoring.timeout_seconds", "12"))
	require.NoError(t, c.Set("scoring.field_allowlist", "impact, description"))
	require.NoError(t, c.Set("scoring.needs_review.risk", "0.4"))

	enabled, err := c.Get("scoring.enabled")
	require.NoError(t, err)
	assert.Equal(t, true, enabled)
	mode, err := c.Get("scoring.identifier_mode")
	require.NoError(t, err)
	assert.Equal(t, "omitted", mode)
	list, err := c.Get("scoring.field_allowlist")
	require.NoError(t, err)
	assert.Equal(t, []string{"impact", "description"}, list)
	risk, err := c.Get("scoring.needs_review.risk")
	require.NoError(t, err)
	assert.InDelta(t, 0.4, risk, 1e-9)

	require.Error(t, c.Set("scoring.identifier_mode", "hashed"))
	require.Error(t, c.Set("scoring.timeout_seconds", "abc"))
	require.Error(t, c.Set("scoring.needs_review.risk", "2"))
	require.Error(t, c.Set("scoring.bogus", "x"))
	require.Error(t, c.Set("scoring", "x"))
	_, err = c.Get("scoring.bogus")
	require.Error(t, err)

	mode, err = c.Get("scoring.identifier_mode")
	require.NoError(t, err)
	assert.Equal(t, "omitted", mode, "failed Set must not change the stored value")
}

func TestConfig_ScoringDisabledByDefaultAndOmittedFromJSON(t *testing.T) {
	c := &Config{Output: OutputConfig{DefaultFormat: "table"}}
	assert.False(t, c.Scoring.Resolve().Enabled)

	data, err := json.Marshal(c)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "scoring")
}

func TestShallowMergeYAML_Scoring(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.hujson")
	require.NoError(t, os.WriteFile(path, []byte(`{
		// scorer opt-in
		"scoring": {"enabled": true, "plugin": "scorer", "field_allowlist": ["impact"]}
	}`), 0o600))

	target := &Config{}
	require.NoError(t, ShallowMergeYAML(target, path))

	require.NotNil(t, target.Scoring)
	assert.True(t, target.Scoring.Enabled)
	assert.Equal(t, "scorer", target.Scoring.Plugin)
	assert.Equal(t, []string{"impact"}, target.Scoring.FieldAllowlist)
}

func TestConfig_ScoringRoundTripKeepsKey(t *testing.T) {
	var c Config
	require.NoError(t, json.Unmarshal([]byte(`{"scoring":{"enabled":true,"plugin":"p"}}`), &c))
	require.NotNil(t, c.Scoring)
	assert.Empty(t, c.extraKeys, "scoring must be a recognized key, not an extra key")

	data, err := json.Marshal(&c)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"scoring"`)
}
