// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
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
			got := pluginStatusError(tt.err)
			assert.Equal(t, tt.want, got.Error())
			assert.Equal(t, status.Code(tt.err), status.Code(got))
		})
	}
}

func TestProjectedFallbackResult_PluginErrorNote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		declines []pluginDecline
		errs     []ErrorDetail
		want     string
	}{
		{
			name:     "decline reasons kept before plugin error",
			declines: []pluginDecline{{plugin: "kubernetes", reason: "not a workload"}},
			errs:     []ErrorDetail{{Error: errors.New("plugin call failed: InvalidArgument: region is required")}},
			want: "No pricing information available (declined by kubernetes: not a workload)" +
				" (plugin call failed: InvalidArgument: region is required)",
		},
		{
			name: "no cost data errors are skipped",
			errs: []ErrorDetail{
				{Error: fmt.Errorf("plugin call failed: %w", ErrNoCostData)},
				{Error: errors.New("plugin call failed: Unavailable: connection refused")},
			},
			want: "No pricing information available (plugin call failed: Unavailable: connection refused)",
		},
		{
			name: "long plugin error is truncated",
			errs: []ErrorDetail{{Error: errors.New(strings.Repeat("x", maxDeclineReasonLen+10))}},
			want: "No pricing information available (" + strings.Repeat("x", maxDeclineReasonLen) + "...)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			eng := New(nil, nil)
			row := eng.projectedFallbackResult(context.Background(), regionlessRDS(), nil, tt.declines, tt.errs)
			assert.Equal(t, tt.want, row.Notes)
			require.NotNil(t, row.Error)
			assert.Equal(t, tt.want, row.Error.Message)
		})
	}
}

type actualErrPlugin struct {
	mockCostSourceClient

	err error
}

func (p *actualErrPlugin) GetActualCost(
	_ context.Context, _ *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	return nil, p.err
}

func TestGetActualCostWithOptionsAndErrors_PluginErrorText(t *testing.T) {
	t.Parallel()

	plugin := &actualErrPlugin{err: status.Error(codes.FailedPrecondition, "billing data not enabled")}
	eng := New([]*pluginhost.Client{{Name: "aws-ce", API: plugin}}, nil)

	res, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources:        []ResourceDescriptor{regionlessRDS()},
		From:             time.Now().Add(-24 * time.Hour),
		To:               time.Now(),
		FallbackEstimate: true,
	})
	require.NoError(t, err)

	require.Len(t, res.Errors, 1)
	assert.Equal(t, "plugin call failed: FailedPrecondition: billing data not enabled", res.Errors[0].Error.Error())
	require.Len(t, res.Results, 1)
	assert.Equal(t, "ERROR: plugin call failed: FailedPrecondition: billing data not enabled", res.Results[0].Notes)
}
