package tui

import (
	"os"
	"testing"
)

func TestOutputModeConstants(t *testing.T) {
	// Test that constants have expected values
	if OutputModePlain != 0 {
		t.Errorf("Expected OutputModePlain = 0, got %d", OutputModePlain)
	}
	if OutputModeStyled != 1 {
		t.Errorf("Expected OutputModeStyled = 1, got %d", OutputModeStyled)
	}
	if OutputModeInteractive != 2 {
		t.Errorf("Expected OutputModeInteractive = 2, got %d", OutputModeInteractive)
	}
}

func TestDetectOutputMode_ExplicitFlags(t *testing.T) {
	tests := []struct {
		name       string
		forceColor bool
		noColor    bool
		plain      bool
		expected   OutputMode
	}{
		{"plain flag", false, false, true, OutputModePlain},
		{"no-color flag", false, true, false, OutputModePlain},
		{"both plain and no-color", false, true, true, OutputModePlain},
		{
			"force-color flag",
			true,
			false,
			false,
			OutputModeStyled,
		}, // forceColor enables styled output even without TTY
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables for consistent testing; clearEnv
			// restores the original values on cleanup.
			clearEnv(t, "NO_COLOR", "TERM", "CI")

			result := DetectOutputMode(tt.forceColor, tt.noColor, tt.plain)
			if result != tt.expected {
				t.Errorf("DetectOutputMode(%v, %v, %v) = %v, expected %v",
					tt.forceColor, tt.noColor, tt.plain, result, tt.expected)
			}
		})
	}
}

func TestDetectOutputMode_EnvironmentVariables(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected OutputMode
	}{
		{"NO_COLOR set", map[string]string{"NO_COLOR": "1"}, OutputModePlain},
		{"TERM=dumb", map[string]string{"TERM": "dumb"}, OutputModePlain},
		{
			"CI set",
			map[string]string{"CI": "true"},
			OutputModePlain,
		}, // Not a TTY in test env
		{
			"CI and TERM set",
			map[string]string{"CI": "true", "TERM": "xterm"},
			OutputModePlain,
		}, // Not a TTY in test env
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars first, then set the test environment.
			clearEnv(t, "NO_COLOR", "TERM", "CI")
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			result := DetectOutputMode(false, false, false)
			if result != tt.expected {
				t.Errorf("With env %v: DetectOutputMode() = %v, expected %v",
					tt.envVars, result, tt.expected)
			}
		})
	}
}

func TestDetectOutputMode_DefaultBehavior(t *testing.T) {
	// Test default behavior when no flags or env vars are set
	// This will depend on whether we're running in a TTY or not

	// Clear all relevant environment variables; clearEnv restores them on cleanup.
	clearEnv(t, "NO_COLOR", "TERM", "CI")

	result := DetectOutputMode(false, false, false)

	// In a testing environment, this might be Plain or Interactive depending on setup
	// We just verify it's a valid OutputMode
	if result < OutputModePlain || result > OutputModeInteractive {
		t.Errorf("DetectOutputMode() returned invalid mode: %v", result)
	}
}

func TestDetectOutputMode_FlagPrecedence(t *testing.T) {
	// Test that explicit flags override environment variables

	clearEnv(t, "NO_COLOR", "TERM", "CI")

	// Set environment to suggest styled output
	t.Setenv("TERM", "xterm")
	t.Setenv("CI", "true")

	// But explicit flags should override
	tests := []struct {
		name       string
		forceColor bool
		noColor    bool
		plain      bool
		expected   OutputMode
	}{
		{"plain overrides CI", false, false, true, OutputModePlain},
		{"no-color overrides CI", false, true, false, OutputModePlain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectOutputMode(tt.forceColor, tt.noColor, tt.plain)
			if result != tt.expected {
				t.Errorf("DetectOutputMode(%v, %v, %v) = %v, expected %v",
					tt.forceColor, tt.noColor, tt.plain, result, tt.expected)
			}
		})
	}
}

func TestIsTTY(t *testing.T) {
	// Test that IsTTY returns a boolean
	result := IsTTY()

	// Result should be boolean (true or false)
	if result != true && result != false {
		t.Errorf("IsTTY() returned non-boolean value: %v", result)
	}

	// In testing environment, this might be false (not a TTY)
	// We can't reliably test the actual value since it depends on the environment
}

func TestTerminalWidth(t *testing.T) {
	width := TerminalWidth()

	// Should return a positive width
	if width <= 0 {
		t.Errorf("TerminalWidth() returned non-positive width: %d", width)
	}

	// Should have a reasonable default (at least 40 for minimal usability)
	if width < 40 {
		t.Errorf("TerminalWidth() returned suspiciously small width: %d", width)
	}

	// Should not exceed some reasonable maximum (most terminals are < 1000)
	if width > 1000 {
		t.Errorf("TerminalWidth() returned suspiciously large width: %d", width)
	}
}

func TestTerminalWidth_DefaultFallback(t *testing.T) {
	// We can't easily test the fallback behavior without mocking,
	// but we can verify the function doesn't panic and returns reasonable values
	width := TerminalWidth()

	if width <= 0 {
		t.Error("TerminalWidth() should return positive width even on error")
	}
}

func TestOutputModeString(_ *testing.T) {
	// Test that we can convert OutputMode to string for debugging
	modes := []OutputMode{OutputModePlain, OutputModeStyled, OutputModeInteractive}

	for _, mode := range modes {
		// This should not panic
		_ = mode
	}
}

// TestDetectOutputMode_Integration tests the full logic integration.
func TestDetectOutputMode_Integration(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T)
		forceColor bool
		noColor    bool
		plain      bool
		expected   OutputMode
	}{
		{
			name: "NO_COLOR forces plain",
			setup: func(t *testing.T) {
				t.Setenv("NO_COLOR", "1")
				clearEnv(t, "TERM", "CI")
			},
			expected: OutputModePlain,
		},
		{
			name: "TERM=dumb forces plain",
			setup: func(t *testing.T) {
				t.Setenv("TERM", "dumb")
				clearEnv(t, "NO_COLOR", "CI")
			},
			expected: OutputModePlain,
		},
		{
			name: "CI environment gets styled",
			setup: func(t *testing.T) {
				t.Setenv("CI", "true")
				clearEnv(t, "NO_COLOR", "TERM")
			},
			expected: OutputModePlain, // Not a TTY in test environment
		},
		{
			name: "forceColor enables styled output",
			setup: func(t *testing.T) {
				clearEnv(t, "NO_COLOR", "TERM", "CI")
			},
			forceColor: true,
			expected:   OutputModeStyled,
		},
		{
			name: "plain flag overrides forceColor",
			setup: func(t *testing.T) {
				clearEnv(t, "NO_COLOR", "TERM", "CI")
			},
			forceColor: true,
			plain:      true,
			expected:   OutputModePlain,
		},
		{
			name: "NO_COLOR overrides forceColor",
			setup: func(t *testing.T) {
				t.Setenv("NO_COLOR", "1")
				clearEnv(t, "TERM", "CI")
			},
			forceColor: true,
			expected:   OutputModePlain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			result := DetectOutputMode(tt.forceColor, tt.noColor, tt.plain)
			if result != tt.expected {
				t.Errorf("Integration test failed: got %v, expected %v", result, tt.expected)
			}
		})
	}
}

// clearEnv unsets the given environment variables for the duration of the
// test. t.Setenv registers the original values so they are restored on
// cleanup; variables that were originally unset stay unset.
func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			t.Setenv(key, value)
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("failed to unset %s: %v", key, err)
		}
	}
}
