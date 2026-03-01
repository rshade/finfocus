package tui

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/stretchr/testify/assert"
)

func TestDefaultSpinner(t *testing.T) {
	s := DefaultSpinner()
	assert.Equal(t, spinner.Dot, s.Spinner)
	// Verify the spinner uses ColorInfo (ANSI color 33 - blue)
	assert.Equal(t, ColorInfo, s.Style.GetForeground(), "DefaultSpinner should use ColorInfo")
}
