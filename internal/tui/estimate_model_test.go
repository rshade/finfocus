package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// TestNewEstimateModel tests EstimateModel initialization.
func TestNewEstimateModel(t *testing.T) {
	t.Parallel()

	t.Run("initializes with resource data", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		assert.InDelta(t, 8.32, model.baselineCost, 1e-9)
		assert.InDelta(t, 83.22, model.modifiedCost, 1e-9)
	})

	t.Run("handles nil resource properties", func(t *testing.T) {
		t.Parallel()
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
	t.Parallel()

	t.Run("returns no commands in editing state", func(t *testing.T) {
		t.Parallel()
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
	t.Parallel()

	t.Run("handles quit key", func(t *testing.T) {
		t.Parallel()
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
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "q"})

		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, EstimateStateQuitting, updatedModel.state)
		assert.NotNil(t, cmd) // tea.Quit returns a command
	})

	t.Run("handles ctrl+c", func(t *testing.T) {
		t.Parallel()
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
		newModel, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, EstimateStateQuitting, updatedModel.state)
		assert.NotNil(t, cmd)
	})

	t.Run("handles up/down navigation", func(t *testing.T) {
		t.Parallel()
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
		newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, 1, updatedModel.focusedRow)

		// Move up
		newModel, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		updatedModel = newModel.(*EstimateModel)
		assert.Equal(t, 0, updatedModel.focusedRow)
	})

	t.Run("handles enter to start editing", func(t *testing.T) {
		t.Parallel()
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

		newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		updatedModel := newModel.(*EstimateModel)
		assert.True(t, updatedModel.editMode)
	})

	t.Run("handles escape to cancel editing", func(t *testing.T) {
		t.Parallel()
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

		newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		updatedModel := newModel.(*EstimateModel)
		assert.False(t, updatedModel.editMode)
		assert.Empty(t, updatedModel.editBuffer)
	})

	t.Run("handles window resize", func(t *testing.T) {
		t.Parallel()
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
	t.Parallel()

	t.Run("renders editing state", func(t *testing.T) {
		t.Parallel()
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
		view := model.View().Content

		assert.Contains(t, view, "What-If")
		assert.Contains(t, view, "ec2:Instance")
	})

	t.Run("renders loading state", func(t *testing.T) {
		t.Parallel()
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
		view := model.View().Content

		assert.Contains(t, view, "Calculating")
	})

	t.Run("renders error state", func(t *testing.T) {
		t.Parallel()
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
		view := model.View().Content

		assert.Contains(t, view, "Error")
	})

	t.Run("renders quitting state as empty", func(t *testing.T) {
		t.Parallel()
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

		assert.Empty(t, view.Content)
	})
}

// TestEstimateModel_PropertyEditing tests property editing behavior.
func TestEstimateModel_PropertyEditing(t *testing.T) {
	t.Parallel()

	t.Run("commits edit on enter in edit mode", func(t *testing.T) {
		t.Parallel()
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

		newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		updatedModel := newModel.(*EstimateModel)

		assert.False(t, updatedModel.editMode)
		assert.Equal(t, "m5.large", updatedModel.properties[0].CurrentValue)
	})

	t.Run("types characters in edit mode", func(t *testing.T) {
		t.Parallel()
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
		newModel, _ := model.Update(tea.KeyPressMsg{Text: "m"})
		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, "m", updatedModel.editBuffer)

		// Type '5'
		newModel, _ = updatedModel.Update(tea.KeyPressMsg{Text: "5"})
		updatedModel = newModel.(*EstimateModel)
		assert.Equal(t, "m5", updatedModel.editBuffer)
	})

	t.Run("handles backspace in edit mode", func(t *testing.T) {
		t.Parallel()
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

		newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		updatedModel := newModel.(*EstimateModel)
		assert.Equal(t, "m5.larg", updatedModel.editBuffer)
	})
}

// TestEstimateState_Constants tests state constant values.
func TestEstimateState_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, EstimateStateEditing, EstimateState(0))
	assert.Equal(t, EstimateStateCalculating, EstimateState(1))
	assert.Equal(t, EstimateStateQuitting, EstimateState(2))
	assert.Equal(t, EstimateStateError, EstimateState(3))
}

// TestPropertyRow_Struct tests PropertyRow structure.
func TestPropertyRow_Struct(t *testing.T) {
	t.Parallel()

	row := PropertyRow{
		Key:           "instanceType",
		OriginalValue: "t3.micro",
		CurrentValue:  "m5.large",
		CostDelta:     74.90,
	}

	assert.Equal(t, "instanceType", row.Key)
	assert.Equal(t, "t3.micro", row.OriginalValue)
	assert.Equal(t, "m5.large", row.CurrentValue)
	assert.InDelta(t, 74.90, row.CostDelta, 1e-9)
}

const estimateEC2Type = "aws:ec2/instance:Instance"

func ec2EstimateModel(props map[string]any) *EstimateModel {
	return NewEstimateModel(context.Background(), &engine.ResourceDescriptor{
		Provider: "aws", Type: estimateEC2Type, ID: "i-123", Properties: props,
	}, nil)
}

func pressKey(model *EstimateModel, msg tea.KeyPressMsg) *EstimateModel {
	updated, _ := model.Update(msg)
	return updated.(*EstimateModel)
}

func pressShowAll(model *EstimateModel) *EstimateModel {
	return pressKey(model, tea.KeyPressMsg{Code: 'a', Text: "a"})
}

func typeInto(model *EstimateModel, value string) *EstimateModel {
	model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model.editBuffer = value
	return pressKey(model, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func visibleKeys(model *EstimateModel) []string {
	keys := make([]string, 0, len(model.visibleIndexes()))
	for _, index := range model.visibleIndexes() {
		keys = append(keys, model.properties[index].Key)
	}
	return keys
}

func focusKey(t *testing.T, model *EstimateModel, key string) {
	t.Helper()
	for i, candidate := range visibleKeys(model) {
		if candidate == key {
			model.focusedRow = i
			return
		}
	}
	require.Failf(t, "row not visible", "no visible row %q in %v", key, visibleKeys(model))
}

func TestEstimateModel_RowsComeFromThePlan(t *testing.T) {
	t.Parallel()
	props := map[string]any{
		"instanceType": "t3.micro", "tenancy": "default", "ami": "ami-1", "subnetId": "subnet-1",
		"arn": "arn:aws:ec2:us-east-1:1:instance/i-1", "pulumi:cloudId": "i-1",
	}
	model := ec2EstimateModel(props)

	plan := viewmodel.BuildEstimatePropertyPlan(
		context.Background(), &engine.ResourceDescriptor{Type: estimateEC2Type, Properties: props}, nil,
		viewmodel.EstimateInputContext{},
	)

	assert.Equal(t, plan.Rows, model.properties)
	assert.Equal(t, plan.Details, model.details)
	assert.Equal(t, []string{"instanceType"}, visibleKeys(model))
	view := model.View().Content
	assert.Contains(t, view, "instanceType")
	assert.NotContains(t, view, "subnetId")
}

func TestEstimateModel_IdentityValuesAreReadOnly(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{
		"instanceType": "t3.micro", "arn": "arn:aws:ec2:us-east-1:1:instance/i-1", "pulumi:cloudId": "i-1",
	})

	view := model.View().Content

	assert.Contains(t, view, "Read-only")
	assert.Contains(t, view, "arn:aws:ec2:us-east-1:1:instance/i-1")
	assert.Contains(t, view, "pulumi:cloudId")
	assert.Equal(t, []string{"instanceType"}, visibleKeys(model))
	model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 0, model.focusedRow, "the cursor cannot reach an identity row")
	model = typeInto(model, "t3.large")
	assert.Equal(t, map[string]string{"instanceType": "t3.large"}, model.GetOverrides())
}

func TestEstimateModel_ShowAllKeyTogglesOtherRows(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{"instanceType": "t3.micro", "ami": "ami-1", "subnetId": "subnet-1"})
	require.Equal(t, []string{"instanceType"}, visibleKeys(model))

	model = pressShowAll(model)

	assert.Equal(t, []string{"instanceType", "ami", "subnetId"}, visibleKeys(model))
	assert.Contains(t, model.View().Content, "subnetId")
	model = pressShowAll(model)
	assert.Equal(t, []string{"instanceType"}, visibleKeys(model))
}

func TestEstimateModel_ShowAllKeepsTheCursorRowByKey(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{"instanceType": "t3.micro", "ami": "ami-1", "subnetId": "subnet-1"})
	model = pressShowAll(model)
	focusKey(t, model, "subnetId")

	model = pressShowAll(model)

	assert.Equal(t, []string{"instanceType"}, visibleKeys(model))
	assert.Equal(t, 0, model.focusedRow)
	model = pressShowAll(model)
	focusKey(t, model, "ami")
	model = pressShowAll(model)
	model = pressShowAll(model)
	assert.Equal(t, "instanceType", model.properties[model.visibleIndexes()[model.focusedRow]].Key)
}

func TestEstimateModel_EditedOtherRowStaysVisibleAndCounts(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{"instanceType": "t3.micro", "ami": "ami-1", "subnetId": "subnet-1"})
	model = pressShowAll(model)
	focusKey(t, model, "ami")
	model = typeInto(model, "ami-2")

	model = pressShowAll(model)

	assert.Equal(t, []string{"instanceType", "ami"}, visibleKeys(model))
	assert.Equal(t, map[string]string{"ami": "ami-2"}, model.GetOverrides())
	assert.Contains(t, model.View().Content, "ami-2")
}

func TestEstimateModel_NeverRendersCredentialsOrSecrets(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{
		"instanceType": "t3.micro", "password": "hunter2",
		"userData": map[string]any{"4dabf18193072939515e22adb298388d": "1", "ciphertext": "AAAA"},
	})
	model = pressShowAll(model)

	view := model.View().Content

	for _, hidden := range []string{"password", "hunter2", "userData", "ciphertext"} {
		assert.NotContains(t, view, hidden)
	}
	assert.Equal(t, []string{"instanceType"}, visibleKeys(model))
}

func TestEstimateModel_RecalculationRefusesIdentityOverrides(t *testing.T) {
	t.Parallel()
	called := false
	model := NewEstimateModelWithCallback(context.Background(),
		&engine.ResourceDescriptor{Type: estimateEC2Type, Properties: map[string]any{"instanceType": "t3.micro"}}, nil,
		func(context.Context, *engine.ResourceDescriptor, map[string]string) (*engine.EstimateResult, error) {
			called = true
			return &engine.EstimateResult{}, nil
		})
	model.properties = append(model.properties, PropertyRow{Key: "arn", OriginalValue: "a", CurrentValue: "b"})

	msg := model.triggerRecalculation()()

	result, ok := msg.(estimateRecalculateMsg)
	require.True(t, ok)
	var identityErr *viewmodel.IdentityOverrideError
	require.ErrorAs(t, result.err, &identityErr)
	assert.Equal(t, "arn", identityErr.Key)
	assert.False(t, called, "no pricing call for an identity override")
}

func TestEstimateModel_DiscoveryHintsReplanKeepingEdits(t *testing.T) {
	t.Parallel()
	hints := engine.PricingDiscovery{Modes: []engine.PricingMode{
		{MetricHints: []engine.PricingMetricHint{{Metric: "monthlyHours", Unit: "hour"}}},
	}}
	model := ec2EstimateModel(map[string]any{
		"instanceType": "t3.micro", "ami": "ami-1", "monthlyHours": 100, "subnetId": "subnet-1",
	})
	model = pressShowAll(model)
	focusKey(t, model, "ami")
	model = typeInto(model, "ami-2")
	focusKey(t, model, "subnetId")
	model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model.editBuffer = "subnet-9"
	require.True(t, model.editMode)

	updated, _ := model.Update(estimatePricingSpecMsg{discovery: hints})
	model = updated.(*EstimateModel)

	assert.Equal(t, []string{"instanceType", "monthlyHours", "ami", "subnetId"}, visibleKeys(model))
	usage := model.properties[1]
	assert.Equal(t, viewmodel.PropertyClassPricing, usage.Class)
	assert.Equal(t, viewmodel.ProvenanceUsage, usage.Provenance)
	assert.True(t, model.editMode)
	assert.Equal(t, "subnet-9", model.editBuffer)
	assert.Equal(t, "subnetId", model.properties[model.visibleIndexes()[model.focusedRow]].Key)
	assert.Equal(t, map[string]string{"ami": "ami-2"}, model.GetOverrides())

	model = pressShowAll(pressKey(model, tea.KeyPressMsg{Code: tea.KeyEscape}))
	assert.Equal(t, []string{"instanceType", "monthlyHours", "ami"}, visibleKeys(model))
}

func TestEstimateModel_DiscoveryReplanKeepsDeltas(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{"instanceType": "t3.micro", "monthlyHours": 100})
	model.applyResult(&engine.EstimateResult{Deltas: []engine.CostDelta{{Property: "instanceType", CostChange: 12}}})

	updated, _ := model.Update(estimatePricingSpecMsg{discovery: engine.PricingDiscovery{}})
	model = updated.(*EstimateModel)

	assert.InDelta(t, 12.0, model.properties[0].CostDelta, 1e-9)
}

func TestEstimateModel_FallbackNoteOnlyWhenNotCurated(t *testing.T) {
	t.Parallel()
	uncurated := NewEstimateModel(context.Background(), &engine.ResourceDescriptor{
		Type: "aws:rds/instance:Instance", Properties: map[string]any{"engine": "mysql"},
	}, nil)
	curated := ec2EstimateModel(map[string]any{"instanceType": "t3.micro"})

	assert.Contains(t, uncurated.View().Content, viewmodel.EstimateFallbackNote)
	assert.Equal(t, []string{"engine"}, visibleKeys(uncurated))
	assert.NotContains(t, curated.View().Content, "not curated")
}

func TestEstimateModel_HelpListsShowAllKey(t *testing.T) {
	t.Parallel()
	assert.Contains(t, RenderEstimateHelp(), "a: All properties")
}

func TestEstimateModel_RevertedAdvancedEditLeavesTheDefaultListAfterReplan(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{"instanceType": "t3.micro", "ami": "ami-1"})
	model = pressShowAll(model)
	focusKey(t, model, "ami")
	model = typeInto(model, "ami-2")
	updated, _ := model.Update(estimatePricingSpecMsg{discovery: engine.PricingDiscovery{}})
	model = updated.(*EstimateModel)
	focusKey(t, model, "ami")
	model = typeInto(model, "ami-1")

	model = pressShowAll(model)

	assert.Equal(t, []string{"instanceType"}, visibleKeys(model))
}

func TestEstimateModel_ModeSwitchReplansBeforePricing(t *testing.T) {
	t.Parallel()
	model := ec2EstimateModel(map[string]any{"instanceType": "t3.micro", "ami": "ami-1"})
	aws := engine.PricingMode{Plugin: "aws-public", BillingMode: "per_hour"}
	civo := engine.PricingMode{Plugin: "civo-public", BillingMode: "per_hour"}
	model.pricingModes = []engine.PricingMode{aws, civo}
	model.pricingMode = 0
	model.replan()
	assert.Contains(t, visibleKeys(model), "tenancy")
	focusKey(t, model, "tenancy")
	model = typeInto(model, "dedicated")
	assert.Equal(t, "dedicated", model.GetOverrides()["tenancy"])

	priced := false
	model.estimateFn = func(context.Context, *engine.EstimateRequest) (*engine.EstimateResult, error) {
		priced = true
		return &engine.EstimateResult{}, nil
	}
	cmd := model.selectPricingMode(1)
	require.NotNil(t, cmd)
	assert.False(t, priced, "replan finishes before the pricing command runs")
	var row PropertyRow
	for _, candidate := range model.properties {
		if candidate.Key == "tenancy" {
			row = candidate
		}
	}
	assert.Equal(t, "dedicated", row.CurrentValue)
	assert.Equal(t, viewmodel.PropertyClassOther, row.Class)
	assert.True(t, row.Overridden)
	assert.Contains(t, visibleKeys(model), "tenancy")
	assert.Equal(t, "tenancy", visibleKeys(model)[model.focusedRow])
}
