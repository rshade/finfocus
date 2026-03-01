package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultProgressBar(t *testing.T) {
	pb := DefaultProgressBar()

	if pb.Width != DefaultProgressBarWidth {
		t.Errorf("Expected Width %d, got %d", DefaultProgressBarWidth, pb.Width)
	}
	if pb.Filled != "█" {
		t.Errorf("Expected Filled '█', got %q", pb.Filled)
	}
	if pb.Empty != "░" {
		t.Errorf("Expected Empty '░', got %q", pb.Empty)
	}
	if !pb.ShowPct {
		t.Error("Expected ShowPct true")
	}
}

func TestProgressBarRender_Clamping(t *testing.T) {
	pb := ProgressBar{Width: 10, Filled: "█", Empty: "░", ShowPct: true}

	// Test negative percentage clamped to 0
	result := pb.Render(-10)
	if !strings.Contains(result, "0%") {
		t.Errorf("Expected 0%% for negative input, got %q", result)
	}

	// Test percentage > 100 clamped to 100
	result = pb.Render(150)
	if !strings.Contains(result, "100%") {
		t.Errorf("Expected 100%% for >100 input, got %q", result)
	}
}

func TestProgressBarRender_BarWidth(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		percent  float64
		expected int // expected filled characters
	}{
		{"0%", 10, 0, 0},
		{"25%", 10, 25, 2},    // 25/100 * 10 = 2.5, should be 2
		{"50%", 10, 50, 5},    // 50/100 * 10 = 5
		{"75%", 10, 75, 7},    // 75/100 * 10 = 7.5, should be 7
		{"100%", 10, 100, 10}, // 100/100 * 10 = 10
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := ProgressBar{Width: tt.width, Filled: "█", Empty: "░", ShowPct: false}
			result := pb.Render(tt.percent)

			filledCount := strings.Count(result, "█")
			if filledCount != tt.expected {
				t.Errorf(
					"Expected %d filled characters, got %d. Result: %q",
					tt.expected,
					filledCount,
					result,
				)
			}

			totalChars := strings.Count(result, "█") + strings.Count(result, "░")
			if totalChars != tt.width {
				t.Errorf(
					"Expected total bar width %d, got %d. Result: %q",
					tt.width,
					totalChars,
					result,
				)
			}
		})
	}
}

func TestProgressBarRender_CustomCharacters(t *testing.T) {
	pb := ProgressBar{Width: 5, Filled: "■", Empty: "□", ShowPct: false}

	result := pb.Render(60) // 60% of 5 = 3 filled

	// Result includes ANSI color codes from lipgloss styling
	assert.Contains(t, result, "■■■□□")
}

func TestProgressBarRender_PercentageDisplay(t *testing.T) {
	tests := []struct {
		name     string
		showPct  bool
		percent  float64
		contains string
	}{
		{"Show percentage", true, 42.7, "43%"}, // Should round to nearest int
		{"Hide percentage", false, 75.0, ""},   // Should not contain percentage
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := ProgressBar{Width: 10, Filled: "█", Empty: "░", ShowPct: tt.showPct}
			result := pb.Render(tt.percent)

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("Expected result to contain %q, got %q", tt.contains, result)
			}
			if tt.contains == "" && strings.Contains(result, "%") {
				t.Errorf("Expected no percentage in result, got %q", result)
			}
		})
	}
}

func TestProgressBarRender_ColorCoding(t *testing.T) {
	tests := []struct {
		name        string
		percent     float64
		expectColor bool // We can't easily test ANSI colors, so just verify rendering works
	}{
		{"Green (< 80%)", 50, true},
		{"Orange (80-99%)", 85, true},
		{"Red (>= 100%)", 100, true},
		{"Red (> 100%)", 120, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := ProgressBar{Width: 10, Filled: "█", Empty: "░", ShowPct: true}
			result := pb.Render(tt.percent)

			// Basic validation that rendering produces output
			if result == "" {
				t.Error("Expected non-empty result")
			}

			// Should contain the bar characters
			if !strings.Contains(result, "█") && !strings.Contains(result, "░") {
				t.Errorf("Expected bar characters in result: %q", result)
			}

			// Should contain percentage if expected
			if tt.expectColor && !strings.Contains(result, "%") {
				t.Errorf("Expected percentage in result: %q", result)
			}
		})
	}
}

func TestProgressBarRender_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		percent float64
	}{
		{"Width 1", 1, 50},
		{"Width 0", 0, 50}, // Should handle gracefully
		{"Width 100", 100, 50},
		{"Percent 0.5", 10, 0.5},
		{"Percent 99.9", 10, 99.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := ProgressBar{Width: tt.width, Filled: "█", Empty: "░", ShowPct: false}

			// Should not panic
			result := pb.Render(tt.percent)

			// Should produce some output
			if result == "" && tt.width > 0 {
				t.Error("Expected non-empty result for width > 0")
			}
		})
	}
}
