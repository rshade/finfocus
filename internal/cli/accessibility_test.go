package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

func clearAccessibilityEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_PLAIN", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "")
	t.Setenv(agentModeEnv, "")
}

func TestAccessibilityFlags_OnOutputCommands(t *testing.T) {
	t.Parallel()

	commands := []*cobra.Command{
		NewCostProjectedCmd(),
		NewCostActualCmd(),
		NewCostRecommendationsCmd(),
		NewOverviewCmd(),
	}
	for _, cmd := range commands {
		assert.NotNil(t, cmd.Flags().Lookup("no-color"), cmd.Name())
		assert.NotNil(t, cmd.Flags().Lookup("plain"), cmd.Name())
		assert.NotNil(t, cmd.Flags().Lookup("color"), cmd.Name())
		assert.NotNil(t, cmd.Flags().Lookup("high-contrast"), cmd.Name())
	}
}

func TestAccessibilityFromCmd_PlainBeatsColor(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "1")

	cmd := NewCostProjectedCmd()
	require.NoError(t, cmd.Flags().Set("plain", "true"))
	require.NoError(t, cmd.Flags().Set("color", "true"))
	require.NoError(t, cmd.Flags().Set("high-contrast", "true"))

	got := accessibilityFromCmd(cmd)

	assert.True(t, got.Plain)
	assert.False(t, got.ForceColor)
	assert.False(t, got.HighContrast)
}

func TestAccessibilityFromCmd_ColorBeatsPlainEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_PLAIN", "1")
	t.Setenv("NO_COLOR", "1")

	cmd := NewCostProjectedCmd()
	require.NoError(t, cmd.Flags().Set("color", "true"))

	got := accessibilityFromCmd(cmd)

	assert.True(t, got.ForceColor)
	assert.False(t, got.Plain)
	assert.False(t, got.NoColor)
}

func TestAccessibilityFromCmd_HighContrastEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "true")

	cmd := NewCostActualCmd()
	got := accessibilityFromCmd(cmd)

	assert.True(t, got.HighContrast)
	assert.False(t, got.Plain)
	assert.False(t, got.ForceColor)
}

//nolint:paralleltest // clearAccessibilityEnv calls t.Setenv
func TestAccessibilityFromCmd_ForceColorAlias(t *testing.T) {
	clearAccessibilityEnv(t)

	cmd := NewOverviewCmd()
	require.NoError(t, cmd.Flags().Set("force-color", "true"))

	got := accessibilityFromCmd(cmd)

	assert.True(t, got.ForceColor)
	assert.False(t, got.Plain)
}

//nolint:paralleltest // clearAccessibilityEnv calls t.Setenv
func TestOutputModeFromCmd_PlainBeatsForceColor(t *testing.T) {
	clearAccessibilityEnv(t)

	cmd := NewCostProjectedCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Flags().Set("plain", "true"))
	require.NoError(t, cmd.Flags().Set("color", "true"))

	assert.Equal(t, tui.OutputModePlain, outputModeFromCmd(cmd))
}

//nolint:paralleltest // clearAccessibilityEnv calls t.Setenv
func TestOutputModeFromCmd_ForceColorStylesBuffer(t *testing.T) {
	clearAccessibilityEnv(t)

	cmd := NewCostRecommendationsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Flags().Set("color", "true"))

	assert.Equal(t, tui.OutputModeStyled, outputModeFromCmd(cmd))
}

func TestHistoryOutputFormat_PlainEnvFillsTheDefault(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_PLAIN", "1")

	format, err := historyOutputFormat(NewCostHistoryViewCmd())

	require.NoError(t, err)
	assert.Equal(t, historyOutputPlain, format)
}

func TestHistoryOutputFormat_PlainEnvDoesNotOverrideExplicitJSON(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_PLAIN", "1")

	cmd := NewCostHistoryViewCmd()
	require.NoError(t, cmd.Flags().Set("output", historyOutputJSON))

	got, err := historyOutputFormat(cmd)

	require.NoError(t, err)
	want := historyOutputJSON
	assert.Equal(t, want, got)
}

func TestHistoryOutputFormat_PlainEnvDoesNotOverrideMachineFormat(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_PLAIN", "1")
	t.Setenv(agentModeEnv, "json")

	got, err := historyOutputFormat(NewCostHistoryViewCmd())

	require.NoError(t, err)
	want := historyOutputJSON
	assert.Equal(t, want, got)
}

//nolint:paralleltest // clearAccessibilityEnv calls t.Setenv
func TestHistoryOutputFormat_PlainFlagStillBeatsJSON(t *testing.T) {
	clearAccessibilityEnv(t)

	cmd := NewCostHistoryViewCmd()
	require.NoError(t, cmd.Flags().Set("output", historyOutputJSON))
	require.NoError(t, cmd.Flags().Set("plain", "true"))

	format, err := historyOutputFormat(cmd)

	require.NoError(t, err)
	assert.Equal(t, historyOutputPlain, format)
}

//nolint:paralleltest // clearAccessibilityEnv calls t.Setenv
func TestHistoryOutputFormat_ExplicitJSONStaysJSON(t *testing.T) {
	clearAccessibilityEnv(t)

	cmd := NewCostHistoryViewCmd()
	require.NoError(t, cmd.Flags().Set("output", historyOutputJSON))
	require.NoError(t, cmd.Flags().Set("plain", "false"))

	got, err := historyOutputFormat(cmd)

	require.NoError(t, err)
	want := historyOutputJSON
	assert.Equal(t, want, got)
}

func TestRenderBudgetForAccess_PlainDropsColor(t *testing.T) {
	t.Parallel()

	status := sampleBudgetStatus()
	var buf bytes.Buffer
	err := renderBudgetForAccess(&buf, status, tui.Accessibility{
		Plain:        true,
		ForceColor:   true,
		HighContrast: true,
	})

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Status: [OK] Within budget")
	assert.Contains(t, output, "=============")
	assert.NotContains(t, output, "\x1b")
}

func TestRenderBudgetForAccess_HighContrastUsesBrightColors(t *testing.T) {
	t.Parallel()

	status := sampleBudgetStatus()
	var buf bytes.Buffer
	err := renderBudgetForAccess(&buf, status, tui.Accessibility{
		ForceColor:   true,
		HighContrast: true,
	})

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "38;5;46")
	assert.Contains(t, output, "38;5;231")
	assert.Contains(t, output, "[OK] Within budget")
}

func TestProgressColor_HighContrast(t *testing.T) {
	t.Parallel()

	pal := tui.Palette(true)
	assert.Equal(t, pal.OK, progressColor(50, true))
	assert.Equal(t, pal.Warning, progressColor(85, true))
	assert.Equal(t, pal.Critical, progressColor(110, true))
	assert.Equal(t, progressOKColor(), progressColor(50, false))
}

func sampleBudgetStatus() *engine.BudgetStatus {
	return &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:   1000,
			Currency: "USD",
			Period:   "monthly",
		},
		CurrentSpend: 500,
		Percentage:   50,
		Currency:     "USD",
	}
}
