// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectedStackJSONErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		includeStack bool
		complete     bool
	}{
		{name: "complete inventory with pricing failure", includeStack: true, complete: true},
		{name: "incomplete inventory with failure", includeStack: true},
		{name: "legacy output excludes stack failures"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			diff := &DiffResult{
				IncludeStack: tt.includeStack,
				Summary:      DiffSummary{StackComplete: tt.complete},
				Errors: []ErrorDetail{{
					ResourceType: "aws:ec2:Instance", ResourceID: "plan-down",
					PluginName: "aws", Error: errors.New("plan pricing failed"),
				}},
				StackErrors: []ErrorDetail{{
					ResourceType: "aws:s3:Bucket", ResourceID: "state-down",
					PluginName: "aws", Error: errors.New("state pricing failed"),
				}},
			}
			var out bytes.Buffer
			require.NoError(t, RenderProjectedDiff(&out, OutputJSON, diff, false))
			var doc map[string]map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
			assert.JSONEq(t, `[{"resourceType":"aws:ec2:Instance","resourceId":"plan-down",`+
				`"pluginName":"aws","message":"plan pricing failed"}]`, string(doc["finfocus"]["errors"]))
			if !tt.includeStack {
				assert.NotContains(t, doc["finfocus"], "stack")
				assert.NotContains(t, out.String(), "state-down")
				return
			}
			var stack struct {
				Complete bool            `json:"complete"`
				Errors   json.RawMessage `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(doc["finfocus"]["stack"], &stack))
			assert.Equal(t, tt.complete, stack.Complete)
			assert.JSONEq(t, `[{"resourceType":"aws:s3:Bucket","resourceId":"state-down",`+
				`"pluginName":"aws","message":"state pricing failed"}]`, string(stack.Errors))
		})
	}
}

func TestProjectedStackOutput(t *testing.T) {
	t.Parallel()
	diff := &DiffResult{
		IncludeStack: true,
		Summary: DiffSummary{
			TotalAfter:     10,
			Currency:       "USD",
			StackTotal:     30,
			StackCurrency:  "USD",
			StackResources: 2,
			StackComplete:  true,
		},
		Entries: []DiffEntry{
			{
				ResourceID: "changed",
				After:      &CostResult{ResourceID: "changed", Monthly: 10, Currency: "USD"},
			},
		},
		StackUnchanged: []DiffEntry{
			{
				Operation:  "same",
				ResourceID: "unchanged",
				After:      &CostResult{ResourceID: "unchanged", Monthly: 20, Currency: "USD"},
			},
		},
	}
	var out bytes.Buffer
	require.NoError(t, RenderProjectedDiff(&out, OutputJSON, diff, false))
	var doc map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	var stack map[string]any
	require.NoError(t, json.Unmarshal(doc["finfocus"]["stack"], &stack))
	assert.InDelta(t, 30.0, stack["totalMonthly"], 1e-9)
	assert.Equal(t, "USD", stack["currency"])
	assert.Equal(t, true, stack["complete"])
	assert.InDelta(t, 2.0, stack["resources"], 1e-9)
	assert.Len(t, stack["unchanged"], 1)
	assert.NotContains(t, stack, "incompleteReason")
	out.Reset()
	require.NoError(t, RenderProjectedDiff(&out, OutputTable, diff, false))
	assert.Contains(t, out.String(), "UNCHANGED RESOURCES (not in the plan total)")
	assert.Contains(
		t,
		out.String(),
		"Stack total: $30.00/mo across 2 resources (plan total $10.00/mo)",
	)
	out.Reset()
	require.NoError(t, RenderProjectedDiff(&out, OutputNDJSON, diff, false))
	assert.Equal(t, 1, bytes.Count(out.Bytes(), []byte("\n")))
	assert.NotContains(t, out.String(), "unchanged")
	diff.IncludeStack = false
	out.Reset()
	require.NoError(t, RenderProjectedDiff(&out, OutputJSON, diff, false))
	doc = nil
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	assert.NotContains(t, doc["finfocus"], "stack")
}

func TestProjectedIncompleteStackOutput(t *testing.T) {
	t.Parallel()
	diff := &DiffResult{
		IncludeStack: true,
		Summary: DiffSummary{
			TotalAfter:            10,
			Currency:              "USD",
			StackTotal:            10,
			StackCurrency:         "USD",
			StackResources:        1,
			StackIncompleteReason: "not_requested",
			OmittedUnchanged:      4,
		},
	}
	var out bytes.Buffer
	require.NoError(t, RenderProjectedDiff(&out, OutputJSON, diff, false))
	var doc struct {
		Finfocus struct {
			Stack map[string]any `json:"stack"`
		} `json:"finfocus"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	assert.Equal(t, "not_requested", doc.Finfocus.Stack["incompleteReason"])
	assert.InDelta(t, 4.0, doc.Finfocus.Stack["omittedUnchanged"], 1e-9)
	assert.InDelta(t, diff.Summary.TotalAfter, doc.Finfocus.Stack["totalMonthly"], 1e-9)
	assert.Equal(t, false, doc.Finfocus.Stack["complete"])
	assert.NotContains(t, doc.Finfocus.Stack, "unchanged")
	out.Reset()
	require.NoError(t, RenderProjectedDiff(&out, OutputTable, diff, false))
	assert.NotContains(t, out.String(), "UNCHANGED RESOURCES")
	assert.NotContains(t, out.String(), "Stack total:")
}

func TestProjectedEmptyCompleteStackTable(t *testing.T) {
	t.Parallel()
	diff := &DiffResult{
		IncludeStack: true,
		EmptyPlan:    true,
		Summary: DiffSummary{
			StackCurrency:  "USD",
			StackTotal:     30,
			StackResources: 2,
			StackComplete:  true,
		},
	}
	var out bytes.Buffer
	require.NoError(t, RenderProjectedDiff(&out, OutputTable, diff, false))
	assert.Contains(t, out.String(), "No changes. Projected monthly cost is zero.\n")
	assert.Contains(t, out.String(), "Stack total: $30.00/mo across 2 unchanged resources.\n")
	assert.NotContains(t, out.String(), "COST DIFF")
}
