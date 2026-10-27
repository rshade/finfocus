package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestOverviewViewPreviewOnlySourceWarning(t *testing.T) {
	t.Parallel()
	const warning = "Stack state unavailable (backend_unreachable); 4 unchanged resources omitted."
	model, _ := NewOverviewModel(t.Context(), nil, 0, nil, nil)
	sized, _ := model.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	updated, cmd := sized.Update(OverviewDataReadyMsg{
		Rows: []engine.OverviewRow{
			{URN: "urn::new", Type: "aws:s3/bucket:Bucket", Status: engine.StatusCreating},
		},
		TotalCount:    1,
		SourceWarning: warning,
	})
	require.Nil(t, cmd)
	updated, _ = updated.Update(OverviewAllResourcesLoadedMsg{})
	assert.Contains(t, updated.View().Content, warning)
	assert.Contains(t, updated.View().Content, "creating")
}

// TestOverviewView_InitializingRender verifies the initializing view output.
func TestOverviewView_InitializingRender(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.progressMsg = ""

	output := model.View().Content
	// Phase checklist is always shown; first phase is always "Loading stack state..."
	assert.Contains(t, output, "Loading stack state")
}

// TestOverviewView_ErrorStateRender verifies the error view output.
func TestOverviewView_ErrorStateRender(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.loadingState = nil

	output := model.renderInitializingView()
	assert.Contains(t, output, "Loading stack state")
}

// TestRenderStatusBar_StateOnlyShowsPHint verifies "[p] load pending changes" hint in state-only mode.
func TestRenderStatusBar_StateOnlyShowsPHint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
	model.state = ViewStateList
	model.isStateOnly = true

	output := model.renderStatusBar()
	assert.Contains(t, output, "[p] load pending changes")
}

// TestRenderStatusBar_LoadingShowsElapsed verifies elapsed timer shown while preview loads.
func TestRenderStatusBar_LoadingShowsElapsed(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	ctx := context.Background()
	rows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateList
	model.isStateOnly = true
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)

	output := model.renderListView()
	assert.Contains(t, output, "projected at current state")
}

// TestRenderFootnote_LoadedHidesAsterisk verifies footnote absent after preview loaded.
func TestRenderFootnote_LoadedHidesAsterisk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.state = ViewStateList
	model.previewLoaded = true
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)

	output := model.renderListView()
	assert.NotContains(t, output, "projected at current state")
}

// ---------------------------------------------------------------------------
// renderDetailPropertyChanges
// ---------------------------------------------------------------------------

func TestRenderDetailView_ShowsPropertyChanges(t *testing.T) {
	t.Parallel()

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
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)

	output := model.View().Content
	assert.Contains(t, output, "PROPERTY CHANGES")
	assert.Contains(t, output, "instanceType")
	assert.Contains(t, output, "t3.medium")
	assert.Contains(t, output, "t3.large")
	assert.Contains(t, output, "rootBlockDevice")
	assert.Contains(t, output, "\u2192") // arrow character
}

func TestRenderDetailView_NoPropertyChanges(t *testing.T) {
	t.Parallel()

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
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)

	output := model.View().Content
	assert.NotContains(t, output, "PROPERTY CHANGES")
}

func TestRenderDetailView_PropertyChangesWithNoneValues(t *testing.T) {
	t.Parallel()

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
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)

	output := model.View().Content
	assert.Contains(t, output, "PROPERTY CHANGES")
	assert.Contains(t, output, "(none)")
	assert.Contains(t, output, "my-tag")
}

func TestRenderDetailPropertyChanges_EmptySlice(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	row := engine.OverviewRow{
		PropertyDiffs: []engine.PropertyDiff{},
	}
	renderDetailPropertyChanges(&content, engine.ComputeOverviewRowResult(row))
	assert.Empty(t, content.String())
}

func TestTruncateDiffValue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "short", truncateDiffValue("short"))
	assert.Empty(t, truncateDiffValue(""))

	long := strings.Repeat("x", 60)
	result := truncateDiffValue(long)
	assert.Len(t, result, maxDiffValueLen)
	assert.True(t, strings.HasSuffix(result, "..."))
}

func TestRenderDetailView_TruncatesLongValues(t *testing.T) {
	t.Parallel()

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
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)

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
	t.Parallel()

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
		PropertyDiffs: []engine.PropertyDiff{{Key: "ami", OldValue: "ami-old", NewValue: "ami-new"}},
	}

	// Pre-populate ComputedDelta (matches production flow).
	rows := []engine.OverviewRow{row}
	engine.PopulateComputedDeltas(rows, 15)
	row = rows[0]

	renderDetailCostImpactForDay(&content, engine.ComputeOverviewRowResult(row), 15)
	output := content.String()

	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "Current (est. monthly)")
	assert.Contains(t, output, "After Change")
	assert.Contains(t, output, "$33.87")
	assert.Contains(t, output, "Delta")
}

func TestRenderDetailCostImpact_CreatingResource(t *testing.T) {
	t.Parallel()

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

	// Pre-populate ComputedDelta (matches production flow).
	rows := []engine.OverviewRow{row}
	engine.PopulateComputedDeltas(rows, 15)
	row = rows[0]

	renderDetailCostImpactForDay(&content, engine.ComputeOverviewRowResult(row), 15)
	output := content.String()

	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "New Monthly Cost")
	assert.Contains(t, output, "$33.87")
	assert.Contains(t, output, "Delta")
	assert.Contains(t, output, "+$33.87")
}

func TestRenderDetailCostImpact_DeletingResource(t *testing.T) {
	t.Parallel()

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

	// Pre-populate ComputedDelta (matches production flow).
	rows := []engine.OverviewRow{row}
	engine.PopulateComputedDeltas(rows, 15)
	row = rows[0]

	// Use fixed day >= driftMinDay to ensure extrapolation is valid.
	renderDetailCostImpactForDay(&content, engine.ComputeOverviewRowResult(row), 15)
	output := content.String()

	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "Current (est. monthly)")
	assert.Contains(t, output, "Delta")
	assert.Contains(t, output, "-$")
}

func TestRenderDetailCostImpact_ActiveResource(t *testing.T) {
	t.Parallel()

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

	renderDetailCostImpactForDay(&content, engine.ComputeOverviewRowResult(row), 15)
	assert.Empty(t, content.String(), "active resources should not show COST IMPACT")
}

func TestRenderDetailCostImpact_NoCostData(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	row := engine.OverviewRow{
		URN:    "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
		Type:   "aws:ec2/instance:Instance",
		Status: engine.StatusReplacing,
		// No ActualCost or ProjectedCost — CalculateRowDelta returns false.
	}

	renderDetailCostImpactForDay(&content, engine.ComputeOverviewRowResult(row), 15)
	assert.Empty(t, content.String(), "no cost data should not show COST IMPACT")
}

func TestRenderDetailView_ShowsCostImpact(t *testing.T) {
	t.Parallel()

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
			PropertyDiffs: []engine.PropertyDiff{{Key: "instanceType", OldValue: "t3.small", NewValue: "t3.large"}},
		},
	}

	// Pre-populate ComputedDelta (matches production flow).
	engine.PopulateComputedDeltas(rows, 15)

	model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
	model.height = 40 // Show the complete cost section without scrolling.
	model.state = ViewStateDetail
	model.selected = 0
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)

	output := model.View().Content
	assert.Contains(t, output, "COST IMPACT")
	assert.Contains(t, output, "Current (est. monthly)")
	assert.Contains(t, output, "After Change")
	assert.Contains(t, output, "$150.00")
	assert.Contains(t, output, "Delta")
}

func listModelAtWidth(width int, rows []engine.OverviewRow) OverviewModel {
	model, _ := NewOverviewModel(context.Background(), rows, len(rows), nil, nil)
	model.state = ViewStateList
	model.width = width
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)
	model.rebuildTable()
	return model
}

func webHintLine(output string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "[w]") {
			return line, true
		}
	}
	return "", false
}

func TestOverviewWebHint_Width(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}
	tests := []struct {
		name  string
		width int
		want  string
	}{
		{name: "long form", width: 120, want: "[w] web dashboard · finfocus --web"},
		{name: "short form", width: 20, want: "[w] web"},
		{name: "omitted when five columns", width: 5, want: ""},
		{name: "omitted when zero columns", width: 0, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output := listModelAtWidth(tt.width, rows).renderListView()
			line, ok := webHintLine(output)
			if tt.want == "" {
				assert.False(t, ok, "hint line: %q", line)
				assert.Empty(t, renderWebHint(tt.width))
				return
			}
			require.True(t, ok, "expected a hint line in %q", output)
			hint := strings.TrimRight(line, " ")
			assert.Contains(t, hint, tt.want)
			if tt.width == 20 {
				assert.NotContains(t, hint, "finfocus --web")
			}
			assert.LessOrEqual(t, lipgloss.Width(hint), tt.width)
			assert.LessOrEqual(t, lipgloss.Width(renderWebHint(tt.width)), tt.width)
		})
	}
}

func TestOverviewWebHint_NotInOtherViews(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assertNoWebHint := func(t *testing.T, output string) {
		t.Helper()
		assert.NotContains(t, output, "[w]")
		assert.NotContains(t, output, "finfocus --web")
	}

	t.Run("initializing", func(t *testing.T) {
		t.Parallel()
		model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
		assert.Equal(t, ViewStateInitializing, model.state)
		assertNoWebHint(t, model.View().Content)
	})

	t.Run("passphrase", func(t *testing.T) {
		t.Parallel()
		model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
		model.showPassphraseInput = true
		output := model.View().Content
		assert.Contains(t, output, "PULUMI_CONFIG_PASSPHRASE")
		assertNoWebHint(t, output)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		model, _ := NewOverviewModel(ctx, nil, 0, nil, nil)
		model.state = ViewStateError
		model.err = assert.AnError
		output := model.View().Content
		assert.Contains(t, output, assert.AnError.Error())
		assertNoWebHint(t, output)
	})

	t.Run("detail", func(t *testing.T) {
		t.Parallel()
		rows := []engine.OverviewRow{
			{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
		}
		model, _ := NewOverviewModel(ctx, rows, 1, nil, nil)
		model.state = ViewStateDetail
		model.selected = 0
		model.allRows = computeRowResults(rows)
		model.rows = computeRowResults(rows)
		output := model.View().Content
		assert.Contains(t, output, "RESOURCE DETAIL")
		assertNoWebHint(t, output)
	})
}

func TestOverviewWebHint_ErrorRowStillShowsHint(t *testing.T) {
	t.Parallel()

	t.Run("error row", func(t *testing.T) {
		t.Parallel()
		rows := []engine.OverviewRow{{
			URN:    "urn:pulumi:dev::app::aws:ec2/instance:Instance::broken",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			Error:  &engine.OverviewRowError{Message: "plugin failed"},
		}}
		output := listModelAtWidth(120, rows).renderListView()
		assert.Contains(t, output, "ERR")
		line, ok := webHintLine(output)
		require.True(t, ok)
		assert.Contains(t, line, "[w] web dashboard · finfocus --web")
	})

	t.Run("empty stack", func(t *testing.T) {
		t.Parallel()
		output := listModelAtWidth(120, nil).renderListView()
		line, ok := webHintLine(output)
		require.True(t, ok)
		assert.Contains(t, line, "[w] web dashboard · finfocus --web")
	})
}
