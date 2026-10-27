// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestDescriptorOriginIsInternal(t *testing.T) {
	t.Parallel()
	resource := diffResource("web", "same", "t3.micro", "")
	assert.Equal(t, OriginPlan, resource.Origin)
	baseline, err := json.Marshal(resource)
	require.NoError(t, err)
	resource.Origin = OriginState
	encoded, err := json.Marshal(resource)
	require.NoError(t, err)
	assert.JSONEq(t, string(baseline), string(encoded))
}

func TestProjectedDiffOriginSplit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		origins              []DescriptorOrigin
		wantPlan, wantStack  int
		wantAfter, wantTotal float64
		wantComplete         bool
	}{
		{"plan only", []DescriptorOrigin{OriginPlan, OriginPlan}, 2, 0, 50, 50, false},
		{"mixed", []DescriptorOrigin{OriginState, OriginPlan}, 1, 1, 40, 50, true},
		{
			"state only inventory uses plan origin",
			[]DescriptorOrigin{OriginPlan, OriginPlan},
			2,
			0,
			50,
			50,
			false,
		},
		{"all omitted state", []DescriptorOrigin{OriginState, OriginState}, 0, 2, 0, 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
			resources := []ResourceDescriptor{
				diffResource("a", "same", "t3.micro", ""),
				diffResource("b", "same", "m5.large", ""),
			}
			for i := range resources {
				resources[i].Origin = tt.origins[i]
			}
			got, err := diffEngine(plugin).GetProjectedCostDiff(context.Background(), resources)
			require.NoError(t, err)
			assert.Len(t, got.Entries, tt.wantPlan)
			assert.Len(t, got.StackUnchanged, tt.wantStack)
			assert.InDelta(t, tt.wantAfter, got.Summary.TotalAfter, 1e-9)
			assert.InDelta(t, tt.wantAfter, sumMonthly(got.AfterCosts()), 1e-9)
			assert.InDelta(t, tt.wantTotal, got.Summary.StackTotal, 1e-9)
			assert.Equal(t, 2, got.Summary.StackResources)
			assert.Equal(t, tt.wantComplete, got.Summary.StackComplete)
			assert.Equal(t, tt.wantPlan, got.Summary.Unchanged)
			assert.Equal(t, 1, plugin.callsFor("a"))
			assert.Equal(t, 1, plugin.callsFor("b"))
		})
	}
}

func TestProjectedDiffOriginsPreservePlan(t *testing.T) {
	t.Parallel()
	resources := []ResourceDescriptor{
		diffResource("new", "create", "t3.micro", ""),
		diffResource("resize", "update", "m5.large", "t3.micro"),
		diffResource("gone", "delete", "m5.large", "m5.large"),
		diffResource("kept", "same", "t3.micro", ""),
	}
	prices := map[string]float64{"t3.micro": 10, "m5.large": 40}
	plan, err := diffEngine(
		newSKUPricePlugin(prices, nil),
	).GetProjectedCostDiff(context.Background(), resources)
	require.NoError(t, err)
	state := diffResource("omitted", "same", "m5.large", "")
	state.Origin = OriginState
	combined, err := diffEngine(
		newSKUPricePlugin(prices, nil),
	).GetProjectedCostDiff(context.Background(), append(resources, state))
	require.NoError(t, err)
	assert.Equal(t, plan.Entries, combined.Entries)
	assert.InDelta(t, plan.Summary.TotalBefore, combined.Summary.TotalBefore, 1e-9)
	assert.InDelta(t, plan.Summary.TotalAfter, combined.Summary.TotalAfter, 1e-9)
	assert.InDelta(t, plan.Summary.TotalDelta, combined.Summary.TotalDelta, 1e-9)
	assert.Equal(t, plan.Summary.Creates, combined.Summary.Creates)
	assert.Equal(t, plan.Summary.Updates, combined.Summary.Updates)
	assert.Equal(t, plan.Summary.Deletes, combined.Summary.Deletes)
	assert.Equal(t, plan.Summary.Unchanged, combined.Summary.Unchanged)
	assert.InDelta(t, plan.Summary.TotalAfter+40, combined.Summary.StackTotal, 1e-9)
}

func TestProjectedDiffPlanWinsOverStateDuplicate(t *testing.T) {
	t.Parallel()
	plan := diffResource("same-urn", "same", "m5.large", "")
	state := diffResource("same-urn", "same", "t3.micro", "")
	state.Origin = OriginState
	plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
	got, err := diffEngine(
		plugin,
	).GetProjectedCostDiff(context.Background(), []ResourceDescriptor{state, plan})
	require.NoError(t, err)
	require.Len(t, got.Entries, 1)
	assert.Empty(t, got.StackUnchanged)
	assert.InDelta(t, 40.0, got.Summary.TotalAfter, 1e-9)
	assert.InDelta(t, 40.0, got.Summary.StackTotal, 1e-9)
	assert.Equal(t, 1, got.Summary.StackResources)
	assert.Equal(t, 1, plugin.callsFor("same-urn"))
}

func TestProjectedDiffOriginWithoutResourceIDs(t *testing.T) {
	t.Parallel()
	state := diffResource("", "same", "t3.micro", "")
	state.Origin = OriginState
	plan := diffResource("", "same", "m5.large", "")
	got, err := diffEngine(
		newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil),
	).GetProjectedCostDiff(context.Background(), []ResourceDescriptor{state, plan})
	require.NoError(t, err)
	require.Len(t, got.Entries, 1)
	require.Len(t, got.StackUnchanged, 1)
	assert.InDelta(t, 40.0, got.Summary.TotalAfter, 1e-9)
	assert.InDelta(t, 50.0, got.Summary.StackTotal, 1e-9)
}

func TestDiffStackMetadataJSON(t *testing.T) {
	t.Parallel()
	baseline, err := json.Marshal(DiffSummary{})
	require.NoError(t, err)
	summary := DiffSummary{
		StackTotal:            50,
		StackCurrency:         "EUR",
		StackResources:        2,
		StackComplete:         true,
		StackIncompleteReason: "read_failed",
		OmittedUnchanged:      2,
	}
	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	assert.JSONEq(
		t,
		string(baseline),
		string(encoded),
		"stack metadata belongs in the renderer's stack object",
	)
	assert.False(t, (&DiffResult{}).IncludeStack, "legacy callers retain their existing output")
}

func TestProjectedDiffOriginExplanations(t *testing.T) {
	t.Parallel()
	plugin := explainPlugin(explainSpec(), nil)
	eng := New([]*pluginhost.Client{{Name: "aws", API: plugin}}, nil).WithExplainPricing(true)
	state := diffResource("state", "same", "t3.micro", "")
	state.Origin = OriginState
	plan := diffResource("plan", "create", "m5.large", "")
	internal := ResourceDescriptor{ID: "stack", Type: "pulumi:pulumi:Stack"}
	got, err := eng.GetProjectedCostDiff(
		context.Background(),
		[]ResourceDescriptor{internal, state, plan},
	)
	require.NoError(t, err)
	require.Len(t, got.Entries, 1)
	require.Len(t, got.StackUnchanged, 1)
	require.NotNil(t, got.Entries[0].PricingSpec)
	require.NotNil(t, got.StackUnchanged[0].PricingSpec)
	assert.Equal(t, "plan", plugin.lastSpecID)
	assert.Equal(t, "m5.large", plugin.lastSKU)
	assert.Equal(t, 2, plugin.specCalls)
	assert.Equal(t, 2, got.Summary.StackResources)
}

func TestProjectedDiffDuplicateStateInventory(t *testing.T) {
	t.Parallel()
	state := diffResource("state", "same", "t3.micro", "")
	state.Origin = OriginState
	plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10}, nil)
	got, err := diffEngine(
		plugin,
	).GetProjectedCostDiff(context.Background(), []ResourceDescriptor{state, state})
	require.NoError(t, err)
	assert.Empty(t, got.Entries)
	assert.Len(t, got.StackUnchanged, 1)
	assert.InDelta(t, 10.0, got.Summary.StackTotal, 1e-9)
	assert.Equal(t, 1, got.Summary.StackResources)
	assert.Equal(t, 1, plugin.callsFor("state"))
}

func TestProjectedDiffStateErrorsStayOutOfPlanErrors(t *testing.T) {
	t.Parallel()
	state := diffResource("state-down", "same", "t3.micro", "")
	state.Origin = OriginState
	plan := diffResource("plan-down", "create", "m5.large", "")
	plugin := newSKUPricePlugin(
		map[string]float64{"t3.micro": 10, "m5.large": 40},
		map[string]bool{"state-down": true, "plan-down": true},
	)
	got, err := diffEngine(plugin).GetProjectedCostDiff(context.Background(), []ResourceDescriptor{state, plan})
	require.NoError(t, err)
	require.Len(t, got.Errors, 1)
	assert.Equal(t, "plan-down", got.Errors[0].ResourceID)
	require.Len(t, got.StackErrors, 1)
	assert.Equal(t, "state-down", got.StackErrors[0].ResourceID)

	var legacy bytes.Buffer
	require.NoError(t, renderDiffTable(&legacy, got, false))
	assert.Contains(t, legacy.String(), "plan-down")
	assert.NotContains(t, legacy.String(), "state-down", "a v0.5.2 caller sees plan errors only")
	var doc struct {
		Finfocus struct {
			Errors []diffErrorJSON `json:"errors"`
		} `json:"finfocus"`
	}
	var encoded bytes.Buffer
	require.NoError(t, renderDiffJSON(&encoded, got))
	require.NoError(t, json.Unmarshal(encoded.Bytes(), &doc))
	require.Len(t, doc.Finfocus.Errors, 1)
	assert.Equal(t, "plan-down", doc.Finfocus.Errors[0].ResourceID)

	got.IncludeStack = true
	var stacked bytes.Buffer
	require.NoError(t, renderDiffTable(&stacked, got, false))
	assert.Contains(t, stacked.String(), "state-down", "the stack section reports its own failures")
}
