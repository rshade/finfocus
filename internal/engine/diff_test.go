// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestGetProjectedCostDiff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("all creates match projected total", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
		eng := diffEngine(plugin)
		resources := []ResourceDescriptor{
			diffResource("web", "", "t3.micro", ""),
			diffResource("api", DiffOperationCreate, "m5.large", ""),
		}

		plain, err := eng.GetProjectedCost(ctx, cloneDescriptors(resources))
		require.NoError(t, err)
		diff, err := eng.GetProjectedCostDiff(ctx, resources)
		require.NoError(t, err)

		require.Len(t, diff.Entries, 2)
		assert.Equal(t, 2, diff.Summary.Creates)
		assert.InDelta(t, 50.0, sumMonthly(plain), 1e-9)
		assert.InDelta(t, 50.0, diff.Summary.TotalAfter, 1e-9)
		assert.InDelta(t, 0.0, diff.Summary.TotalBefore, 1e-9)
		assert.InDelta(t, 50.0, diff.Summary.TotalDelta, 1e-9)
		assert.Equal(t, DiffOperationCreate, diff.Entries[0].Operation)
		assert.InDelta(t, 10.0, diff.Entries[0].DeltaMonthly, 1e-9)
		assert.InDelta(t, 0.0, monthlyOf(diff.Entries[0].Before), 1e-9)
	})

	t.Run("mixed create update delete and same", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
		eng := diffEngine(plugin)
		resources := []ResourceDescriptor{
			diffResource("new", DiffOperationCreate, "t3.micro", ""),
			diffResource("resized", DiffOperationUpdate, "m5.large", "t3.micro"),
			diffResource("gone", DiffOperationDelete, "m5.large", "m5.large"),
			diffResource("kept", DiffOperationSame, "t3.micro", ""),
		}

		diff, err := eng.GetProjectedCostDiff(ctx, resources)
		require.NoError(t, err)

		require.Len(t, diff.Entries, 4)
		assert.InDelta(t, 10.0, diff.Entries[0].DeltaMonthly, 1e-9)
		assert.InDelta(t, 30.0, diff.Entries[1].DeltaMonthly, 1e-9)
		assert.InDelta(t, 10.0, monthlyOf(diff.Entries[1].Before), 1e-9)
		assert.InDelta(t, 40.0, monthlyOf(diff.Entries[1].After), 1e-9)
		assert.InDelta(t, -40.0, diff.Entries[2].DeltaMonthly, 1e-9)
		assert.InDelta(t, 0.0, monthlyOf(diff.Entries[2].After), 1e-9)
		assert.InDelta(t, 0.0, diff.Entries[3].DeltaMonthly, 1e-9)
		assert.InDelta(t, monthlyOf(diff.Entries[3].After), monthlyOf(diff.Entries[3].Before), 1e-9)
		assert.Equal(t, 1, diff.Summary.Creates)
		assert.Equal(t, 1, diff.Summary.Updates)
		assert.Equal(t, 1, diff.Summary.Deletes)
		assert.Equal(t, 1, diff.Summary.Unchanged)
		// before 0+10+40+10 = 60; after 10+40+0+10 = 60; delta 0
		assert.InDelta(t, 60.0, diff.Summary.TotalBefore, 1e-9)
		assert.InDelta(t, 60.0, diff.Summary.TotalAfter, 1e-9)
		assert.InDelta(t, 0.0, diff.Summary.TotalDelta, 1e-9)
		assert.Equal(t, 1, plugin.callsFor("kept"))
		assert.Equal(t, 2, plugin.callsFor("resized"))
	})

	t.Run("replace is priced as an update", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
		eng := diffEngine(plugin)
		diff, err := eng.GetProjectedCostDiff(ctx, []ResourceDescriptor{
			diffResource("box", "replace", "m5.large", "t3.micro"),
		})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		assert.Equal(t, DiffOperationUpdate, diff.Entries[0].Operation)
		assert.InDelta(t, 30.0, diff.Entries[0].DeltaMonthly, 1e-9)
		assert.Equal(t, 2, plugin.callsFor("box"))
	})

	t.Run("all same is zero delta", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10}, nil)
		eng := diffEngine(plugin)
		diff, err := eng.GetProjectedCostDiff(ctx, []ResourceDescriptor{
			diffResource("a", DiffOperationSame, "t3.micro", ""),
			diffResource("b", DiffOperationSame, "t3.micro", ""),
		})
		require.NoError(t, err)
		assert.InDelta(t, 0.0, diff.Summary.TotalDelta, 1e-9)
		assert.InDelta(t, 20.0, diff.Summary.TotalAfter, 1e-9)
		assert.InDelta(t, diff.Summary.TotalAfter, diff.Summary.TotalBefore, 1e-9)
		assert.Equal(t, 2, diff.Summary.Unchanged)
		assert.Equal(t, 1, plugin.callsFor("a"))
		assert.Equal(t, 1, plugin.callsFor("b"))
	})

	t.Run("all deletes are a negative total", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
		eng := diffEngine(plugin)
		diff, err := eng.GetProjectedCostDiff(ctx, []ResourceDescriptor{
			diffResource("a", DiffOperationDelete, "", "t3.micro"),
			diffResource("b", "delete-replaced", "", "m5.large"),
		})
		require.NoError(t, err)
		assert.Equal(t, DiffOperationDelete, diff.Entries[0].Operation)
		assert.Equal(t, DiffOperationDelete, diff.Entries[1].Operation)
		assert.InDelta(t, 50.0, diff.Summary.TotalBefore, 1e-9)
		assert.InDelta(t, 0.0, diff.Summary.TotalAfter, 1e-9)
		assert.InDelta(t, -50.0, diff.Summary.TotalDelta, 1e-9)
		assert.Equal(t, 2, diff.Summary.Deletes)
	})

	t.Run("empty plan", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10}, nil)
		eng := diffEngine(plugin)
		diff, err := eng.GetProjectedCostDiff(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, diff)
		assert.Empty(t, diff.Entries)
		assert.InDelta(t, 0.0, diff.Summary.TotalDelta, 1e-9)
		assert.Equal(t, 0, plugin.calls)
	})

	t.Run("property change changes cost", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
		eng := diffEngine(plugin)
		diff, err := eng.GetProjectedCostDiff(ctx, []ResourceDescriptor{
			diffResource("web", DiffOperationUpdate, "m5.large", "t3.micro"),
		})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		assert.InDelta(t, 30.0, diff.Summary.TotalDelta, 1e-9)
		assert.InDelta(t, 40.0, diff.Summary.TotalAfter, 1e-9)
	})

	t.Run("one plugin failure keeps the other resource", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10}, map[string]bool{"bad": true})
		eng := diffEngine(plugin)
		diff, err := eng.GetProjectedCostDiff(ctx, []ResourceDescriptor{
			diffResource("bad", DiffOperationCreate, "t3.micro", ""),
			diffResource("good", DiffOperationCreate, "t3.micro", ""),
		})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 2)
		assert.NotEmpty(t, diff.Errors)
		assert.InDelta(t, 10.0, diff.Entries[1].DeltaMonthly, 1e-9)
		assert.Equal(t, "good", diff.Entries[1].ResourceID)
	})

	t.Run("duplicate ids are priced separately", func(t *testing.T) {
		t.Parallel()
		plugin := newSKUPricePlugin(map[string]float64{"t3.micro": 10, "m5.large": 40}, nil)
		eng := diffEngine(plugin)
		diff, err := eng.GetProjectedCostDiff(ctx, []ResourceDescriptor{
			diffResource("same-id", DiffOperationCreate, "t3.micro", ""),
			diffResource("same-id", DiffOperationCreate, "m5.large", ""),
		})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 2)
		assert.InDelta(t, 10.0, diff.Entries[0].DeltaMonthly, 1e-9)
		assert.InDelta(t, 40.0, diff.Entries[1].DeltaMonthly, 1e-9)
	})
}

func TestRenderProjectedDiff(t *testing.T) {
	t.Parallel()

	diff := sampleDiff()

	t.Run("table", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := RenderProjectedDiff(&buf, OutputTable, diff, true)
		require.NoError(t, err)
		text := buf.String()
		t.Log(text)
		assert.Contains(t, text, "COST DIFF")
		assert.Contains(t, text, "+")
		assert.Contains(t, text, "~")
		assert.Contains(t, text, "-")
		assert.Contains(t, text, "=")
		assert.Contains(t, text, "compute")
		assert.Contains(t, text, "Before")
		assert.Contains(t, text, "After")
		assert.Contains(t, text, "Change")
	})

	t.Run("table keeps plugin notes", func(t *testing.T) {
		t.Parallel()
		noted := sampleDiff()
		noted.Entries[0].After.Notes = "No pricing information available " +
			"(declined by kubernetes: type not served)"
		noted.Entries[2].Before.Notes = "delete baseline note"
		var buf bytes.Buffer
		err := RenderProjectedDiff(&buf, OutputTable, noted, false)
		require.NoError(t, err)
		text := buf.String()
		assert.Contains(t, text, "declined by kubernetes")
		assert.Contains(t, text, "delete baseline note")
	})

	t.Run("json keeps after total and zero before", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := RenderProjectedDiff(&buf, OutputJSON, diff, true)
		require.NoError(t, err)
		payload := buf.String()
		assert.NotContains(t, payload, "├─")

		var decoded struct {
			Finfocus struct {
				Summary struct {
					TotalMonthly float64 `json:"totalMonthly"`
				} `json:"summary"`
				Diff struct {
					TotalBefore float64 `json:"totalBefore"`
					TotalAfter  float64 `json:"totalAfter"`
					TotalDelta  float64 `json:"totalDelta"`
				} `json:"diff"`
				Resources []struct {
					ResourceID    string  `json:"resourceId"`
					Operation     string  `json:"operation"`
					Monthly       float64 `json:"monthly"`
					BeforeMonthly float64 `json:"beforeMonthly"`
					DeltaMonthly  float64 `json:"deltaMonthly"`
				} `json:"resources"`
			} `json:"finfocus"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
		assert.InDelta(t, 60.0, decoded.Finfocus.Summary.TotalMonthly, 1e-9)
		assert.InDelta(t, 60.0, decoded.Finfocus.Diff.TotalBefore, 1e-9)
		assert.InDelta(t, 60.0, decoded.Finfocus.Diff.TotalAfter, 1e-9)
		assert.InDelta(t, 0.0, decoded.Finfocus.Diff.TotalDelta, 1e-9)
		require.Len(t, decoded.Finfocus.Resources, 4)
		assert.Equal(t, "create", decoded.Finfocus.Resources[0].Operation)
		assert.InDelta(t, 0.0, decoded.Finfocus.Resources[0].BeforeMonthly, 1e-9)
		assert.InDelta(t, 10.0, decoded.Finfocus.Resources[0].Monthly, 1e-9)
		assert.Contains(t, payload, `"beforeMonthly": 0`)
	})

	t.Run("ndjson is one entry per line", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := RenderProjectedDiff(&buf, OutputNDJSON, diff, true)
		require.NoError(t, err)
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 4)
		assert.Contains(t, lines[0], `"operation":"create"`)
		assert.Contains(t, lines[2], `"operation":"delete"`)
		assert.Contains(t, lines[3], `"deltaMonthly":0`)
	})
}

func sampleDiff() *DiffResult {
	newRes := diffResource("new", DiffOperationCreate, "t3.micro", "")
	resized := diffResource("resized", DiffOperationUpdate, "m5.large", "t3.micro")
	gone := diffResource("gone", DiffOperationDelete, "m5.large", "m5.large")
	kept := diffResource("kept", DiffOperationSame, "t3.micro", "")
	entries := []DiffEntry{
		finishDiffEntry(newRes, DiffOperationCreate, nil, costOf(newRes, 10), 10),
		finishDiffEntry(resized, DiffOperationUpdate, costOf(resized, 10), costOf(resized, 40), 30),
		finishDiffEntry(gone, DiffOperationDelete, costOf(gone, 40), nil, -40),
		finishDiffEntry(kept, DiffOperationSame, costOf(kept, 10), costOf(kept, 10), 0),
	}
	result := &DiffResult{Entries: entries}
	for i := range entries {
		addDiffSummary(&result.Summary, entries[i])
	}
	result.Summary.TotalDelta = result.Summary.TotalAfter - result.Summary.TotalBefore
	result.Summary.Currency = summaryCurrency(result.Entries)
	return result
}

func costOf(resource ResourceDescriptor, monthly float64) *CostResult {
	return &CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Currency:     "USD",
		Monthly:      monthly,
		Breakdown:    map[string]float64{"compute": monthly * 0.6, "storage": monthly * 0.4},
	}
}

func diffEngine(plugin *skuPricePlugin) *Engine {
	return New([]*pluginhost.Client{{Name: "prices", API: plugin}}, nil)
}

func diffResource(id, op, sku, oldSKU string) ResourceDescriptor {
	resource := ResourceDescriptor{
		Type:       "aws:ec2/instance:Instance",
		ID:         id,
		Provider:   "aws",
		Operation:  op,
		Properties: map[string]any{},
	}
	if sku != "" {
		resource.Properties["instanceType"] = sku
	}
	if oldSKU != "" {
		resource.OldProperties = map[string]any{"instanceType": oldSKU}
	}
	return resource
}

func cloneDescriptors(in []ResourceDescriptor) []ResourceDescriptor {
	out := make([]ResourceDescriptor, len(in))
	copy(out, in)
	return out
}

func sumMonthly(results []CostResult) float64 {
	total := 0.0
	for _, result := range results {
		total += result.Monthly
	}
	return total
}

type skuPricePlugin struct {
	mockCostSourceClient

	mu     sync.Mutex
	prices map[string]float64
	fail   map[string]bool
	calls  int
	byID   map[string]int
}

func newSKUPricePlugin(prices map[string]float64, fail map[string]bool) *skuPricePlugin {
	return &skuPricePlugin{prices: prices, fail: fail, byID: map[string]int{}}
}

func (p *skuPricePlugin) callsFor(id string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.byID[id]
}

func (p *skuPricePlugin) GetProjectedCost(
	_ context.Context, in *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if in == nil || len(in.Resources) == 0 || in.Resources[0] == nil {
		return &proto.GetProjectedCostResponse{}, nil
	}
	resource := in.Resources[0]
	p.byID[resource.ID]++
	if p.fail[resource.ID] {
		return nil, errors.New("plugin down")
	}
	sku := resource.Properties["instanceType"]
	monthly := p.prices[sku]
	return &proto.GetProjectedCostResponse{Results: []*proto.CostResult{{
		Currency:    "USD",
		MonthlyCost: monthly,
		HourlyCost:  monthly / hoursPerMonth,
	}}}, nil
}
