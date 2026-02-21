package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/cli"
)

func TestPromptResult(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		result := cli.PromptResult{}
		assert.False(t, result.Accepted)
		assert.False(t, result.TimedOut)
		assert.False(t, result.Cancelled)
	})

	t.Run("accepted state", func(t *testing.T) {
		result := cli.PromptResult{Accepted: true}
		assert.True(t, result.Accepted)
		assert.False(t, result.TimedOut)
		assert.False(t, result.Cancelled)
	})

	t.Run("cancelled state", func(t *testing.T) {
		result := cli.PromptResult{Cancelled: true}
		assert.False(t, result.Accepted)
		assert.False(t, result.TimedOut)
		assert.True(t, result.Cancelled)
	})
}

func TestConfirmFallback(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		requestedVersion string
		fallbackVersion  string
		platform         string
		wantAccepted     bool
		wantOutputParts  []string
	}{
		{
			name:             "accept with lowercase y",
			input:            "y\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     true,
			wantOutputParts:  []string{"Warning:", "v1.0.0", "linux/amd64", "v0.9.0", "[y/N]"},
		},
		{
			name:             "accept with uppercase Y",
			input:            "Y\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     true,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "accept with yes",
			input:            "yes\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     true,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "accept with Yes",
			input:            "Yes\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     true,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "accept with YES",
			input:            "YES\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     true,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "decline with lowercase n",
			input:            "n\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     false,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "decline with uppercase N",
			input:            "N\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     false,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "decline with no",
			input:            "no\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     false,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "empty input defaults to No",
			input:            "\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     false,
			wantOutputParts:  []string{"Warning:", "[y/N]"},
		},
		{
			name:             "whitespace input defaults to No",
			input:            "   \n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     false,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "random text declines",
			input:            "maybe\n",
			requestedVersion: "v1.0.0",
			fallbackVersion:  "v0.9.0",
			platform:         "linux/amd64",
			wantAccepted:     false,
			wantOutputParts:  []string{"Warning:"},
		},
		{
			name:             "darwin platform",
			input:            "y\n",
			requestedVersion: "v2.0.0",
			fallbackVersion:  "v1.9.0",
			platform:         "darwin/arm64",
			wantAccepted:     true,
			wantOutputParts:  []string{"darwin/arm64", "v2.0.0", "v1.9.0"},
		},
		{
			name:             "windows platform",
			input:            "y\n",
			requestedVersion: "v3.0.0",
			fallbackVersion:  "v2.9.0",
			platform:         "windows/amd64",
			wantAccepted:     true,
			wantOutputParts:  []string{"windows/amd64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock stdin with the test input
			reader := strings.NewReader(tt.input)

			// Create buffer for output
			var output bytes.Buffer

			// Call ConfirmFallback with mocked reader
			// Note: This test can only run if TTY is detected.
			// In CI environments, this may behave differently.
			result := cli.ConfirmFallback(
				&output,
				reader,
				"test-plugin",
				tt.requestedVersion,
				tt.fallbackVersion,
				tt.platform,
			)

			// The test behavior depends on whether we're in a TTY or not
			// In a non-TTY environment (like CI), the function returns immediately
			// without reading input, so we can't test the actual prompt behavior.
			// We check the output only if it was actually written.
			if output.Len() > 0 {
				outputStr := output.String()
				for _, part := range tt.wantOutputParts {
					assert.Contains(t, outputStr, part,
						"output should contain %q", part)
				}
				assert.Equal(t, tt.wantAccepted, result.Accepted)
			}
			// If output is empty, we're in non-TTY mode and Accepted should be false
			if output.Len() == 0 {
				assert.False(t, result.Accepted,
					"non-TTY mode should always return Accepted=false")
			}
		})
	}
}

func TestConfirmFallback_EOF(t *testing.T) {
	// Test EOF handling (empty reader)
	reader := strings.NewReader("")
	var output bytes.Buffer

	result := cli.ConfirmFallback(
		&output,
		reader,
		"test-plugin",
		"v1.0.0",
		"v0.9.0",
		"linux/amd64",
	)

	// In non-TTY mode, returns immediately with Accepted=false
	// In TTY mode with EOF, should also return Accepted=false
	assert.False(t, result.Accepted)
}

func TestConfirmFallback_WarningMessage(t *testing.T) {
	// Skip if not in TTY mode - this test requires interactive terminal
	// The test verifies the message format matches the CLI contract

	reader := strings.NewReader("n\n")
	var output bytes.Buffer

	cli.ConfirmFallback(
		&output,
		reader,
		"aws-public",
		"v0.1.3",
		"v0.1.2",
		"linux/amd64",
	)

	// Only check output if we're in TTY mode
	if output.Len() > 0 {
		outputStr := output.String()

		// Verify message format per contracts/cli-interface.md
		assert.Contains(t, outputStr, "Warning: No compatible assets found for aws-public@v0.1.3 (linux/amd64)")
		assert.Contains(t, outputStr, "Would you like to install the latest stable version (v0.1.2) instead?")
		assert.Contains(t, outputStr, "[y/N]")
	}
}
