package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2ETestConfig_LoadFromEnv(t *testing.T) {
	// Set environment variables using t.Setenv for automatic cleanup
	t.Setenv("FINFOCUS_E2E_AWS_REGION", "us-west-2")
	t.Setenv("FINFOCUS_E2E_TOLERANCE", "0.05")
	t.Setenv("FINFOCUS_E2E_TIMEOUT", "5m")

	config := LoadConfig()
	require.NotNil(t, config)

	assert.Equal(t, "us-west-2", config.AWSRegion)
	assert.Equal(t, 0.05, config.Tolerance)
	assert.Equal(t, 5*time.Minute, config.Timeout)
}

func TestE2ETestConfig_Defaults(t *testing.T) {
	// Unset the specific env vars we care about; t.Setenv registers the
	// original values so they are restored on cleanup.
	for _, key := range []string{"FINFOCUS_E2E_AWS_REGION", "FINFOCUS_E2E_TOLERANCE", "FINFOCUS_E2E_TIMEOUT"} {
		if value, ok := os.LookupEnv(key); ok {
			t.Setenv(key, value)
		}
		os.Unsetenv(key)
	}

	config := LoadConfig()
	require.NotNil(t, config)

	// Check defaults
	assert.Equal(t, "us-east-1", config.AWSRegion)
	assert.Equal(t, 0.05, config.Tolerance) // Default 5%
	assert.Equal(t, 10*time.Minute, config.Timeout)
}
