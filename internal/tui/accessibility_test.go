package tui_test

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/tui"
)

func clearAccessibilityEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_PLAIN", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "")
}

func TestResolveAccessibility_EnvFillsUnset(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "true")

	got := tui.ResolveAccessibility(tui.Accessibility{}, tui.Accessibility{})

	assert.True(t, got.ForceColor)
	assert.True(t, got.HighContrast)
	assert.False(t, got.Plain)
	assert.False(t, got.NoColor)
}

func TestResolveAccessibility_PlainEnvBlocksColorEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_PLAIN", "1")
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "1")

	got := tui.ResolveAccessibility(tui.Accessibility{}, tui.Accessibility{})

	assert.True(t, got.Plain)
	assert.False(t, got.ForceColor)
	assert.False(t, got.HighContrast)
}

func TestResolveAccessibility_NoColorEnvBlocksColorEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "true")
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "true")

	got := tui.ResolveAccessibility(tui.Accessibility{}, tui.Accessibility{})

	assert.True(t, got.NoColor)
	assert.False(t, got.ForceColor)
	assert.False(t, got.HighContrast)
}

//nolint:paralleltest // clearAccessibilityEnv calls t.Setenv
func TestResolveAccessibility_ExplicitPlainClearsColorFlags(t *testing.T) {
	clearAccessibilityEnv(t)
	flags := tui.Accessibility{Plain: true, ForceColor: true, HighContrast: true}

	got := tui.ResolveAccessibility(flags, flags)

	assert.True(t, got.Plain)
	assert.False(t, got.ForceColor)
	assert.False(t, got.HighContrast)
}

func TestResolveAccessibility_ExplicitColorBeatsPlainEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_PLAIN", "1")
	t.Setenv("NO_COLOR", "1")
	flags := tui.Accessibility{ForceColor: true}
	explicit := tui.Accessibility{ForceColor: true}

	got := tui.ResolveAccessibility(flags, explicit)

	assert.True(t, got.ForceColor)
	assert.False(t, got.Plain)
	assert.False(t, got.NoColor)
}

func TestResolveAccessibility_ExplicitNoColorBeatsForceEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FORCE_COLOR", "true")
	t.Setenv("FINFOCUS_HIGH_CONTRAST", "true")
	flags := tui.Accessibility{NoColor: true}
	explicit := tui.Accessibility{NoColor: true}

	got := tui.ResolveAccessibility(flags, explicit)

	assert.True(t, got.NoColor)
	assert.False(t, got.ForceColor)
	assert.False(t, got.HighContrast)
}

func TestResolveAccessibility_InvalidEnvIgnored(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FORCE_COLOR", "always")
	t.Setenv("FINFOCUS_PLAIN", "yes")

	got := tui.ResolveAccessibility(tui.Accessibility{}, tui.Accessibility{})

	assert.False(t, got.ForceColor)
	assert.False(t, got.Plain)
	assert.False(t, got.HighContrast)
}

func TestResolveAccessibility_ExplicitFalseBlocksEnv(t *testing.T) {
	clearAccessibilityEnv(t)
	t.Setenv("FINFOCUS_PLAIN", "1")

	got := tui.ResolveAccessibility(tui.Accessibility{}, tui.Accessibility{Plain: true})

	assert.False(t, got.Plain)
}

func TestPalette_HighContrast(t *testing.T) {
	t.Parallel()

	pal := tui.Palette(true)

	assert.Equal(t, lipgloss.Color("46"), pal.OK)
	assert.Equal(t, lipgloss.Color("226"), pal.Warning)
	assert.Equal(t, lipgloss.Color("196"), pal.Critical)
	assert.Equal(t, lipgloss.Color("231"), pal.Header)
}

func TestPalette_Defaults(t *testing.T) {
	t.Parallel()

	pal := tui.Palette(false)

	assert.Equal(t, tui.ColorOK, pal.OK)
	assert.Equal(t, tui.ColorWarning, pal.Warning)
	assert.Equal(t, tui.ColorCritical, pal.Critical)
	assert.Equal(t, tui.ColorHeader, pal.Header)
}

func TestStatusIndicator(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "[OK] ✓", tui.StatusIndicator("ok", true))
	assert.Equal(t, "✓ OK", tui.StatusIndicator("OK", false))
	assert.Equal(t, "[WARNING] ⚠", tui.StatusIndicator("warning", true))
	assert.Equal(t, "⚠ WARNING", tui.StatusIndicator("WARNING", false))
	assert.Equal(t, "[CRITICAL] 🚨", tui.StatusIndicator("critical", true))
	assert.Equal(t, "🚨 CRITICAL", tui.StatusIndicator("exceeded", false))
	assert.Equal(t, "[ERROR] 🚨", tui.StatusIndicator("error", true))
	assert.Equal(t, "[PENDING] ○", tui.StatusIndicator("pending", true))
	assert.Equal(t, "○ PENDING", tui.StatusIndicator("", false))
}

func TestForceColorEnvLevels(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"1", true},
		{"2", true},
		{"3", true},
		{"true", true},
		{"TRUE", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"yes", false},
	}

	for _, tt := range tests {
		t.Run("FORCE_COLOR="+tt.value, func(t *testing.T) {
			t.Setenv("FINFOCUS_PLAIN", "")
			t.Setenv("NO_COLOR", "")
			t.Setenv("FINFOCUS_HIGH_CONTRAST", "")
			t.Setenv("FORCE_COLOR", tt.value)

			got := tui.ResolveAccessibility(tui.Accessibility{}, tui.Accessibility{})

			assert.Equal(t, tt.want, got.ForceColor)
		})
	}
}
