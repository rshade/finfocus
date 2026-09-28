package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersionConstraint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"greater than or equal", ">=1.0.0", false},
		{"less than", "<2.0.0", false},
		{"range", ">=1.0.0,<2.0.0", false},
		{"tilde", "~1.2.3", false},
		{"caret", "^1.2.3", false},
		{"empty", "", true},
		{"invalid", "not-a-version", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVersionConstraint(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSatisfiesConstraint(t *testing.T) {
	constraint, err := ParseVersionConstraint(">=1.0.0,<2.0.0")
	require.NoError(t, err)

	tests := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"1.5.0", true},
		{"v1.9.9", true},
		{"0.9.0", false},
		{"2.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := SatisfiesConstraint(tt.version, constraint)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "SatisfiesConstraint(%q)", tt.version)
		})
	}
}

func TestSatisfiesConstraintInvalidVersion(t *testing.T) {
	constraint, err := ParseVersionConstraint(">=1.0.0")
	require.NoError(t, err)
	_, err = SatisfiesConstraint("invalid", constraint)
	require.Error(t, err, "expected error for invalid version")
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"v1.0.0", "1.0.0", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.0.0-alpha", "1.0.0", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+" vs "+tt.v2, func(t *testing.T) {
			got, err := CompareVersions(tt.v1, tt.v2)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "CompareVersions(%q, %q)", tt.v1, tt.v2)
		})
	}
}

func TestCompareVersionsInvalid(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
	}{
		{"invalid v1", "invalid", "1.0.0"},
		{"invalid v2", "1.0.0", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CompareVersions(tt.v1, tt.v2)
			require.Error(t, err, "expected error for invalid version")
		})
	}
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"v1.0.0", true},
		{"1.2.3-beta", true},
		{"1.2.3-beta.1", true},
		{"1.2.3+build", true},
		{"invalid", false},
		{"", false},
		{"1", true},   // semver coerces to 1.0.0
		{"1.2", true}, // semver coerces to 1.2.0
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidVersion(tt.version), "IsValidVersion(%q)", tt.version)
		})
	}
}

func TestCanonicalVersion(t *testing.T) {
	tests := []struct {
		name, tag, prefix, want string
	}{
		{"no prefix passes through", "v1.2.3", "", "v1.2.3"},
		{"prefix stripped", "kubernetes-v0.1.0", "kubernetes-", "v0.1.0"},
		{"prefix absent leaves tag", "v0.3.7", "kubernetes-", "v0.3.7"},
		{"other plugin prefix untouched", "prometheus-v0.1.0", "kubernetes-", "prometheus-v0.1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CanonicalVersion(tt.tag, tt.prefix))
		})
	}
}

func TestReleaseTag(t *testing.T) {
	tests := []struct {
		name, version, prefix, want string
	}{
		{"no prefix passes through", "v1.0.0", "", "v1.0.0"},
		{"bare v version gets prefix", "v0.1.0", "kubernetes-", "kubernetes-v0.1.0"},
		{"bare semver gets v and prefix", "0.1.0", "kubernetes-", "kubernetes-v0.1.0"},
		{"already prefixed kept", "kubernetes-v0.1.0", "kubernetes-", "kubernetes-v0.1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ReleaseTag(tt.version, tt.prefix))
		})
	}
}

func TestHintsForEntry(t *testing.T) {
	assert.Nil(t, HintsForEntry(&RegistryEntry{Name: "x"}))

	h := HintsForEntry(&RegistryEntry{Name: "kubernetes", TagPrefix: "kubernetes-"})
	require.NotNil(t, h)
	assert.Equal(t, "kubernetes-", h.TagPrefix)

	h = HintsForEntry(&RegistryEntry{
		Name:       "aws-public",
		AssetHints: &RegistryAssetHints{AssetPrefix: "finfocus-plugin-aws-public", DefaultRegion: "us-east-1"},
	})
	require.NotNil(t, h)
	assert.Equal(t, "finfocus-plugin-aws-public", h.AssetPrefix)
	assert.Equal(t, "us-east-1", h.Region)
	assert.Empty(t, h.TagPrefix)
}

func TestTagPrefixOf(t *testing.T) {
	assert.Empty(t, tagPrefixOf(nil))
	assert.Equal(t, "kubernetes-", tagPrefixOf(&AssetNamingHints{TagPrefix: "kubernetes-"}))
}
