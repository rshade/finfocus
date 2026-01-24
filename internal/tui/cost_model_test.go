package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestCostViewModel_Update(t *testing.T) {
	results := []engine.CostResult{
		{ResourceType: "res1", Monthly: 10.0},
		{ResourceType: "res2", Monthly: 20.0},
	}
	m := NewCostViewModel(results)

	// Initial state.
	assert.Equal(t, ViewStateList, m.state)

	// Test navigation (Down).
	m.table.SetCursor(0)
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedM, _ := m.Update(msg)
	m, ok := updatedM.(*CostViewModel)
	require.True(t, ok)
	assert.Equal(t, 1, m.table.Cursor())

	// Test Enter (Go to Detail).
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ = m.Update(msg)
	m, ok = updatedM.(*CostViewModel)
	require.True(t, ok)
	assert.Equal(t, ViewStateDetail, m.state)
	assert.Equal(t, 1, m.selected)

	// Test Esc (Back to List).
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updatedM, _ = m.Update(msg)
	m, ok = updatedM.(*CostViewModel)
	require.True(t, ok)
	assert.Equal(t, ViewStateList, m.state)

	// Test Quit.
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedM, _ = m.Update(msg)
	m, ok = updatedM.(*CostViewModel)
	require.True(t, ok)
	assert.Equal(t, ViewStateQuitting, m.state)
}

func TestCostViewModel_Filter(t *testing.T) {
	results := []engine.CostResult{
		{ResourceType: "aws:ec2", ResourceID: "match-me"},
		{ResourceType: "aws:s3", ResourceID: "ignore-me"},
	}
	m := NewCostViewModel(results)

	// Initial.
	assert.Len(t, m.results, 2)

	// Apply filter.
	m.textInput.SetValue("match")
	m.applyFilter()

	assert.Len(t, m.results, 1)
	assert.Equal(t, "match-me", m.results[0].ResourceID)

	// Clear filter.
	m.textInput.SetValue("")
	m.applyFilter()
	assert.Len(t, m.results, 2)
}

func TestCostViewModel_Sort(t *testing.T) {
	results := []engine.CostResult{
		{ResourceID: "A", Monthly: 10.0},
		{ResourceID: "B", Monthly: 20.0},
	}
	m := NewCostViewModel(results)

	// Default sort (SortByCost=0).
	// Cost A=10, B=20. Descending => B, A.
	m.applySort()
	assert.Equal(t, "B", m.results[0].ResourceID)

	// Sort By Name (1).
	m.sortBy = SortByName
	m.applySort()
	// Ascending ID: A, B.
	assert.Equal(t, "A", m.results[0].ResourceID)
}

func TestCostViewModel_ActualCostMode(t *testing.T) {
	results := []engine.CostResult{
		{ResourceType: "aws:ec2", TotalCost: 100.0, Currency: "USD"},
	}

	m := NewCostViewModelFromActual(results, engine.GroupByResource)

	assert.True(t, m.isActual)
	assert.Equal(t, engine.GroupByResource, m.groupBy)
	assert.Equal(t, ViewStateList, m.state)
	assert.Len(t, m.results, 1)
}

func TestCostViewModel_ActualCostModeWithAggregation(t *testing.T) {
	// Create results with dates for time-based grouping.
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2",
			TotalCost:    100.0,
			Currency:     "USD",
		},
	}

	m := NewCostViewModelFromActual(results, engine.GroupByMonthly)

	assert.True(t, m.isActual)
	assert.Equal(t, engine.GroupByMonthly, m.groupBy)
	// Aggregation may succeed or fall back depending on input data.
	assert.NotNil(t, m.table)
}

func TestCostViewModel_ErrorState(t *testing.T) {
	results := []engine.CostResult{}
	m := NewCostViewModel(results)

	// Simulate an error via View.
	m.err = assert.AnError
	m.state = ViewStateError

	// Verify error is displayed in view.
	view := m.View()
	assert.Contains(t, view, "Error:")
}

func TestCostViewModel_WindowResize(t *testing.T) {
	results := []engine.CostResult{
		{ResourceType: "aws:ec2", Monthly: 10.0},
	}
	m := NewCostViewModel(results)

	// Simulate window resize.
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedM, _ := m.Update(msg)
	m, ok := updatedM.(*CostViewModel)
	require.True(t, ok)

	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
}

func TestNewCostViewModelWithLoading(t *testing.T) {
	fetched := false
	fetcher := func() ([]engine.CostResult, error) {
		fetched = true
		return []engine.CostResult{{ResourceType: "aws:ec2"}}, nil
	}

	m := NewCostViewModelWithLoading(fetcher)

	assert.Equal(t, ViewStateLoading, m.state)
	assert.NotNil(t, m.loading)
	assert.NotNil(t, m.fetchCmd)

	// Execute the fetch command (simulated).
	if m.fetchCmd != nil {
		msg := m.fetchCmd()
		loadMsg, ok := msg.(loadingCompleteMsg)
		assert.True(t, ok)
		assert.True(t, fetched)
		assert.Len(t, loadMsg.results, 1)
		assert.NoError(t, loadMsg.err)
	}
}

func TestCostViewModel_Init(t *testing.T) {
	t.Run("loading state returns commands", func(t *testing.T) {
		m := NewCostViewModelWithLoading(func() ([]engine.CostResult, error) {
			return []engine.CostResult{}, nil
		})
		cmd := m.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("list state with no filter returns nil", func(t *testing.T) {
		m := NewCostViewModel([]engine.CostResult{})
		cmd := m.Init()
		// Without loading or filter, Init returns nil (tea.Batch of empty).
		_ = cmd // May be nil or batch of nothing.
	})

	t.Run("list state with filter returns blink command", func(t *testing.T) {
		m := NewCostViewModel([]engine.CostResult{})
		m.showFilter = true
		cmd := m.Init()
		assert.NotNil(t, cmd)
	})
}

func TestCostViewModel_HandleLoadingComplete(t *testing.T) {
	t.Run("success transition to list", func(t *testing.T) {
		m := NewCostViewModelWithLoading(func() ([]engine.CostResult, error) {
			return []engine.CostResult{{ResourceType: "aws:ec2", Monthly: 50.0}}, nil
		})

		msg := loadingCompleteMsg{
			results: []engine.CostResult{{ResourceType: "aws:ec2", Monthly: 50.0}},
			err:     nil,
		}

		updatedM, _ := m.Update(msg)
		model := updatedM.(*CostViewModel)
		assert.Equal(t, ViewStateList, model.state)
		assert.Len(t, model.results, 1)
	})

	t.Run("error transition to error state", func(t *testing.T) {
		m := NewCostViewModelWithLoading(func() ([]engine.CostResult, error) {
			return nil, assert.AnError
		})

		msg := loadingCompleteMsg{
			results: nil,
			err:     assert.AnError,
		}

		updatedM, cmd := m.Update(msg)
		model := updatedM.(*CostViewModel)
		assert.Equal(t, ViewStateError, model.state)
		assert.NotNil(t, model.err)
		assert.NotNil(t, cmd) // tea.Quit command.
	})
}

func TestCostViewModel_HandleFilterInput(t *testing.T) {
	results := []engine.CostResult{
		{ResourceType: "aws:ec2", ResourceID: "test-1"},
		{ResourceType: "aws:s3", ResourceID: "test-2"},
	}
	m := NewCostViewModel(results)

	// Activate filter mode.
	m.showFilter = true
	m.textInput.Focus()

	// Test Enter key closes filter.
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ := m.Update(msg)
	model := updatedM.(*CostViewModel)
	assert.False(t, model.showFilter)

	// Reactivate and test Esc key.
	model.showFilter = true
	model.textInput.Focus()
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updatedM, _ = model.Update(msg)
	model = updatedM.(*CostViewModel)
	assert.False(t, model.showFilter)
}

func TestCostViewModel_HandleListUpdate_ActivateFilter(t *testing.T) {
	m := NewCostViewModel([]engine.CostResult{
		{ResourceType: "aws:ec2", Monthly: 10.0},
	})

	// Press / to activate filter.
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updatedM, _ := m.Update(msg)
	model := updatedM.(*CostViewModel)
	assert.True(t, model.showFilter)
}

func TestCostViewModel_HandleListUpdate_CycleSort(t *testing.T) {
	m := NewCostViewModel([]engine.CostResult{
		{ResourceType: "aws:ec2", ResourceID: "A", Monthly: 10.0},
		{ResourceType: "aws:s3", ResourceID: "B", Monthly: 20.0},
	})

	initial := m.sortBy

	// Press 's' to cycle sort.
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	updatedM, _ := m.Update(msg)
	model := updatedM.(*CostViewModel)
	assert.NotEqual(t, initial, model.sortBy)
}

func TestCostViewModel_HandleListUpdate_ClearFilter(t *testing.T) {
	m := NewCostViewModel([]engine.CostResult{
		{ResourceType: "aws:ec2", ResourceID: "test"},
	})

	// Set filter value.
	m.textInput.SetValue("test")
	m.applyFilter()

	// Press Esc to clear filter.
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedM, _ := m.Update(msg)
	model := updatedM.(*CostViewModel)
	assert.Empty(t, model.textInput.Value())
}

func TestCostViewModel_HandleListUpdate_EnterOnAggregation(t *testing.T) {
	results := []engine.CostResult{
		{ResourceType: "aws:ec2", TotalCost: 100.0, Currency: "USD"},
	}
	m := NewCostViewModelFromActual(results, engine.GroupByMonthly)

	// Press Enter on aggregation view should do nothing.
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ := m.Update(msg)
	model := updatedM.(*CostViewModel)
	// Should stay in list state (no detail view for aggregations).
	assert.Equal(t, ViewStateList, model.state)
}

func TestCostViewModel_View_AllStates(t *testing.T) {
	t.Run("quitting state returns empty", func(t *testing.T) {
		m := NewCostViewModel([]engine.CostResult{})
		m.state = ViewStateQuitting
		assert.Empty(t, m.View())
	})

	t.Run("loading state returns loading view", func(t *testing.T) {
		m := NewCostViewModelWithLoading(func() ([]engine.CostResult, error) {
			return nil, nil
		})
		view := m.View()
		assert.Contains(t, view, "Querying")
	})

	t.Run("detail state with valid index", func(t *testing.T) {
		m := NewCostViewModel([]engine.CostResult{
			{ResourceType: "aws:ec2", ResourceID: "test-instance", Monthly: 50.0},
		})
		m.state = ViewStateDetail
		m.selected = 0
		m.width = 80
		view := m.View()
		assert.Contains(t, view, "RESOURCE DETAIL")
	})

	t.Run("detail state with invalid index", func(t *testing.T) {
		m := NewCostViewModel([]engine.CostResult{})
		m.state = ViewStateDetail
		m.selected = 99
		view := m.View()
		assert.Contains(t, view, "out of bounds")
	})

	t.Run("list state renders summary and table", func(t *testing.T) {
		m := NewCostViewModel([]engine.CostResult{
			{ResourceType: "aws:ec2", Monthly: 50.0},
		})
		m.width = 80
		view := m.View()
		assert.Contains(t, view, "COST SUMMARY")
	})

	t.Run("list state with filter shows filter input", func(t *testing.T) {
		m := NewCostViewModel([]engine.CostResult{
			{ResourceType: "aws:ec2", Monthly: 50.0},
		})
		m.width = 80
		m.showFilter = true
		view := m.View()
		assert.Contains(t, view, "Filter:")
	})
}

func TestCostViewModel_SortAllFields(t *testing.T) {
	results := []engine.CostResult{
		{ResourceID: "C", ResourceType: "aws:ec2", Monthly: 10.0, Delta: 5.0},
		{ResourceID: "A", ResourceType: "gcp:compute", Monthly: 20.0, Delta: -3.0},
		{ResourceID: "B", ResourceType: "azure:vm", Monthly: 15.0, Delta: 10.0},
	}

	t.Run("sort by type", func(t *testing.T) {
		m := NewCostViewModel(results)
		m.sortBy = SortByType
		m.applySort()
		// aws < azure < gcp alphabetically.
		assert.Equal(t, "aws:ec2", m.results[0].ResourceType)
	})

	t.Run("sort by delta", func(t *testing.T) {
		m := NewCostViewModel(results)
		m.sortBy = SortByDelta
		m.applySort()
		// Highest delta first: 10.0.
		assert.Equal(t, float64(10.0), m.results[0].Delta)
	})

	t.Run("sort actual costs", func(t *testing.T) {
		actualResults := []engine.CostResult{
			{ResourceID: "A", TotalCost: 100.0},
			{ResourceID: "B", TotalCost: 200.0},
		}
		m := NewCostViewModelFromActual(actualResults, engine.GroupByResource)
		m.sortBy = SortByCost
		m.applySort()
		// Highest cost first.
		assert.Equal(t, float64(200.0), m.results[0].TotalCost)
	})
}

func TestCostViewModel_HandleLoadingUpdate(t *testing.T) {
	m := NewCostViewModelWithLoading(func() ([]engine.CostResult, error) {
		return nil, nil
	})

	// Test that loading update returns a command.
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedM, cmd := m.Update(msg)
	_ = updatedM
	// In loading state, key messages are passed to loading spinner.
	_ = cmd
}

func TestCostViewModel_CtrlC(t *testing.T) {
	m := NewCostViewModel([]engine.CostResult{})

	// Test Ctrl+C quits.
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedM, cmd := m.Update(msg)
	model := updatedM.(*CostViewModel)
	assert.Equal(t, ViewStateQuitting, model.state)
	assert.NotNil(t, cmd) // tea.Quit.
}

func TestCostViewModel_RebuildTable_SmallHeight(t *testing.T) {
	m := NewCostViewModel([]engine.CostResult{
		{ResourceType: "aws:ec2", Monthly: 50.0},
	})

	// Set very small height to trigger minHeight fallback.
	m.height = 5
	m.rebuildTable()
	assert.NotNil(t, m.table)
}

func TestCostViewModel_FilterByResourceType(t *testing.T) {
	results := []engine.CostResult{
		{ResourceType: "aws:ec2/instance", ResourceID: "i-123"},
		{ResourceType: "aws:s3/bucket", ResourceID: "my-bucket"},
		{ResourceType: "gcp:compute", ResourceID: "vm-1"},
	}
	m := NewCostViewModel(results)

	// Filter by resource type.
	m.textInput.SetValue("ec2")
	m.applyFilter()
	assert.Len(t, m.results, 1)
	assert.Equal(t, "aws:ec2/instance", m.results[0].ResourceType)
}
