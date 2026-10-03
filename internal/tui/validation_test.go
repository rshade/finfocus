package tui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/tui"
)

func TestRenderValidationReport_PlainAndStyled(t *testing.T) {
	t.Parallel()
	view := tui.ValidationView{
		File:  "config.hujson",
		Valid: true,
		Warnings: []tui.ValidationViewItem{{
			Line:       5,
			Path:       "cost.budgets.global.ammount",
			Message:    "Unknown field 'ammount'",
			Suggestion: "Did you mean: 'amount'?",
		}},
	}

	plain := tui.RenderValidationReport(view, true)
	assert.Contains(t, plain, "Configuration Validation")
	assert.Contains(t, plain, "File: config.hujson")
	assert.Contains(t, plain, "Warning at line 5: Unknown field 'ammount'")
	assert.Contains(t, plain, "Did you mean: 'amount'?")
	assert.NotContains(t, plain, "╭")

	styled := tui.RenderValidationReport(view, false)
	assert.Contains(t, styled, "Configuration Validation")
	assert.Contains(t, styled, "Unknown field 'ammount'")
	assert.Contains(t, styled, "╭")
	assert.Greater(t, strings.Count(styled, "\n"), 1)

	invalid := tui.RenderValidationReport(tui.ValidationView{
		File:  "config.hujson",
		Valid: false,
		Errors: []tui.ValidationViewItem{{
			Line:    4,
			Path:    "cost.budgets.global.period",
			Message: "budget period must be 'monthly'",
			Hint:    "The only supported period is monthly.",
			Example: "period: monthly",
		}},
	}, true)
	assert.Contains(t, invalid, "Configuration Error: config.hujson")
	assert.Contains(t, invalid, "Error at line 4:")
	assert.Contains(t, invalid, "Hint: The only supported period is monthly.")
	assert.Contains(t, invalid, "period: monthly")
}
