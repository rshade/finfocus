package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

func TestOutputModeFromCmd_ColorFlagBeatsNoColorEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("NO_COLOR", "1")

	cmd := NewCostActualCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Flags().Set("color", "true"))

	assert.Equal(t, tui.OutputModeStyled, outputModeFromCmd(cmd))
}

func TestOutputModeFromCmd_NoColorEnvStillPlainWithoutAFlag(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("NO_COLOR", "1")

	cmd := NewCostActualCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	assert.Equal(t, tui.OutputModePlain, outputModeFromCmd(cmd))
}

func sampleScopedResult() *engine.ScopedBudgetResult {
	return &engine.ScopedBudgetResult{
		Global: &engine.ScopedBudgetStatus{
			ScopeType:    engine.ScopeTypeGlobal,
			Budget:       config.ScopedBudget{Amount: 10000, Currency: "USD"},
			CurrentSpend: 5000,
			Percentage:   50,
			Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			Currency:     "USD",
		},
		ByProvider: map[string]*engine.ScopedBudgetStatus{
			"aws": {
				ScopeType:    engine.ScopeTypeProvider,
				ScopeKey:     "aws",
				Budget:       config.ScopedBudget{Amount: 5000, Currency: "USD"},
				CurrentSpend: 3500,
				Percentage:   70,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
	}
}

func TestRenderScopedBudgetStatus_FollowsAccessibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		access     tui.Accessibility
		wantANSI   bool
		wantInside []string
	}{
		{"a non-terminal writer is plain", tui.Accessibility{}, false, nil},
		{"plain drops color even when color is forced", tui.Accessibility{Plain: true, ForceColor: true}, false, nil},
		{
			"no-color drops color even when color is forced",
			tui.Accessibility{NoColor: true, ForceColor: true},
			false,
			nil,
		},
		{"color forces the styled box onto a non-terminal", tui.Accessibility{ForceColor: true}, true, nil},
		{
			"high contrast recolors the styled box",
			tui.Accessibility{ForceColor: true, HighContrast: true},
			true,
			[]string{"38;5;46", "38;5;231"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			require.NoError(t, RenderScopedBudgetStatus(&buf, sampleScopedResult(), nil, tt.access))

			output := buf.String()
			assert.Contains(t, output, "BUDGET STATUS")
			assert.Equal(t, tt.wantANSI, bytes.Contains(buf.Bytes(), []byte("\x1b")))
			for _, want := range tt.wantInside {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestRenderScopedBudgetStatus_DefaultPaletteIsNotHighContrast(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, RenderScopedBudgetStatus(&buf, sampleScopedResult(), nil, tui.Accessibility{ForceColor: true}))

	assert.NotContains(t, buf.String(), "38;5;231")
}

func TestCostProjectedAccessibilityFlagsDocumentTheirScope(t *testing.T) {
	t.Parallel()

	cmd := NewCostProjectedCmd()
	for _, name := range []string{"plain", "no-color", "color", "high-contrast"} {
		flag := cmd.Flags().Lookup(name)
		require.NotNil(t, flag, name)
		assert.Contains(t, flag.Usage, "budget", name)
	}
}
