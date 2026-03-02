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

	output := model.View().Content
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

	output := model.View().Content
	// Phase checklist is always shown; first phase is always "Loading stack state..."
	assert.Contains(t, output, "Loading stack state")
}

// TestOverviewView_ErrorStateRender verifies the error view output.
func TestOverviewView_ErrorStateRender(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.state = ViewStateError
	model.err = assert.AnError

	output := model.View().Content
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

// ---------------------------------------------------------------------------
// renderDetailPropertyChanges
// ---------------------------------------------------------------------------

func TestRenderDetailView_ShowsPropertyChanges(t *testing.T) {
	ctx := context.Background()
	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusUpdating,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "instanceType", OldValue: "t3.medium", NewValue: "t3.large"},
				{Key: "rootBlockDevice", OldValue: "20", NewValue: "50"},
			},
		},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateDetail
	model.selected = 0
	model.allRows = rows
	model.rows = rows

	output := model.View().Content
	assert.Contains(t, output, "PROPERTY CHANGES")
	assert.Contains(t, output, "instanceType")
	assert.Contains(t, output, "t3.medium")
	assert.Contains(t, output, "t3.large")
	assert.Contains(t, output, "rootBlockDevice")
	assert.Contains(t, output, "\u2192") // arrow character
}

func TestRenderDetailView_NoPropertyChanges(t *testing.T) {
	ctx := context.Background()
	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
		},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateDetail
	model.selected = 0
	model.allRows = rows
	model.rows = rows

	output := model.View().Content
	assert.NotContains(t, output, "PROPERTY CHANGES")
}

func TestRenderDetailView_PropertyChangesWithNoneValues(t *testing.T) {
	ctx := context.Background()
	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusUpdating,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "newTag", OldValue: "", NewValue: "my-tag"},
			},
		},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateDetail
	model.selected = 0
	model.allRows = rows
	model.rows = rows

	output := model.View().Content
	assert.Contains(t, output, "PROPERTY CHANGES")
	assert.Contains(t, output, "(none)")
	assert.Contains(t, output, "my-tag")
}

func TestRenderDetailPropertyChanges_EmptySlice(t *testing.T) {
	var content strings.Builder
	row := engine.OverviewRow{
		PropertyDiffs: []engine.PropertyDiff{},
	}
	renderDetailPropertyChanges(&content, row)
	assert.Empty(t, content.String())
}

func TestTruncateDiffValue(t *testing.T) {
	assert.Equal(t, "short", truncateDiffValue("short"))
	assert.Equal(t, "", truncateDiffValue(""))

	long := strings.Repeat("x", 60)
	result := truncateDiffValue(long)
	assert.Len(t, result, maxDiffValueLen)
	assert.True(t, strings.HasSuffix(result, "..."))
}

func TestRenderDetailView_TruncatesLongValues(t *testing.T) {
	ctx := context.Background()
	longValue := strings.Repeat("a", 80)
	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusUpdating,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "tags", OldValue: longValue, NewValue: "short"},
			},
		},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateDetail
	model.selected = 0
	model.allRows = rows
	model.rows = rows

	output := model.View().Content
	assert.Contains(t, output, "PROPERTY CHANGES")
	// The long value should be truncated, not appear in full.
	assert.NotContains(t, output, longValue)
	assert.Contains(t, output, "...")
}

// ---------------------------------------------------------------------------
// renderDetailCostImpact
// ---------------------------------------------------------------------------

func TestRenderDetailCostImpact_ReplacingResource(t *testing.T) {
	now := time.Now()
	period := engine.DateRange{Start: now.Add(-24 * time.Hour), End: now}

	var content strings.Builder
	row := engine.OverviewRow{
		URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
		Type:   "aws:ec2/instance:Instance",
		Status: engine.StatusReplacing,
		ActualCost: &engine.ActualCostData{
			MTDCost:  25.35,
			Currency: "USD",
			Period:   period,
		},
		ProjectedCost: &engine.ProjectedCostData{
			MonthlyCost: 33.87,
			Currency:    "USD",
		},
	}

	renderDetailCostImpact(&content, row)
	output := content.String()

	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "Current (est. monthly)")
	assert.Contains(t, output, "After Change")
	assert.Contains(t, output, "$33.87")
	assert.Contains(t, output, "Delta")
}

func TestRenderDetailCostImpact_CreatingResource(t *testing.T) {
	var content strings.Builder
	row := engine.OverviewRow{
		URN:    "urn:pulumi:stack::proj::aws:s3:Bucket::data",
		Type:   "aws:s3:Bucket",
		Status: engine.StatusCreating,
		ProjectedCost: &engine.ProjectedCostData{
			MonthlyCost: 33.87,
			Currency:    "USD",
		},
	}

	renderDetailCostImpact(&content, row)
	output := content.String()

	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "New Monthly Cost")
	assert.Contains(t, output, "$33.87")
	assert.Contains(t, output, "Delta")
	assert.Contains(t, output, "+$33.87")
}

func TestRenderDetailCostImpact_DeletingResource(t *testing.T) {
	now := time.Now()
	period := engine.DateRange{Start: now.Add(-24 * time.Hour), End: now}

	var content strings.Builder
	row := engine.OverviewRow{
		URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::old",
		Type:   "aws:ec2/instance:Instance",
		Status: engine.StatusDeleting,
		ActualCost: &engine.ActualCostData{
			MTDCost:  25.35,
			Currency: "USD",
			Period:   period,
		},
	}

	renderDetailCostImpact(&content, row)
	output := content.String()

	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "Current (est. monthly)")
	assert.Contains(t, output, "Delta")
	assert.Contains(t, output, "-$")
}

func TestRenderDetailCostImpact_ActiveResource(t *testing.T) {
	var content strings.Builder
	row := engine.OverviewRow{
		URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
		Type:   "aws:ec2/instance:Instance",
		Status: engine.StatusActive,
		ProjectedCost: &engine.ProjectedCostData{
			MonthlyCost: 100.00,
			Currency:    "USD",
		},
	}

	renderDetailCostImpact(&content, row)
	assert.Empty(t, content.String(), "active resources should not show COST IMPACT")
}

func TestRenderDetailCostImpact_NoCostData(t *testing.T) {
	var content strings.Builder
	row := engine.OverviewRow{
		URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
		Type:   "aws:ec2/instance:Instance",
		Status: engine.StatusReplacing,
		// No ActualCost or ProjectedCost — CalculateRowDelta returns false.
	}

	renderDetailCostImpact(&content, row)
	assert.Empty(t, content.String(), "no cost data should not show COST IMPACT")
}

func TestRenderDetailView_ShowsCostImpact(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	period := engine.DateRange{Start: now.Add(-24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusUpdating,
			ActualCost: &engine.ActualCostData{
				MTDCost:  50.00,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
			},
		},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateDetail
	model.selected = 0
	model.allRows = rows
	model.rows = rows

	output := model.View().Content
	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "Current (est. monthly)")
	assert.Contains(t, output, "After Change")
	assert.Contains(t, output, "$150.00")
	assert.Contains(t, output, "Delta")
}
