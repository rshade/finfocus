package tui

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// TestNewOverviewModel verifies initial model state.
func TestNewOverviewModel(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::project::aws:ec2:Instance::instance-1",
			Type:   "aws:ec2:Instance",
			Status: engine.StatusActive,
		},
		{
			URN:    "urn:pulumi:stack::project::aws:s3:Bucket::bucket-1",
			Type:   "aws:s3:Bucket",
			Status: engine.StatusCreating,
		},
	}

	model, cmd := NewOverviewModel(ctx, skeletonRows, 2)

	assert.Equal(t, ViewStateLoading, model.state)
	assert.Equal(t, 2, model.totalCount)
	assert.Equal(t, 0, model.loadedCount)
	assert.Len(t, model.allRows, 2)
	assert.NotNil(t, cmd)
	assert.Equal(t, SortByCost, model.sortBy)
}

// TestOverviewModel_StateTransitions verifies state machine transitions.
func TestOverviewModel_StateTransitions(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 1)
	assert.Equal(t, ViewStateLoading, model.state)

	// Transition: Loading -> List (all resources loaded)
	msg := OverviewAllResourcesLoadedMsg{}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, ViewStateList, model.state)

	// Transition: List -> Detail (Enter key)
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = model.Update(keyMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, ViewStateDetail, model.state)

	// Transition: Detail -> List (Esc key)
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updatedModel, _ = model.Update(escMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, ViewStateList, model.state)
}

// TestOverviewModel_ResourceLoadedMsg verifies row updates.
func TestOverviewModel_ResourceLoadedMsg(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test1", Type: "aws:ec2:Instance", Status: engine.StatusActive},
		{URN: "urn:test2", Type: "aws:s3:Bucket", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 2)

	enrichedRow := engine.OverviewRow{
		URN:    "urn:test1",
		Type:   "aws:ec2:Instance",
		Status: engine.StatusActive,
		ProjectedCost: &engine.ProjectedCostData{
			MonthlyCost: 123.45,
			Currency:    "USD",
		},
	}

	msg := OverviewResourceLoadedMsg{Index: 0, Row: enrichedRow}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(OverviewModel)

	assert.Equal(t, 1, model.loadedCount)
	assert.NotNil(t, model.allRows[0].ProjectedCost)
	assert.Equal(t, 123.45, model.allRows[0].ProjectedCost.MonthlyCost)
}

// TestOverviewModel_LoadingProgressMsg verifies progress updates.
func TestOverviewModel_LoadingProgressMsg(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 10)

	msg := OverviewLoadingProgressMsg{Loaded: 5, Total: 10}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(OverviewModel)

	assert.Contains(t, model.progressMsg, "5/10")
	assert.Contains(t, model.progressMsg, "50%")
}

// TestOverviewModel_KeyboardNavigation verifies up/down/j/k keys.
func TestOverviewModel_KeyboardNavigation(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test1", Type: "aws:ec2:Instance", Status: engine.StatusActive},
		{URN: "urn:test2", Type: "aws:s3:Bucket", Status: engine.StatusActive},
		{URN: "urn:test3", Type: "aws:rds:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 3)
	model.state = ViewStateList

	// Initial cursor at row 0
	assert.Equal(t, 0, model.table.Cursor())

	// Down arrow
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := model.Update(downMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 1, model.table.Cursor())

	// 'j' key (vim-style down)
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updatedModel, _ = model.Update(jMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 2, model.table.Cursor())

	// 'k' key (vim-style up)
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	updatedModel, _ = model.Update(kMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 1, model.table.Cursor())

	// Up arrow
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = model.Update(upMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 0, model.table.Cursor())
}

// TestOverviewModel_SortCycling verifies 's' key sort cycling.
func TestOverviewModel_SortCycling(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test1", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 1)
	model.state = ViewStateList

	assert.Equal(t, SortByCost, model.sortBy)

	sMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}

	// Cycle: Cost -> Name
	updatedModel, _ := model.Update(sMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, SortByName, model.sortBy)

	// Cycle: Name -> Type
	updatedModel, _ = model.Update(sMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, SortByType, model.sortBy)

	// Cycle: Type -> Delta
	updatedModel, _ = model.Update(sMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, SortByDelta, model.sortBy)

	// Cycle: Delta -> Cost (wrap around)
	updatedModel, _ = model.Update(sMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, SortByCost, model.sortBy)
}

// TestOverviewModel_FilterMode verifies filter entry/exit.
func TestOverviewModel_FilterMode(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test:ec2", Type: "aws:ec2:Instance", Status: engine.StatusActive},
		{URN: "urn:test:s3", Type: "aws:s3:Bucket", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 2)
	model.state = ViewStateList

	assert.False(t, model.showFilter)

	// Enter filter mode with '/'
	slashMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updatedModel, _ := model.Update(slashMsg)
	model = updatedModel.(OverviewModel)
	assert.True(t, model.showFilter)

	// Exit filter mode with Esc
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updatedModel, _ = model.Update(escMsg)
	model = updatedModel.(OverviewModel)
	assert.False(t, model.showFilter)
}

// TestOverviewModel_FilterTextMatching verifies URN and Type matching.
func TestOverviewModel_FilterTextMatching(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::project::aws:ec2:Instance::instance-1",
			Type:   "aws:ec2:Instance",
			Status: engine.StatusActive,
		},
		{URN: "urn:pulumi:stack::project::aws:s3:Bucket::bucket-1", Type: "aws:s3:Bucket", Status: engine.StatusActive},
		{
			URN:    "urn:pulumi:stack::project::aws:rds:Instance::db-1",
			Type:   "aws:rds:Instance",
			Status: engine.StatusActive,
		},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 3)
	model.state = ViewStateList
	model.allRows = skeletonRows

	// Filter by "ec2" (should match URN and Type)
	model.applyFilter("ec2")
	assert.Len(t, model.rows, 1)
	assert.Contains(t, model.rows[0].Type, "ec2")

	// Filter by "Bucket" (should match Type)
	model.applyFilter("Bucket")
	assert.Len(t, model.rows, 1)
	assert.Contains(t, model.rows[0].Type, "Bucket")

	// Filter by "instance" (should match multiple - case insensitive)
	model.applyFilter("instance")
	assert.Len(t, model.rows, 2)

	// Clear filter
	model.applyFilter("")
	assert.Len(t, model.rows, 3)
}

// TestOverviewModel_PaginationBoundaries verifies PgUp/PgDn at boundaries.
func TestOverviewModel_PaginationBoundaries(t *testing.T) {
	ctx := context.Background()

	// Create 300 rows to trigger pagination (threshold is 250)
	skeletonRows := make([]engine.OverviewRow, 300)
	for i := range skeletonRows {
		skeletonRows[i] = engine.OverviewRow{
			URN:    fmt.Sprintf("urn:test:%d", i),
			Type:   "aws:ec2:Instance",
			Status: engine.StatusActive,
		}
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 300)
	model.state = ViewStateList
	model.enablePaginationIfNeeded()

	require.True(t, model.paginationEnabled)
	assert.Equal(t, 1, model.currentPage)
	assert.Equal(t, 2, model.totalPages) // 300 rows / 250 per page = 2 pages

	// PgUp at first page (should stay at page 1)
	pgUpMsg := tea.KeyMsg{Type: tea.KeyPgUp}
	updatedModel, _ := model.Update(pgUpMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 1, model.currentPage)

	// PgDn to page 2
	pgDnMsg := tea.KeyMsg{Type: tea.KeyPgDown}
	updatedModel, _ = model.Update(pgDnMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 2, model.currentPage)

	// PgDn at last page (should stay at page 2)
	updatedModel, _ = model.Update(pgDnMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 2, model.currentPage)

	// PgUp back to page 1
	updatedModel, _ = model.Update(pgUpMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, 1, model.currentPage)
}

// TestOverviewModel_QuitKeys verifies q and Ctrl+C quit.
func TestOverviewModel_QuitKeys(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 1)
	model.state = ViewStateList

	// Test 'q' key
	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, cmd := model.Update(qMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, ViewStateQuitting, model.state)
	assert.NotNil(t, cmd) // Should return tea.Quit command

	// Reset and test Ctrl+C
	model.state = ViewStateList
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, cmd = model.Update(ctrlCMsg)
	model = updatedModel.(OverviewModel)
	assert.Equal(t, ViewStateQuitting, model.state)
	assert.NotNil(t, cmd) // Should return tea.Quit command
}

// TestOverviewModel_WindowResize verifies terminal resize handling.
func TestOverviewModel_WindowResize(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 1)

	assert.Equal(t, defaultWidth, model.width)
	assert.Equal(t, defaultHeight, model.height)

	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := model.Update(resizeMsg)
	model = updatedModel.(OverviewModel)

	assert.Equal(t, 120, model.width)
	assert.Equal(t, 40, model.height)
}

// TestOverviewModel_GetCost verifies cost extraction for sorting.
func TestOverviewModel_GetCost(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		row      engine.OverviewRow
		expected float64
	}{
		{
			name: "projected cost only",
			row: engine.OverviewRow{
				ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 100.0},
			},
			expected: 100.0,
		},
		{
			name: "actual cost only",
			row: engine.OverviewRow{
				ActualCost: &engine.ActualCostData{MTDCost: 50.0},
			},
			expected: 50.0,
		},
		{
			name: "both costs - projected takes precedence",
			row: engine.OverviewRow{
				ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 100.0},
				ActualCost:    &engine.ActualCostData{MTDCost: 50.0},
			},
			expected: 100.0,
		},
		{
			name:     "no cost data",
			row:      engine.OverviewRow{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, _ := NewOverviewModel(ctx, []engine.OverviewRow{tt.row}, 1)
			cost := model.getCost(tt.row)
			assert.Equal(t, tt.expected, cost)
		})
	}
}

// TestOverviewModel_GetDelta verifies drift delta extraction for sorting.
func TestOverviewModel_GetDelta(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		row      engine.OverviewRow
		expected float64
	}{
		{
			name: "with drift data",
			row: engine.OverviewRow{
				CostDrift: &engine.CostDriftData{Delta: 25.5},
			},
			expected: 25.5,
		},
		{
			name:     "no drift data",
			row:      engine.OverviewRow{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, _ := NewOverviewModel(ctx, []engine.OverviewRow{tt.row}, 1)
			delta := model.getDelta(tt.row)
			assert.Equal(t, tt.expected, delta)
		})
	}
}

// TestOverviewModel_GetVisibleRows verifies pagination row slicing.
func TestOverviewModel_GetVisibleRows(t *testing.T) {
	ctx := context.Background()

	// Create 300 rows
	rows := make([]engine.OverviewRow, 300)
	for i := range rows {
		rows[i] = engine.OverviewRow{URN: fmt.Sprintf("urn:%d", i), Type: "test", Status: engine.StatusActive}
	}

	model, _ := NewOverviewModel(ctx, rows, 300)
	model.allRows = rows
	model.rows = rows
	model.enablePaginationIfNeeded()

	// Page 1: rows 0-249
	visibleRows := model.getVisibleRows()
	assert.Len(t, visibleRows, 250)
	assert.Equal(t, rows[0].URN, visibleRows[0].URN)
	assert.Equal(t, rows[249].URN, visibleRows[249].URN)

	// Page 2: rows 250-299 (50 rows)
	model.currentPage = 2
	visibleRows = model.getVisibleRows()
	assert.Len(t, visibleRows, 50)
	assert.Equal(t, rows[250].URN, visibleRows[0].URN)
	assert.Equal(t, rows[299].URN, visibleRows[49].URN)
}

// TestOverviewModel_AllResourcesLoadedTransition verifies loading completion.
func TestOverviewModel_AllResourcesLoadedTransition(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test1", Type: "aws:ec2:Instance", Status: engine.StatusActive},
		{URN: "urn:test2", Type: "aws:s3:Bucket", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 2)
	assert.Equal(t, ViewStateLoading, model.state)

	// Send completion message
	msg := OverviewAllResourcesLoadedMsg{}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(OverviewModel)

	assert.Equal(t, ViewStateList, model.state)
	assert.Equal(t, 2, model.loadedCount)
}

// TestOverviewModel_InitCmd verifies Init returns loading spinner command.
func TestOverviewModel_InitCmd(t *testing.T) {
	ctx := context.Background()
	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 1)
	cmd := model.Init()

	assert.NotNil(t, cmd)
}

// TestOverviewModel_EnrichmentIntegration verifies progressive row updates.
func TestOverviewModel_EnrichmentIntegration(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	skeletonRows := []engine.OverviewRow{
		{URN: "urn:test1", Type: "aws:ec2:Instance", Status: engine.StatusActive},
		{URN: "urn:test2", Type: "aws:s3:Bucket", Status: engine.StatusActive},
	}

	model, _ := NewOverviewModel(ctx, skeletonRows, 2)

	// Simulate progressive enrichment
	enrichedRow1 := engine.OverviewRow{
		URN:    "urn:test1",
		Type:   "aws:ec2:Instance",
		Status: engine.StatusActive,
		ProjectedCost: &engine.ProjectedCostData{
			MonthlyCost: 75.0,
			Currency:    "USD",
		},
		ActualCost: &engine.ActualCostData{
			MTDCost:  50.0,
			Currency: "USD",
			Period: engine.DateRange{
				Start: now.AddDate(0, 0, -15),
				End:   now,
			},
		},
	}

	msg1 := OverviewResourceLoadedMsg{Index: 0, Row: enrichedRow1}
	updatedModel, _ := model.Update(msg1)
	model = updatedModel.(OverviewModel)

	assert.Equal(t, 1, model.loadedCount)
	assert.NotNil(t, model.allRows[0].ProjectedCost)
	assert.NotNil(t, model.allRows[0].ActualCost)
	assert.Equal(t, 75.0, model.allRows[0].ProjectedCost.MonthlyCost)
}
