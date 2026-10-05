// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// regionlessRDS mirrors the reproducer in #1670: an aws:rds/instance:Instance
// with engine and instance class but no region or availabilityZone.
func regionlessRDS() ResourceDescriptor {
	return ResourceDescriptor{
		Type:     "aws:rds/instance:Instance",
		ID:       "database",
		Provider: "aws",
		Properties: map[string]any{
			"dbInstanceClass": "db.t3.micro",
			"engine":          "postgres",
		},
	}
}

// TestGetProjectedCostWithErrors_PluginErrorText verifies the engine keeps the
// plugin's failure reason on the row and in the errors list instead of
// replacing it with ErrNoCostData (#1670).
func TestGetProjectedCostWithErrors_PluginErrorText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		projected       *proto.GetProjectedCostResponse
		projectedErr    error
		wantErrContains []string
		wantErrMissing  []string
		wantNote        string
	}{
		{
			name: "invalid argument keeps plugin text",
			projectedErr: status.Error(
				codes.InvalidArgument,
				"region is required for GetProjectedCost",
			),
			wantErrContains: []string{
				"plugin call failed",
				"InvalidArgument",
				"region is required for GetProjectedCost",
			},
			wantErrMissing: []string{"no cost data available"},
			wantNote: "No pricing information available " +
				"(plugin call failed: InvalidArgument: region is required for GetProjectedCost)",
		},
		{
			name:            "plain error passes through",
			projectedErr:    errors.New("connection refused"),
			wantErrContains: []string{"plugin call failed", "connection refused"},
			wantErrMissing:  []string{"no cost data available"},
			wantNote:        "No pricing information available (plugin call failed: connection refused)",
		},
		{
			name:            "empty results keep no cost data",
			projected:       &proto.GetProjectedCostResponse{},
			wantErrContains: []string{"plugin call failed: no cost data available"},
			wantNote:        "No pricing information available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plugin := &pricingSpecPlugin{projected: tt.projected, projectedErr: tt.projectedErr}
			eng := New([]*pluginhost.Client{{Name: "aws-public", API: plugin}}, nil)

			res, err := eng.GetProjectedCostWithErrors(
				context.Background(),
				[]ResourceDescriptor{regionlessRDS()},
			)
			require.NoError(t, err)

			require.True(t, res.HasErrors())
			require.Len(t, res.Errors, 1)
			assert.Equal(t, "aws-public", res.Errors[0].PluginName)
			for _, want := range tt.wantErrContains {
				assert.Contains(t, res.Errors[0].Error.Error(), want)
			}
			for _, unwanted := range tt.wantErrMissing {
				assert.NotContains(t, res.Errors[0].Error.Error(), unwanted)
			}

			require.Len(t, res.Results, 1)
			row := res.Results[0]
			require.NotNil(t, row.Error)
			assert.Equal(t, ErrCodeNoCostData, row.Error.Code)
			assert.Equal(t, tt.wantNote, row.Notes)
			assert.Equal(t, tt.wantNote, row.Error.Message)
		})
	}
}

// TestGetProjectedCost_PluginErrorOnRow verifies the plain GetProjectedCost
// path also keeps the plugin's failure reason on the placeholder row (#1670).
func TestGetProjectedCost_PluginErrorOnRow(t *testing.T) {
	t.Parallel()

	plugin := &pricingSpecPlugin{
		projectedErr: status.Error(
			codes.InvalidArgument,
			"region is required for GetProjectedCost",
		),
	}
	eng := New([]*pluginhost.Client{{Name: "aws-public", API: plugin}}, nil)

	results, err := eng.GetProjectedCost(
		context.Background(),
		[]ResourceDescriptor{regionlessRDS()},
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Notes, "No pricing information available")
	assert.Contains(t, results[0].Notes, "region is required for GetProjectedCost")
	assert.NotContains(t, results[0].Notes, "no cost data available")
}

// TestGetProjectedCostWithErrors_ExplicitRegionPriced confirms a resource with
// an explicit region still prices normally: a successful plugin response is
// used unchanged and no error is recorded (#1670 regression guard).
func TestGetProjectedCostWithErrors_ExplicitRegionPriced(t *testing.T) {
	t.Parallel()

	plugin := &pricingSpecPlugin{
		projected: &proto.GetProjectedCostResponse{Results: []*proto.CostResult{{
			Currency:    "USD",
			MonthlyCost: 15.44,
			HourlyCost:  15.44 / hoursPerMonth,
		}}},
	}
	eng := New([]*pluginhost.Client{{Name: "aws-public", API: plugin}}, nil)

	resource := regionlessRDS()
	resource.Properties["region"] = "us-west-2"

	res, err := eng.GetProjectedCostWithErrors(context.Background(), []ResourceDescriptor{resource})
	require.NoError(t, err)
	assert.False(t, res.HasErrors())
	require.Len(t, res.Results, 1)
	assert.Equal(t, "aws-public", res.Results[0].Adapter)
	assert.InDelta(t, 15.44, res.Results[0].Monthly, 1e-9)
}

func TestPluginStatusError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "grpc status renders code and message",
			err:  status.Error(codes.InvalidArgument, "region is required for GetProjectedCost"),
			want: "InvalidArgument: region is required for GetProjectedCost",
		},
		{
			name: "non-status error passes through",
			err:  errors.New("dial tcp: connection refused"),
			want: "dial tcp: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, pluginStatusError(tt.err).Error())
		})
	}
}
