package recorder

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment variables
	os.Unsetenv(EnvOutputDir)
	os.Unsetenv(EnvMockResponse)

	cfg := LoadConfig()

	assert.Equal(t, DefaultOutputDir, cfg.OutputDir)
	assert.Equal(t, DefaultMockResponse, cfg.MockResponse)
}

func TestLoadConfig_CustomOutputDir(t *testing.T) {
	customDir := "/tmp/my-recordings"
	t.Setenv(EnvOutputDir, customDir)

	cfg := LoadConfig()

	assert.Equal(t, customDir, cfg.OutputDir)
}

func TestLoadConfig_MockResponseTrue(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{"lowercase true", "true"},
		{"uppercase TRUE", "TRUE"},
		{"mixed case True", "True"},
		{"numeric 1", "1"},
		{"yes", "yes"},
		{"YES", "YES"},
		{"on", "on"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvMockResponse, tc.value)

			cfg := LoadConfig()

			assert.True(t, cfg.MockResponse, "expected MockResponse=true for value %q", tc.value)
		})
	}
}

func TestLoadConfig_MockResponseFalse(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{"lowercase false", "false"},
		{"uppercase FALSE", "FALSE"},
		{"numeric 0", "0"},
		{"empty string", ""},
		{"no", "no"},
		{"off", "off"},
		{"invalid", "invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == "" {
				os.Unsetenv(EnvMockResponse)
			} else {
				t.Setenv(EnvMockResponse, tc.value)
			}

			cfg := LoadConfig()

			assert.False(t, cfg.MockResponse, "expected MockResponse=false for value %q", tc.value)
		})
	}
}

func TestLoadConfig_WithWhitespace(t *testing.T) {
	t.Setenv(EnvMockResponse, "  true  ")

	cfg := LoadConfig()

	assert.True(t, cfg.MockResponse)
}

func TestParseBool(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"anything", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := parseBool(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConfigConstants(t *testing.T) {
	// Verify constants are defined correctly
	require.Equal(t, "FINFOCUS_RECORDER_OUTPUT_DIR", EnvOutputDir)
	require.Equal(t, "FINFOCUS_RECORDER_MOCK_RESPONSE", EnvMockResponse)
	require.Equal(t, "./recorded_data", DefaultOutputDir)
	require.False(t, DefaultMockResponse)
}
