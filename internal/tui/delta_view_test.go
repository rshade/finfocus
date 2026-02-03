package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// TestRenderEstimateDelta tests the delta visualization component.
func TestRenderEstimateDelta(t *testing.T) {
	t.Run("renders positive delta with plus sign and up arrow", func(t *testing.T) {
		result := RenderEstimateDelta(74.90)

		assert.Contains(t, result, "+")
		assert.Contains(t, result, IconArrowUp)
		assert.Contains(t, result, "74.90")
	})

	t.Run("renders negative delta with down arrow", func(t *testing.T) {
		result := RenderEstimateDelta(-25.50)

		assert.NotContains(t, result, "+")
		assert.Contains(t, result, IconArrowDown)
		assert.Contains(t, result, "25.50")
	})

	t.Run("renders zero delta with right arrow", func(t *testing.T) {
		result := RenderEstimateDelta(0.0)

		assert.Contains(t, result, IconArrowRight)
		assert.Contains(t, result, "0.00")
	})

	t.Run("rounds small values correctly", func(t *testing.T) {
		// Values smaller than a cent should render as zero
		result := RenderEstimateDelta(0.001)

		assert.Contains(t, result, IconArrowRight)
	})
}

// TestRenderEstimateHeader tests the estimate header rendering.
func TestRenderEstimateHeader(t *testing.T) {
	t.Run("renders resource type and provider", func(t *testing.T) {
		result := RenderEstimateHeader("aws", "ec2:Instance", "i-123")

		assert.Contains(t, result, "What-If")
		assert.Contains(t, result, "aws")
		assert.Contains(t, result, "ec2:Instance")
	})

	t.Run("renders without ID when empty", func(t *testing.T) {
		result := RenderEstimateHeader("aws", "ec2:Instance", "")

		assert.Contains(t, result, "aws")
		assert.Contains(t, result, "ec2:Instance")
		assert.NotContains(t, result, "ID:")
	})
}

// TestRenderCostComparison tests the cost comparison rendering.
func TestRenderCostComparison(t *testing.T) {
	t.Run("renders baseline and modified costs", func(t *testing.T) {
		result := RenderCostComparison(8.32, 83.22, "USD")

		assert.Contains(t, result, "Baseline")
		assert.Contains(t, result, "8.32")
		assert.Contains(t, result, "Modified")
		assert.Contains(t, result, "83.22")
		assert.Contains(t, result, "USD")
	})

	t.Run("renders total change", func(t *testing.T) {
		result := RenderCostComparison(8.32, 83.22, "USD")

		assert.Contains(t, result, "Change")
		// Should show the delta
		assert.Contains(t, result, "74.90")
	})

	t.Run("renders negative change correctly", func(t *testing.T) {
		result := RenderCostComparison(100.00, 75.00, "USD")

		// Should show savings with down arrow (no "-" prefix for formatting)
		assert.Contains(t, result, IconArrowDown)
		assert.Contains(t, result, "25.00")
	})
}

// TestRenderPropertyTable tests the property table rendering.
func TestRenderPropertyTable(t *testing.T) {
	t.Run("renders property rows with deltas", func(t *testing.T) {
		properties := []PropertyRow{
			{
				Key:           "instanceType",
				OriginalValue: "t3.micro",
				CurrentValue:  "m5.large",
				CostDelta:     65.70,
			},
			{
				Key:           "volumeSize",
				OriginalValue: "8",
				CurrentValue:  "100",
				CostDelta:     9.20,
			},
		}

		result := RenderPropertyTable(properties, 0, false)

		assert.Contains(t, result, "instanceType")
		assert.Contains(t, result, "t3.micro")
		assert.Contains(t, result, "m5.large")
		assert.Contains(t, result, "volumeSize")
	})

	t.Run("highlights focused row", func(t *testing.T) {
		properties := []PropertyRow{
			{
				Key:           "instanceType",
				OriginalValue: "t3.micro",
				CurrentValue:  "m5.large",
				CostDelta:     65.70,
			},
		}

		result := RenderPropertyTable(properties, 0, false)

		// Verify focus indicator is present
		assert.Contains(t, result, "instanceType", "should contain property name")

		// The focused row should have the focus indicator "→ "
		// Note: lipgloss may add ANSI codes, so we strip them for reliable checking
		lines := strings.Split(result, "\n")
		var instanceTypeLine string
		for _, line := range lines {
			if strings.Contains(line, "instanceType") {
				instanceTypeLine = line
				break
			}
		}
		require.NotEmpty(t, instanceTypeLine, "should find instanceType line")

		// Strip ANSI codes for reliable indicator checking
		ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
		cleanLine := ansiRegex.ReplaceAllString(instanceTypeLine, "")
		assert.Contains(t, cleanLine, "→ ", "focused row should have → indicator")
	})

	t.Run("shows edit indicator when editing", func(t *testing.T) {
		properties := []PropertyRow{
			{
				Key:           "instanceType",
				OriginalValue: "t3.micro",
				CurrentValue:  "m5.large",
				CostDelta:     65.70,
			},
		}

		result := RenderPropertyTable(properties, 0, true)

		// Verify property name is present
		assert.Contains(t, result, "instanceType", "should contain property name")

		// The editing row should have the edit indicator "> "
		lines := strings.Split(result, "\n")
		var instanceTypeLine string
		for _, line := range lines {
			if strings.Contains(line, "instanceType") {
				instanceTypeLine = line
				break
			}
		}
		require.NotEmpty(t, instanceTypeLine, "should find instanceType line")

		// Strip ANSI codes for reliable indicator checking
		ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
		cleanLine := ansiRegex.ReplaceAllString(instanceTypeLine, "")
		assert.Contains(t, cleanLine, "> ", "editing row should have > indicator")
	})

	t.Run("handles empty properties", func(t *testing.T) {
		result := RenderPropertyTable([]PropertyRow{}, 0, false)

		assert.Contains(t, result, "No properties")
	})
}

// TestRenderEstimateResult tests the full estimate result rendering.
func TestRenderEstimateResult(t *testing.T) {
	t.Run("renders complete estimate result", func(t *testing.T) {
		result := &engine.EstimateResult{
			Resource: &engine.ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
				ID:       "i-123",
			},
			Baseline: &engine.CostResult{Monthly: 8.32, Currency: "USD"},
			Modified: &engine.CostResult{Monthly: 83.22, Currency: "USD"},
			Deltas: []engine.CostDelta{
				{
					Property:      "instanceType",
					OriginalValue: "t3.micro",
					NewValue:      "m5.large",
					CostChange:    74.90,
				},
			},
			UsedFallback: false,
		}

		rendered := RenderEstimateResultView(result, 80)

		require.NotEmpty(t, rendered)
		assert.Contains(t, rendered, "aws")
		assert.Contains(t, rendered, "ec2:Instance")
		assert.Contains(t, rendered, "Baseline")
		assert.Contains(t, rendered, "Modified")
	})

	t.Run("shows fallback note when used", func(t *testing.T) {
		result := &engine.EstimateResult{
			Resource: &engine.ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
			},
			Baseline:     &engine.CostResult{Monthly: 8.32, Currency: "USD"},
			Modified:     &engine.CostResult{Monthly: 83.22, Currency: "USD"},
			Deltas:       []engine.CostDelta{},
			UsedFallback: true,
		}

		rendered := RenderEstimateResultView(result, 80)

		assert.Contains(t, rendered, "fallback")
	})

	t.Run("handles nil result gracefully", func(t *testing.T) {
		rendered := RenderEstimateResultView(nil, 80)

		assert.Contains(t, rendered, "No")
	})
}

// TestRenderEstimateHelp tests the help text rendering.
func TestRenderEstimateHelp(t *testing.T) {
	t.Run("renders keyboard shortcuts", func(t *testing.T) {
		result := RenderEstimateHelp()

		assert.Contains(t, result, "↑/↓")
		assert.Contains(t, result, "Enter")
		assert.Contains(t, result, "Esc")
		assert.Contains(t, result, "q")
	})
}

// TestRenderLoadingIndicator tests the loading indicator.
func TestRenderLoadingIndicator(t *testing.T) {
	t.Run("renders calculating message", func(t *testing.T) {
		result := RenderLoadingIndicator()

		assert.Contains(t, result, "Calculating")
	})
}
