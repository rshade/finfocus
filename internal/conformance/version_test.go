package conformance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/conformance"
)

func TestCheckProtocolVersion_Matching(t *testing.T) {
	t.Parallel()

	err := conformance.CheckProtocolVersion("1.0", conformance.ProtocolVersion)

	require.NoError(t, err)
}

func TestCheckProtocolVersion_Mismatch(t *testing.T) {
	t.Parallel()

	err := conformance.CheckProtocolVersion("0.9", conformance.ProtocolVersion)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol version mismatch")
	assert.Contains(t, err.Error(), "0.9")
	assert.Contains(t, err.Error(), conformance.ProtocolVersion)
}

func TestCheckProtocolVersion_Empty(t *testing.T) {
	t.Parallel()

	err := conformance.CheckProtocolVersion("", conformance.ProtocolVersion)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol version")
}

func TestCheckProtocolVersion_Invalid(t *testing.T) {
	t.Parallel()

	err := conformance.CheckProtocolVersion("invalid", conformance.ProtocolVersion)

	require.Error(t, err)
}

func TestCheckProtocolVersion_MajorMinor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		plugin   string
		expected string
		wantErr  bool
	}{
		{"exact match", "1.0", "1.0", false},
		{"minor version ahead", "1.1", "1.0", false}, // Plugin supports newer minor
		{"major version ahead", "2.0", "1.0", true},  // Major version mismatch
		{"major version behind", "0.9", "1.0", true}, // Major version mismatch
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := conformance.CheckProtocolVersion(tc.plugin, tc.expected)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		version      string
		wantMajor    int
		wantMinor    int
		wantErr      bool
		errorContain string
	}{
		{"valid 1.0", "1.0", 1, 0, false, ""},
		{"valid 2.5", "2.5", 2, 5, false, ""},
		{"valid 10.20", "10.20", 10, 20, false, ""},
		{"empty string", "", 0, 0, true, "empty"},
		{"no minor", "1", 0, 0, true, "format"},
		{"too many parts", "1.2.3", 0, 0, true, "format"},
		{"invalid major", "x.0", 0, 0, true, "parse"},
		{"invalid minor", "1.x", 0, 0, true, "parse"},
		{"negative major", "-1.0", 0, 0, true, "negative"},
		{"negative minor", "1.-1", 0, 0, true, "negative"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			major, minor, err := conformance.ParseVersion(tc.version)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errorContain != "" {
					assert.Contains(t, err.Error(), tc.errorContain)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantMajor, major)
				assert.Equal(t, tc.wantMinor, minor)
			}
		})
	}
}

func TestIsCompatibleVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		pluginMajor    int
		pluginMinor    int
		expectedMajor  int
		expectedMinor  int
		wantCompatible bool
	}{
		{"exact match", 1, 0, 1, 0, true},
		{"plugin minor ahead", 1, 1, 1, 0, true},
		{"plugin minor behind", 1, 0, 1, 1, false},
		{"plugin major ahead", 2, 0, 1, 0, false},
		{"plugin major behind", 0, 9, 1, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compatible := conformance.IsCompatibleVersion(
				tc.pluginMajor, tc.pluginMinor,
				tc.expectedMajor, tc.expectedMinor,
			)
			assert.Equal(t, tc.wantCompatible, compatible)
		})
	}
}
