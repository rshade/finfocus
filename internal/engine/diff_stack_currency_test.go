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
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type stackCurrencyPlugin struct {
	*skuPricePlugin

	planCurrency string
}

func (p *stackCurrencyPlugin) GetProjectedCost(
	ctx context.Context,
	req *proto.GetProjectedCostRequest,
	opts ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	response, err := p.skuPricePlugin.GetProjectedCost(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	for _, result := range response.Results {
		result.Currency = "EUR"
		if req.Resources[0].ID == "plan" {
			result.Currency = p.planCurrency
		}
	}
	return response, nil
}

func TestProjectedStackCurrency(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		planCurrency      string
		wantPlanCurrency  string
		wantStackCurrency string
		wantTable         string
	}{
		{
			"EUR unchanged empty plan",
			"",
			"USD",
			"EUR",
			"Stack total: 20.00 EUR/mo across 1 unchanged resources.",
		},
		{
			"EUR plan and state",
			"EUR",
			"EUR",
			"EUR",
			"Stack total: 30.00 EUR/mo across 2 resources (plan total 10.00 EUR/mo)",
		},
		{
			"USD plan and EUR state",
			"USD",
			"USD",
			"",
			"Stack total: 30.00 (mixed currencies)/mo across 2 resources (plan total $10.00/mo)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			state := diffResource("state", "same", "m5.large", "")
			state.Origin = OriginState
			resources := []ResourceDescriptor{state}
			if tt.planCurrency != "" {
				resources = append(resources, diffResource("plan", "create", "t3.micro", ""))
			}
			plugin := &stackCurrencyPlugin{
				skuPricePlugin: newSKUPricePlugin(
					map[string]float64{"t3.micro": 10, "m5.large": 20},
					nil,
				),
				planCurrency: tt.planCurrency,
			}
			diff, err := New(
				[]*pluginhost.Client{{Name: "aws", API: plugin}},
				nil,
			).GetProjectedCostDiff(context.Background(), resources)
			require.NoError(t, err)
			diff.IncludeStack = true
			diff.EmptyPlan = tt.planCurrency == ""
			assert.Equal(t, tt.wantPlanCurrency, diff.Summary.Currency)
			assert.Equal(t, tt.wantStackCurrency, diff.Summary.StackCurrency)
			var out bytes.Buffer
			require.NoError(t, RenderProjectedDiff(&out, OutputJSON, diff, false))
			var doc struct {
				Finfocus struct {
					Stack struct {
						Currency string `json:"currency"`
					} `json:"stack"`
				} `json:"finfocus"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
			assert.Equal(t, tt.wantStackCurrency, doc.Finfocus.Stack.Currency)
			out.Reset()
			require.NoError(t, RenderProjectedDiff(&out, OutputTable, diff, false))
			assert.Contains(t, out.String(), tt.wantTable)
		})
	}
}
