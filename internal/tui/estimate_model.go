package tui

import (
	"context"
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rshade/finfocus/internal/engine"
)

// EstimateState represents the current state of the estimate TUI.
type EstimateState int

const (
	// EstimateStateEditing indicates the user is editing properties.
	EstimateStateEditing EstimateState = iota
	// EstimateStateCalculating indicates cost calculation is in progress.
	EstimateStateCalculating
	// EstimateStateQuitting indicates the application is exiting.
	EstimateStateQuitting
	// EstimateStateError indicates an error occurred.
	EstimateStateError
)

// PropertyRow represents a single editable property in the estimate TUI.
type PropertyRow struct {
	Key           string
	OriginalValue string
	CurrentValue  string
	CostDelta     float64
}

// estimateRecalculateMsg is sent when cost recalculation completes.
type estimateRecalculateMsg struct {
	result *engine.EstimateResult
	err    error
}

// Default dimensions for estimate model.
const (
	estimateDefaultWidth  = 80
	estimateDefaultHeight = 20
)

// EstimateModel is the Bubble Tea model for interactive cost estimation.
type EstimateModel struct {
	// Resource context
	resource *engine.ResourceDescriptor
	ctx      context.Context

	// Editable properties
	properties []PropertyRow
	focusedRow int
	editMode   bool
	editBuffer string

	// Cost display
	baselineCost float64
	modifiedCost float64
	currency     string
	deltas       []engine.CostDelta

	// State management
	state   EstimateState
	loading bool
	err     error

	// Display dimensions
	width  int
	height int

	// Cost calculation callback
	recalculateFn func(context.Context, *engine.ResourceDescriptor, map[string]string) (*engine.EstimateResult, error)
}

// NewEstimateModel creates a new EstimateModel for interactive cost estimation.
//
// Parameters:
//   - ctx: Context for tracing and cancellation
//   - resource: The resource to estimate costs for
//   - result: Optional existing estimate result (for initial display)
//
// Returns a new EstimateModel ready for use with Bubble Tea.
func NewEstimateModel(
	ctx context.Context,
	resource *engine.ResourceDescriptor,
	result *engine.EstimateResult,
) *EstimateModel {
	m := &EstimateModel{
		ctx:      ctx,
		resource: resource,
		state:    EstimateStateEditing,
		currency: "USD",
		width:    estimateDefaultWidth,
		height:   estimateDefaultHeight,
	}

	// Initialize properties from resource
	m.initializeProperties()

	// Apply existing result if provided
	if result != nil {
		m.applyResult(result)
	}

	return m
}

// NewEstimateModelWithCallback creates an EstimateModel with a recalculation callback.
//
// The callback is called whenever a property is modified to recalculate costs.
func NewEstimateModelWithCallback(
	ctx context.Context,
	resource *engine.ResourceDescriptor,
	result *engine.EstimateResult,
	recalculateFn func(context.Context, *engine.ResourceDescriptor, map[string]string) (*engine.EstimateResult, error),
) *EstimateModel {
	m := NewEstimateModel(ctx, resource, result)
	m.recalculateFn = recalculateFn
	return m
}

// initializeProperties extracts properties from the resource into editable rows.
func (m *EstimateModel) initializeProperties() {
	if m.resource == nil || m.resource.Properties == nil {
		m.properties = []PropertyRow{}
		return
	}

	// Extract and sort property keys for consistent ordering
	keys := make([]string, 0, len(m.resource.Properties))
	for k := range m.resource.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	m.properties = make([]PropertyRow, 0, len(keys))
	for _, key := range keys {
		value := m.resource.Properties[key]
		strValue := fmt.Sprintf("%v", value)
		m.properties = append(m.properties, PropertyRow{
			Key:           key,
			OriginalValue: strValue,
			CurrentValue:  strValue,
			CostDelta:     0,
		})
	}
}

// applyResult applies an estimate result to the model state.
func (m *EstimateModel) applyResult(result *engine.EstimateResult) {
	if result.Baseline != nil {
		m.baselineCost = result.Baseline.Monthly
		if result.Baseline.Currency != "" {
			m.currency = result.Baseline.Currency
		}
	}
	if result.Modified != nil {
		m.modifiedCost = result.Modified.Monthly
	}
	m.deltas = result.Deltas

	// Update property deltas - reset all first, then apply from result
	for i := range m.properties {
		m.properties[i].CostDelta = 0 // Reset to avoid stale deltas
		for _, delta := range result.Deltas {
			if delta.Property == m.properties[i].Key {
				m.properties[i].CostDelta = delta.CostChange
				break
			}
		}
	}
}

// Init initializes the model.
func (m *EstimateModel) Init() tea.Cmd {
	// No initial commands needed for editing state
	return nil
}

// Update handles messages and updates the model state.
func (m *EstimateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case estimateRecalculateMsg:
		return m.handleRecalculateComplete(msg)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// handleKeyMsg processes keyboard input.
//
//nolint:exhaustive // Only handling relevant key types for estimate TUI navigation.
func (m *EstimateModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle edit mode separately
	if m.editMode {
		return m.handleEditModeKey(msg)
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		m.state = EstimateStateQuitting
		return m, tea.Quit

	case tea.KeyRunes:
		if string(msg.Runes) == "q" {
			m.state = EstimateStateQuitting
			return m, tea.Quit
		}

	case tea.KeyUp:
		if m.focusedRow > 0 {
			m.focusedRow--
		}
		return m, nil

	case tea.KeyDown:
		if m.focusedRow < len(m.properties)-1 {
			m.focusedRow++
		}
		return m, nil

	case tea.KeyEnter:
		if len(m.properties) > 0 && m.focusedRow < len(m.properties) {
			m.editMode = true
			m.editBuffer = m.properties[m.focusedRow].CurrentValue
		}
		return m, nil

	case tea.KeyEsc:
		// Clear any pending changes
		return m, nil
	}

	return m, nil
}

// handleEditModeKey processes keyboard input while editing a property.
//
//nolint:exhaustive // Only handling relevant key types for text editing.
func (m *EstimateModel) handleEditModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Commit the edit
		if m.focusedRow < len(m.properties) {
			m.properties[m.focusedRow].CurrentValue = m.editBuffer
		}
		m.editMode = false

		// Trigger recalculation if callback is set
		if m.recalculateFn != nil {
			return m, m.triggerRecalculation()
		}
		return m, nil

	case tea.KeyEsc:
		// Cancel the edit
		m.editMode = false
		m.editBuffer = ""
		return m, nil

	case tea.KeyBackspace:
		runes := []rune(m.editBuffer)
		if len(runes) > 0 {
			m.editBuffer = string(runes[:len(runes)-1])
		}
		return m, nil

	case tea.KeyRunes:
		m.editBuffer += string(msg.Runes)
		return m, nil
	}

	return m, nil
}

// triggerRecalculation creates a command to recalculate costs.
func (m *EstimateModel) triggerRecalculation() tea.Cmd {
	m.loading = true

	// Build overrides from changed properties
	overrides := make(map[string]string)
	for _, prop := range m.properties {
		if prop.CurrentValue != prop.OriginalValue {
			overrides[prop.Key] = prop.CurrentValue
		}
	}

	// Capture references before goroutine to avoid accessing model fields concurrently
	ctx := m.ctx
	resource := m.resource
	recalculateFn := m.recalculateFn

	return func() tea.Msg {
		result, err := recalculateFn(ctx, resource, overrides)
		return estimateRecalculateMsg{result: result, err: err}
	}
}

// handleRecalculateComplete processes the result of a cost recalculation.
func (m *EstimateModel) handleRecalculateComplete(msg estimateRecalculateMsg) (tea.Model, tea.Cmd) {
	m.loading = false

	if msg.err != nil {
		m.err = msg.err
		m.state = EstimateStateError
		return m, nil
	}

	if msg.result != nil {
		m.applyResult(msg.result)
	}

	return m, nil
}

// View renders the current view.
func (m *EstimateModel) View() string {
	switch m.state {
	case EstimateStateQuitting:
		return ""

	case EstimateStateError:
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)

	case EstimateStateEditing, EstimateStateCalculating:
		// Handled below
	}

	if m.loading {
		return RenderLoadingIndicator()
	}

	return m.renderEditingView()
}

// renderEditingView renders the main editing interface.
func (m *EstimateModel) renderEditingView() string {
	var output string

	// Header
	resourceID := ""
	if m.resource != nil {
		resourceID = m.resource.ID
	}
	provider := ""
	resourceType := ""
	if m.resource != nil {
		provider = m.resource.Provider
		resourceType = m.resource.Type
	}
	output += RenderEstimateHeader(provider, resourceType, resourceID)
	output += "\n\n"

	// Cost comparison
	output += RenderCostComparison(m.baselineCost, m.modifiedCost, m.currency)
	output += "\n\n"

	// Property table with edit buffer if in edit mode
	if m.editMode && m.focusedRow < len(m.properties) {
		// Show the edit buffer in the property table
		propsCopy := make([]PropertyRow, len(m.properties))
		copy(propsCopy, m.properties)
		propsCopy[m.focusedRow].CurrentValue = m.editBuffer + "▌" // Cursor indicator
		output += RenderPropertyTable(propsCopy, m.focusedRow, true)
	} else {
		output += RenderPropertyTable(m.properties, m.focusedRow, false)
	}

	output += "\n\n"

	// Help text
	output += RenderEstimateHelp()

	return output
}

// GetOverrides returns the current property overrides (changed values).
func (m *EstimateModel) GetOverrides() map[string]string {
	overrides := make(map[string]string)
	for _, prop := range m.properties {
		if prop.CurrentValue != prop.OriginalValue {
			overrides[prop.Key] = prop.CurrentValue
		}
	}
	return overrides
}

// GetResult returns the current estimate result based on model state.
func (m *EstimateModel) GetResult() *engine.EstimateResult {
	deltas := make([]engine.CostDelta, 0, len(m.properties))
	for _, prop := range m.properties {
		if prop.CurrentValue != prop.OriginalValue {
			deltas = append(deltas, engine.CostDelta{
				Property:      prop.Key,
				OriginalValue: prop.OriginalValue,
				NewValue:      prop.CurrentValue,
				CostChange:    prop.CostDelta,
			})
		}
	}

	return &engine.EstimateResult{
		Resource:    m.resource,
		Baseline:    &engine.CostResult{Monthly: m.baselineCost, Currency: m.currency},
		Modified:    &engine.CostResult{Monthly: m.modifiedCost, Currency: m.currency},
		TotalChange: m.modifiedCost - m.baselineCost,
		Deltas:      deltas,
	}
}
