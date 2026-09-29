// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// recordingSupportsClient answers Supports like a region-bound plugin and
// records every request it receives.
type recordingSupportsClient struct {
	mockCostSourceClient

	region string
	seen   []*pbc.ResourceDescriptor
}

func (c *recordingSupportsClient) Supports(
	_ context.Context, req *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	c.seen = append(c.seen, req.GetResource())
	r := req.GetResource()
	return &pbc.SupportsResponse{Supported: r.GetProvider() == "aws" && r.GetRegion() == c.region}, nil
}

func supportsEngine(api proto.CostSourceClient) (*Engine, *pluginhost.Client) {
	client := &pluginhost.Client{Name: "aws-public", API: api}
	return New([]*pluginhost.Client{client}, nil), client
}

func TestCheckPluginSupports_SendsProviderRegionAndSKU(t *testing.T) {
	api := &recordingSupportsClient{region: "us-east-1"}
	e, client := supportsEngine(api)

	ok := e.checkPluginSupports(context.Background(), client, ResourceDescriptor{
		Type: "aws:ec2/instance:Instance", ID: "web", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "us-east-1a"},
	}, "ProjectedCosts")

	assert.True(t, ok)
	require.Len(t, api.seen, 1)
	assert.Equal(t, "aws", api.seen[0].GetProvider())
	assert.Equal(t, "aws:ec2/instance:Instance", api.seen[0].GetResourceType())
	assert.Equal(t, "us-east-1", api.seen[0].GetRegion())
	assert.Equal(t, "m5.large", api.seen[0].GetSku())
}

func TestCheckPluginSupports_CacheKeyIncludesRegion(t *testing.T) {
	api := &recordingSupportsClient{region: "us-east-1"}
	e, client := supportsEngine(api)
	res := func(az string) ResourceDescriptor {
		return ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
			Properties: map[string]interface{}{"instanceType": "t3.micro", "availabilityZone": az}}
	}

	assert.True(t, e.checkPluginSupports(context.Background(), client, res("us-east-1a"), "ProjectedCosts"))
	assert.False(t, e.checkPluginSupports(context.Background(), client, res("us-west-2a"), "ProjectedCosts"),
		"a different region must not reuse the us-east-1 answer")
	assert.True(t, e.checkPluginSupports(context.Background(), client, res("us-east-1b"), "ProjectedCosts"))
	assert.Len(t, api.seen, 2, "same provider, type, region, and feature is served from cache")
}

// skuAwareSupportsClient answers Supports based on SKU alone, like
// azure-public: only VMs with a known SKU are supported.
type skuAwareSupportsClient struct {
	mockCostSourceClient

	seen []*pbc.ResourceDescriptor
}

func (c *skuAwareSupportsClient) Supports(
	_ context.Context, req *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	c.seen = append(c.seen, req.GetResource())
	return &pbc.SupportsResponse{Supported: req.GetResource().GetSku() == "Standard_D2s_v3"}, nil
}

// TestCheckPluginSupports_CacheKeyIncludesSKU covers final-review finding 5:
// the cache key used to omit SKU, so a first SKU-less resource in a region
// (e.g. a VM whose SKU can't be resolved yet) would cache "false" for the
// whole provider/type/region, hiding every other SKU in that region behind
// the same stale answer.
func TestCheckPluginSupports_CacheKeyIncludesSKU(t *testing.T) {
	api := &skuAwareSupportsClient{}
	e, client := supportsEngine(api)
	res := func(sku string) ResourceDescriptor {
		return ResourceDescriptor{Type: "azure:compute/virtualMachine:VirtualMachine", Provider: "azure",
			Properties: map[string]interface{}{"vmSize": sku, "location": "eastus"}}
	}

	assert.False(t, e.checkPluginSupports(context.Background(), client, res(""), "ProjectedCosts"))
	assert.True(t, e.checkPluginSupports(context.Background(), client, res("Standard_D2s_v3"), "ProjectedCosts"),
		"a different SKU in the same provider/type/region must not reuse the SKU-less answer")
	assert.False(t, e.checkPluginSupports(context.Background(), client, res(""), "ProjectedCosts"))
	assert.Len(t, api.seen, 2, "same provider, type, region, sku, and feature is served from cache")
}

// notImplementedSupportsClient mimics a finfocus-spec v0.6.2+ plugin that
// never registered a SupportsProvider: Serve's generic fallback answers
// Supported:false with the SDK's standardized reason (sdk.go).
type notImplementedSupportsClient struct {
	mockCostSourceClient

	calls int
}

func (c *notImplementedSupportsClient) Supports(
	_ context.Context, _ *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	c.calls++
	return &pbc.SupportsResponse{Supported: false, Reason: pluginsdk.DefaultSupportsNotImplementedReason}, nil
}

// TestCheckPluginSupports_NotImplementedReasonFailsOpen covers final-review
// finding 6: a plugin without a SupportsProvider is not making a real
// capability decision, so it must be treated as supported (and cached as
// such), not silently dropped.
func TestCheckPluginSupports_NotImplementedReasonFailsOpen(t *testing.T) {
	api := &notImplementedSupportsClient{}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "kubernetes:apps/v1:Deployment", Provider: "kubernetes"}

	assert.True(t, e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts"))
	assert.True(t, e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts"),
		"the fail-open answer must be cached, not re-queried")
	assert.Equal(t, 1, api.calls)
}

// erroringSupportsClient always fails the Supports RPC.
type erroringSupportsClient struct {
	mockCostSourceClient
}

func (c *erroringSupportsClient) Supports(
	_ context.Context, _ *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return nil, errors.New("supports rpc unavailable")
}

// TestCheckPluginSupports_RPCErrorFailsOpen covers final-review finding 6's
// second requirement: an RPC-level failure (network error, plugin crash,
// unimplemented) must still fail open, exactly like the pre-v0.6.2 (no
// Supports at all) case.
func TestCheckPluginSupports_RPCErrorFailsOpen(t *testing.T) {
	api := &erroringSupportsClient{}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws"}

	assert.True(t, e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts"))
}

// stubSupportsClient delegates Supports to a function so a table can script
// every answer shape a plugin can return.
type stubSupportsClient struct {
	mockCostSourceClient

	supports func(req *pbc.SupportsRequest) (*pbc.SupportsResponse, error)
}

func (c *stubSupportsClient) Supports(
	_ context.Context, req *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return c.supports(req)
}

// TestCheckPluginSupports_FailOpenMatrix is the acceptance matrix for
// issue #1512: only a genuine "not supported" answer filters a plugin out;
// the SDK's not-implemented fallback and every RPC error (including
// Unimplemented) fail open, and answers may vary per region.
func TestCheckPluginSupports_FailOpenMatrix(t *testing.T) {
	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "us-east-1a"}}

	tests := []struct {
		name     string
		supports func(req *pbc.SupportsRequest) (*pbc.SupportsResponse, error)
		want     bool
	}{
		{
			name: "supported",
			supports: func(_ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
				return &pbc.SupportsResponse{Supported: true}, nil
			},
			want: true,
		},
		{
			name: "declined with a reason is filtered out",
			supports: func(_ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
				return &pbc.SupportsResponse{Supported: false, Reason: "region not served"}, nil
			},
			want: false,
		},
		{
			name: "sdk default not-implemented reason fails open",
			supports: func(_ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
				return &pbc.SupportsResponse{
					Supported: false,
					Reason:    pluginsdk.DefaultSupportsNotImplementedReason,
				}, nil
			},
			want: true,
		},
		{
			name: "unimplemented rpc fails open",
			supports: func(_ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
				return nil, status.Error(codes.Unimplemented, "unknown method Supports")
			},
			want: true,
		},
		{
			name: "any other rpc error fails open",
			supports: func(_ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
				return nil, errors.New("connection refused")
			},
			want: true,
		},
		{
			name: "region-specific answer",
			supports: func(req *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
				return &pbc.SupportsResponse{Supported: req.GetResource().GetRegion() == "us-east-1"}, nil
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &stubSupportsClient{supports: tt.supports}
			e, client := supportsEngine(api)

			assert.Equal(t, tt.want, e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts"))
		})
	}
}

// TestCheckPluginSupports_RegionSpecificAnswerDeclines asserts the negative
// side of a region-bound plugin: the same resource type in an unsupported
// region is filtered out.
func TestCheckPluginSupports_RegionSpecificAnswerDeclines(t *testing.T) {
	api := &stubSupportsClient{supports: func(req *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
		return &pbc.SupportsResponse{Supported: req.GetResource().GetRegion() == "eu-west-1"}, nil
	}}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "us-east-1a"}}

	assert.False(t, e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts"))
}
