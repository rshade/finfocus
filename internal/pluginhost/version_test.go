package pluginhost

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareSpecVersions(t *testing.T) {
	tests := []struct {
		name          string
		coreVersion   string
		pluginVersion string
		wantResult    CompatibilityResult
		wantErr       bool
	}{
		{
			name:          "Same version",
			coreVersion:   "0.4.14",
			pluginVersion: "0.4.14",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "Compatible minor update",
			coreVersion:   "0.4.14",
			pluginVersion: "0.4.15",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "Minor difference returns Compatible",
			coreVersion:   "0.4.14",
			pluginVersion: "0.5.0",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "Core newer minor (backward compatible)",
			coreVersion:   "0.4.14",
			pluginVersion: "0.4.10",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "Major mismatch (plugin newer)",
			coreVersion:   "0.4.14",
			pluginVersion: "1.0.0",
			wantResult:    MajorMismatch,
			wantErr:       false,
		},
		{
			name:          "Major mismatch (core newer)",
			coreVersion:   "1.0.0",
			pluginVersion: "0.4.14",
			wantResult:    MajorMismatch,
			wantErr:       false,
		},
		{
			name:          "Invalid core version",
			coreVersion:   "invalid",
			pluginVersion: "0.4.14",
			wantResult:    Invalid,
			wantErr:       true,
		},
		{
			name:          "Invalid plugin version",
			coreVersion:   "0.4.14",
			pluginVersion: "invalid",
			wantResult:    Invalid,
			wantErr:       true,
		},
		{
			name:          "Empty core version",
			coreVersion:   "",
			pluginVersion: "0.4.14",
			wantResult:    Invalid,
			wantErr:       true,
		},
		{
			name:          "Empty plugin version",
			coreVersion:   "0.4.14",
			pluginVersion: "",
			wantResult:    Invalid,
			wantErr:       true,
		},
		{
			name:          "V-prefixed versions (both)",
			coreVersion:   "v0.4.14",
			pluginVersion: "v0.4.14",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "V-prefixed core version",
			coreVersion:   "v0.4.14",
			pluginVersion: "0.4.14",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "V-prefixed plugin version",
			coreVersion:   "0.4.14",
			pluginVersion: "v0.4.14",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "Pre-release core version",
			coreVersion:   "0.5.0-alpha.1",
			pluginVersion: "0.5.0",
			wantResult:    Compatible,
			wantErr:       false,
		},
		{
			name:          "Pre-release plugin version",
			coreVersion:   "0.5.0",
			pluginVersion: "0.5.0-alpha.1",
			wantResult:    Compatible,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareSpecVersions(tt.coreVersion, tt.pluginVersion)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantResult, got)
		})
	}
}

func TestCompatibilityResult_String(t *testing.T) {
	tests := []struct {
		name string
		r    CompatibilityResult
		want string
	}{
		{"Compatible", Compatible, "Compatible"},
		{"MajorMismatch", MajorMismatch, "MajorMismatch"},
		{"Invalid", Invalid, "Invalid"},
		{"Unknown", CompatibilityResult(99), "CompatibilityResult(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.r.String())
		})
	}
}

func TestCheckVersionCompatibility_StrictMode(t *testing.T) {
	ctx := context.Background()

	t.Run("parse error in permissive mode returns nil", func(t *testing.T) {
		// Ensure strict mode is off
		os.Unsetenv("FINFOCUS_STRICT_COMPATIBILITY")

		err := checkVersionCompatibility(ctx, "test-plugin", "invalid-version")
		assert.NoError(t, err, "permissive mode should not error on parse failures")
	})

	t.Run("parse error in strict mode returns error", func(t *testing.T) {
		// Enable strict mode
		t.Setenv("FINFOCUS_STRICT_COMPATIBILITY", "true")

		err := checkVersionCompatibility(ctx, "test-plugin", "invalid-version")
		require.Error(t, err, "strict mode should error on parse failures")
		assert.ErrorIs(t, err, ErrPluginIncompatible)
		assert.Contains(t, err.Error(), "failed to parse spec version")
		assert.Contains(t, err.Error(), "test-plugin")
	})

	t.Run("valid version with strict mode passes", func(t *testing.T) {
		// Enable strict mode
		t.Setenv("FINFOCUS_STRICT_COMPATIBILITY", "true")

		err := checkVersionCompatibility(ctx, "test-plugin", "0.4.14")
		assert.NoError(t, err, "valid version should pass in strict mode")
	})
}
