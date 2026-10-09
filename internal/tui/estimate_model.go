package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
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
type PropertyRow = viewmodel.EstimatePropertyRow

// estimateRecalculateMsg is sent when cost recalculation completes.
type estimateRecalculateMsg struct {
	result     *engine.EstimateResult
	err        error
	generation uint64
}

// estimatePricingSpecMsg delivers GetPricingSpec discovery for the open resource.
type estimatePricingSpecMsg struct {
	discovery engine.PricingDiscovery
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

	// A mode-aware callback shares the engine request without changing legacy callers.
	estimateFn func(context.Context, *engine.EstimateRequest) (*engine.EstimateResult, error)
	generation uint64

	// Plugin pricing-spec discovery. Empty modes leave the estimate editable.
	discoverFn         func(context.Context, *engine.ResourceDescriptor) engine.PricingDiscovery
	pricingModes       []engine.PricingMode
	pricingMode        int
	pricingSpecLoading bool
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
		currency: defaultEstimateCurrency,
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

// WithPricingDiscovery loads GetPricingSpec when the TUI starts.
// A nil discovery function leaves the estimate view unchanged.
func (m *EstimateModel) WithPricingDiscovery(
	fn func(context.Context, *engine.ResourceDescriptor) engine.PricingDiscovery,
) *EstimateModel {
	if m == nil {
		return nil
	}
	m.discoverFn = fn
	return m
}

// WithEstimateCallback installs shared provider-aware estimation. The callback
// handles unchanged properties with Engine.EstimateBaseline.
func (m *EstimateModel) WithEstimateCallback(
	fn func(context.Context, *engine.EstimateRequest) (*engine.EstimateResult, error),
) *EstimateModel {
	if m != nil {
		m.estimateFn = fn
	}
	return m
}

// initializeProperties extracts properties from the resource into editable rows.
func (m *EstimateModel) initializeProperties() {
	if m.resource == nil || m.resource.Properties == nil {
		m.properties = []PropertyRow{}
		return
	}

	m.properties = viewmodel.BuildEstimatePropertyRows(m.resource.Properties, nil)
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

	viewmodel.ApplyEstimateDeltas(m.properties, result.Deltas)
}

// Init initializes the model and starts pricing-spec discovery when configured.
func (m *EstimateModel) Init() tea.Cmd {
	if m.discoverFn == nil || m.resource == nil || strings.TrimSpace(m.resource.Type) == "" {
		return nil
	}
	m.pricingSpecLoading = true
	ctx := m.ctx
	resource := m.resource
	discoverFn := m.discoverFn
	return func() tea.Msg {
		return estimatePricingSpecMsg{discovery: discoverFn(ctx, resource)}
	}
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

	case estimatePricingSpecMsg:
		m.pricingSpecLoading = false
		m.pricingModes = msg.discovery.Modes
		m.pricingMode = 0
		if m.estimateFn != nil && len(m.pricingModes) > 0 {
			return m, m.triggerRecalculation()
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// handleKeyMsg processes keyboard input.
func (m *EstimateModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle edit mode separately
	if m.editMode {
		return m.handleEditModeKey(msg)
	}

	// Handle ctrl+c (no Code constant in v2, uses modifier)
	if msg.String() == "ctrl+c" {
		m.state = EstimateStateQuitting
		return m, tea.Quit
	}

	switch msg.Code {
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

	case tea.KeyLeft:
		return m, m.selectPricingMode(m.pricingMode - 1)
	case tea.KeyRight:
		return m, m.selectPricingMode(m.pricingMode + 1)

	case tea.KeyEnter:
		if len(m.properties) > 0 && m.focusedRow < len(m.properties) {
			m.editMode = true
			m.editBuffer = m.properties[m.focusedRow].CurrentValue
		}
		return m, nil

	case tea.KeyEscape:
		// Clear any pending changes
		return m, nil

	default:
		if msg.Text == "q" {
			m.state = EstimateStateQuitting
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *EstimateModel) selectPricingMode(index int) tea.Cmd {
	if index < 0 || index >= len(m.pricingModes) || index == m.pricingMode {
		return nil
	}
	m.pricingMode = index
	if m.estimateFn != nil || m.recalculateFn != nil {
		return m.triggerRecalculation()
	}
	return nil
}

// handleEditModeKey processes keyboard input while editing a property.
func (m *EstimateModel) handleEditModeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEnter:
		// Commit the edit
		if m.focusedRow < len(m.properties) {
			m.properties[m.focusedRow].CurrentValue = m.editBuffer
		}
		m.editMode = false

		// Trigger recalculation if callback is set
		if m.recalculateFn != nil || m.estimateFn != nil {
			return m, m.triggerRecalculation()
		}
		return m, nil

	case tea.KeyEscape:
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

	default:
		if msg.Text != "" {
			m.editBuffer += msg.Text
		}
		return m, nil
	}
}

// triggerRecalculation creates a command to recalculate costs.
func (m *EstimateModel) triggerRecalculation() tea.Cmd {
	m.loading = true
	m.generation++

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
	estimateFn := m.estimateFn
	generation := m.generation
	mode := ""
	if len(m.pricingModes) > 0 && m.pricingMode < len(m.pricingModes) {
		mode = m.pricingModes[m.pricingMode].ID()
	}

	return func() tea.Msg {
		var result *engine.EstimateResult
		var err error
		if estimateFn != nil {
			result, err = estimateFn(
				ctx,
				&engine.EstimateRequest{Resource: resource, PropertyOverrides: overrides, PricingMode: mode},
			)
		} else {
			result, err = recalculateFn(ctx, resource, overrides)
		}
		return estimateRecalculateMsg{result: result, err: err, generation: generation}
	}
}

// handleRecalculateComplete processes the result of a cost recalculation.
func (m *EstimateModel) handleRecalculateComplete(msg estimateRecalculateMsg) (tea.Model, tea.Cmd) {
	if msg.generation != m.generation {
		return m, nil
	}
	m.loading = false

	if msg.err != nil {
		m.err = msg.err
		m.state = EstimateStateError
		return m, nil
	}

	m.err = nil
	m.state = EstimateStateEditing
	if msg.result != nil {
		m.applyResult(msg.result)
	}

	return m, nil
}

// View renders the current view.
func (m *EstimateModel) View() tea.View {
	switch m.state {
	case EstimateStateQuitting:
		return tea.NewView("")

	case EstimateStateError:
		return tea.NewView(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))

	case EstimateStateEditing, EstimateStateCalculating:
		// Handled below
	}

	if m.loading {
		return tea.NewView(RenderLoadingIndicator())
	}

	return tea.NewView(m.renderEditingView())
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

	if section := m.renderPricingSection(); section != "" {
		output += section
		output += "\n\n"
	}

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
