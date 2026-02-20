package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

// TestOverviewView_InitializingRender verifies the initializing view output.
func TestOverviewView_InitializingRender(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	assert.Equal(t, ViewStateInitializing, model.state)

	// Set a phase message
	model.progressMsg = "Loading stack state..."

	output := model.View()
	assert.Contains(t, output, "Loading stack state...")
	// Should not contain table elements
	assert.NotContains(t, output, "Resource")
	assert.NotContains(t, output, "Type")
}

// TestOverviewView_InitializingDefaultMsg verifies the phase checklist is shown when progressMsg is empty.
func TestOverviewView_InitializingDefaultMsg(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.progressMsg = ""

	output := model.View()
	// Phase checklist is always shown; first phase is always "Loading stack state..."
	assert.Contains(t, output, "Loading stack state")
}

// TestOverviewView_ErrorStateRender verifies the error view output.
func TestOverviewView_ErrorStateRender(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.state = ViewStateError
	model.err = assert.AnError

	output := model.View()
	assert.Contains(t, output, assert.AnError.Error())
}

// TestOverviewView_InitializingRender_UsesLipglossWidth asserts that the longest line
// in the output is >= model.width - borderPadding, proving lipgloss Width() padding is applied.
func TestOverviewView_InitializingRender_UsesLipglossWidth(t *testing.T) {
	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
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
// loadingState is nil: no panic and phase checklist is present.
func TestOverviewView_InitializingRender_NilLoadingState(t *testing.T) {
	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.loadingState = nil

	output := model.renderInitializingView()
	assert.Contains(t, output, "Loading stack state")
}

// TestRenderStatusBar_StateOnlyShowsPHint verifies "[p] load pending changes" hint in state-only mode.
func TestRenderStatusBar_StateOnlyShowsPHint(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.state = ViewStateList
	model.isStateOnly = true

	output := model.renderStatusBar()
	assert.Contains(t, output, "[p] load pending changes")
}

// TestRenderStatusBar_LoadingShowsElapsed verifies elapsed timer shown while preview loads.
func TestRenderStatusBar_LoadingShowsElapsed(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.state = ViewStateList
	model.isPreviewLoading = true
	model.previewElapsed = 42 * time.Second

	output := model.renderStatusBar()
	assert.Contains(t, output, "Loading pending changes")
	assert.Contains(t, output, "0:42", "elapsed should be formatted as M:SS")
}

// TestRenderStatusBar_LoadedShowsNormal verifies normal status bar after preview loaded.
func TestRenderStatusBar_LoadedShowsNormal(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.state = ViewStateList
	model.previewLoaded = true

	output := model.renderStatusBar()
	assert.NotContains(t, output, "[p] load pending changes")
	assert.NotContains(t, output, "Loading pending changes")
}

// TestRenderFootnote_StateOnlyShowsAsterisk verifies footnote visible in state-only mode.
func TestRenderFootnote_StateOnlyShowsAsterisk(t *testing.T) {
	ctx := context.Background()
	rows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateList
	model.isStateOnly = true
	model.allRows = rows
	model.rows = rows

	output := model.renderListView()
	assert.Contains(t, output, "projected at current state")
}

// TestRenderFootnote_LoadedHidesAsterisk verifies footnote absent after preview loaded.
func TestRenderFootnote_LoadedHidesAsterisk(t *testing.T) {
	ctx := context.Background()
	rows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateList
	model.previewLoaded = true
	model.allRows = rows
	model.rows = rows

	output := model.renderListView()
	assert.NotContains(t, output, "projected at current state")
}
