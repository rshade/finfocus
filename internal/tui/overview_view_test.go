package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOverviewView_InitializingRender verifies the initializing view output.
func TestOverviewView_InitializingRender(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0)
	assert.Equal(t, ViewStateInitializing, model.state)

	// Set a phase message
	model.progressMsg = "Loading stack state..."

	output := model.View()
	assert.Contains(t, output, "Loading stack state...")
	// Should not contain table elements
	assert.NotContains(t, output, "Resource")
	assert.NotContains(t, output, "Type")
}

// TestOverviewView_InitializingDefaultMsg verifies default message when progressMsg is empty.
func TestOverviewView_InitializingDefaultMsg(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0)
	model.progressMsg = ""

	output := model.View()
	assert.Contains(t, output, "Initializing...")
}

// TestOverviewView_ErrorStateRender verifies the error view output.
func TestOverviewView_ErrorStateRender(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0)
	model.state = ViewStateError
	model.err = assert.AnError

	output := model.View()
	assert.Contains(t, output, assert.AnError.Error())
}

// TestOverviewView_InitializingRender_UsesLipglossWidth asserts that the longest line
// in the output is >= model.width - borderPadding, proving lipgloss Width() padding is applied.
func TestOverviewView_InitializingRender_UsesLipglossWidth(t *testing.T) {
	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, nil, 0)
	model.progressMsg = "test"

	output := model.renderInitializingView()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}
	assert.GreaterOrEqual(t, maxLen, model.width-borderPadding,
		"expected output to be padded to width by lipgloss")
}

// TestOverviewView_InitializingRender_NilLoadingState verifies nil safety when
// loadingState is nil: no panic and default message is present.
func TestOverviewView_InitializingRender_NilLoadingState(t *testing.T) {
	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, nil, 0)
	model.loadingState = nil

	require.NotPanics(t, func() {
		output := model.renderInitializingView()
		assert.Contains(t, output, "Initializing...")
	})
}
