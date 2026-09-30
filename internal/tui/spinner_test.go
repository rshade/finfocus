package tui_test

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/tui"
)

func TestDefaultSpinner(t *testing.T) {
	t.Parallel()

	s := tui.DefaultSpinner()
	assert.Equal(t, spinner.Dot, s.Spinner)
	// Verify the spinner uses ColorInfo (ANSI color 33 - blue)
	assert.Equal(t, tui.ColorInfo, s.Style.GetForeground(), "DefaultSpinner should use ColorInfo")
}
