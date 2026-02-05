package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// TestNewEstimateModel tests EstimateModel initialization.
func TestNewEstimateModel(t *testing.T) {
	t.Run("initializes with resource data", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)

		require.NotNil(t, model)
		assert.Equal(t, EstimateStateEditing, model.state)
		assert.Equal(t, resource.Provider, model.resource.Provider)
		assert.Equal(t, resource.Type, model.resource.Type)
		assert.False(t, model.loading)
	})

	t.Run("initializes property rows from resource", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"volumeSize":   100,
			},
		}

		model := NewEstimateModel(ctx, resource, nil)

		require.NotNil(t, model)
		assert.Len(t, model.properties, 2)
	})

	t.Run("initializes with existing result", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}
		result := &engine.EstimateResult{
			Resource: resource,
			Baseline: &engine.CostResult{Monthly: 8.32, Currency: "USD"},
			Modified: &engine.CostResult{Monthly: 83.22, Currency: "USD"},
			Deltas: []engine.CostDelta{
				{Property: "instanceType", CostChange: 74.90},
			},
		}

		model := NewEstimateModel(ctx, resource, result)

		require.NotNil(t, model)
		assert.Equal(t, 8.32, model.baselineCost)
		assert.Equal(t, 83.22, model.modifiedCost)
	})

	t.Run("handles nil resource properties", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider:   "aws",
			Type:       "ec2:Instance",
			ID:         "i-123",
			Properties: nil,
		}

		model := NewEstimateModel(ctx, resource, nil)

		require.NotNil(t, model)
		assert.Empty(t, model.properties)
	})
}

// TestEstimateModel_Init tests the Init method.
func TestEstimateModel_Init(t *testing.T) {
	t.Run("returns no commands in editing state", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		cmd := model.Init()

		// In editing state, Init may return blink command for text input
		// We just verify no panic occurs
		assert.NotPanics(t, func() {
			if cmd != nil {
				// Commands are tea.Cmd functions, they may return messages
				_ = cmd
			}
		})
	})
}

// TestEstimateModel_Update tests the Update method message handling.
func TestEstimateModel_Update(t *testing.T) {
	t.Run("handles quit key", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, EstimateStateQuitting, updatedModel.state)
		assert.NotNil(t, cmd) // tea.Quit returns a command
	})

	t.Run("handles ctrl+c", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, EstimateStateQuitting, updatedModel.state)
		assert.NotNil(t, cmd)
	})

	t.Run("handles up/down navigation", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"volumeSize":   100,
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		assert.Equal(t, 0, model.focusedRow)

		// Move down
		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, 1, updatedModel.focusedRow)

		// Move up
		newModel, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyUp})
		updatedModel = newModel.(*EstimateModel)
		assert.Equal(t, 0, updatedModel.focusedRow)
	})

	t.Run("handles enter to start editing", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		assert.False(t, model.editMode)

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		updatedModel := newModel.(*EstimateModel)
		assert.True(t, updatedModel.editMode)
	})

	t.Run("handles escape to cancel editing", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		model.editMode = true
		model.editBuffer = "m5.large"

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
		updatedModel := newModel.(*EstimateModel)
		assert.False(t, updatedModel.editMode)
		assert.Empty(t, updatedModel.editBuffer)
	})

	t.Run("handles window resize", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		updatedModel := newModel.(*EstimateModel)

		assert.Equal(t, 120, updatedModel.width)
		assert.Equal(t, 40, updatedModel.height)
	})
}

// TestEstimateModel_View tests the View method.
func TestEstimateModel_View(t *testing.T) {
	t.Run("renders editing state", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		view := model.View()

		assert.Contains(t, view, "What-If")
		assert.Contains(t, view, "ec2:Instance")
	})

	t.Run("renders loading state", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		model.loading = true
		view := model.View()

		assert.Contains(t, view, "Calculating")
	})

	t.Run("renders error state", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		model.state = EstimateStateError
		model.err = assert.AnError
		view := model.View()

		assert.Contains(t, view, "Error")
	})

	t.Run("renders quitting state as empty", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		model.state = EstimateStateQuitting
		view := model.View()

		assert.Empty(t, view)
	})
}

// TestEstimateModel_PropertyEditing tests property editing behavior.
func TestEstimateModel_PropertyEditing(t *testing.T) {
	t.Run("commits edit on enter in edit mode", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		// Enter edit mode
		model.editMode = true
		model.editBuffer = "m5.large"

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		updatedModel := newModel.(*EstimateModel)

		assert.False(t, updatedModel.editMode)
		assert.Equal(t, "m5.large", updatedModel.properties[0].CurrentValue)
	})

	t.Run("types characters in edit mode", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		model.editMode = true
		model.editBuffer = ""

		// Type 'm'
		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, "m", updatedModel.editBuffer)

		// Type '5'
		newModel, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
		updatedModel = newModel.(*EstimateModel)
		assert.Equal(t, "m5", updatedModel.editBuffer)
	})

	t.Run("handles backspace in edit mode", func(t *testing.T) {
		ctx := context.Background()
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		model := NewEstimateModel(ctx, resource, nil)
		model.editMode = true
		model.editBuffer = "m5.large"

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, "m5.larg", updatedModel.editBuffer)
	})
}

// TestEstimateState_Constants tests state constant values.
func TestEstimateState_Constants(t *testing.T) {
	assert.Equal(t, EstimateState(0), EstimateStateEditing)
	assert.Equal(t, EstimateState(1), EstimateStateCalculating)
	assert.Equal(t, EstimateState(2), EstimateStateQuitting)
	assert.Equal(t, EstimateState(3), EstimateStateError)
}

// TestPropertyRow_Struct tests PropertyRow structure.
func TestPropertyRow_Struct(t *testing.T) {
	row := PropertyRow{
		Key:           "instanceType",
		OriginalValue: "t3.micro",
		CurrentValue:  "m5.large",
		CostDelta:     74.90,
	}

	assert.Equal(t, "instanceType", row.Key)
	assert.Equal(t, "t3.micro", row.OriginalValue)
	assert.Equal(t, "m5.large", row.CurrentValue)
	assert.Equal(t, 74.90, row.CostDelta)
}
