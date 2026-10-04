// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

// supportsOnly unwraps checkPluginSupports for tests that only care about the
// boolean (fail-open matrix, cache behavior).
func supportsOnly(e *Engine, client *pluginhost.Client, res ResourceDescriptor) bool {
	ok, _ := e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts")
	return ok
}

func TestCheckPluginSupports_SendsProviderRegionAndSKU(t *testing.T) {
	t.Parallel()

	api := &recordingSupportsClient{region: "us-east-1"}
	e, client := supportsEngine(api)

	ok, reason := e.checkPluginSupports(context.Background(), client, ResourceDescriptor{
		Type: "aws:ec2/instance:Instance", ID: "web", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "us-east-1a"},
	}, "ProjectedCosts")

	assert.True(t, ok)
	assert.Empty(t, reason, "a supported answer carries no decline reason")
	require.Len(t, api.seen, 1)
	assert.Equal(t, "aws", api.seen[0].GetProvider())
	assert.Equal(t, "aws:ec2/instance:Instance", api.seen[0].GetResourceType())
	assert.Equal(t, "us-east-1", api.seen[0].GetRegion())
	assert.Equal(t, "m5.large", api.seen[0].GetSku())
}

func TestCheckPluginSupports_CacheKeyIncludesRegion(t *testing.T) {
	t.Parallel()

	api := &recordingSupportsClient{region: "us-east-1"}
	e, client := supportsEngine(api)
	res := func(az string) ResourceDescriptor {
		return ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
			Properties: map[string]interface{}{"instanceType": "t3.micro", "availabilityZone": az}}
	}

	assert.True(t, supportsOnly(e, client, res("us-east-1a")))
	assert.False(t, supportsOnly(e, client, res("us-west-2a")),
		"a different region must not reuse the us-east-1 answer")
	assert.True(t, supportsOnly(e, client, res("us-east-1a")))
	assert.Len(t, api.seen, 2, "the same resource again is served from cache")
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
	t.Parallel()

	api := &skuAwareSupportsClient{}
	e, client := supportsEngine(api)
	res := func(sku string) ResourceDescriptor {
		return ResourceDescriptor{Type: "azure:compute/virtualMachine:VirtualMachine", Provider: "azure",
			Properties: map[string]interface{}{"vmSize": sku, "location": "eastus"}}
	}

	assert.False(t, supportsOnly(e, client, res("")))
	assert.True(t, supportsOnly(e, client, res("Standard_D2s_v3")),
		"a different SKU in the same provider/type/region must not reuse the SKU-less answer")
	assert.False(t, supportsOnly(e, client, res("")))
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
	t.Parallel()

	api := &notImplementedSupportsClient{}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "kubernetes:apps/v1:Deployment", Provider: "kubernetes"}

	assert.True(t, supportsOnly(e, client, res))
	assert.True(t, supportsOnly(e, client, res),
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
	t.Parallel()

	api := &erroringSupportsClient{}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws"}

	assert.True(t, supportsOnly(e, client, res))
}

// TestCheckPluginSupports_RPCErrorReturnsNoReason ensures a fail-open RPC
// failure never fabricates a decline reason.
func TestCheckPluginSupports_RPCErrorReturnsNoReason(t *testing.T) {
	t.Parallel()

	api := &erroringSupportsClient{}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws"}

	ok, reason := e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts")
	assert.True(t, ok)
	assert.Empty(t, reason)
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
	t.Parallel()

	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "us-east-1a"}}

	tests := []struct {
		name       string
		supports   func(req *pbc.SupportsRequest) (*pbc.SupportsResponse, error)
		want       bool
		wantReason string
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
			want:       false,
			wantReason: "region not served",
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
			t.Parallel()
			api := &stubSupportsClient{supports: tt.supports}
			e, client := supportsEngine(api)

			got, reason := e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts")
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

// TestCheckPluginSupports_RegionSpecificAnswerDeclines asserts the negative
// side of a region-bound plugin: the same resource type in an unsupported
// region is filtered out.
func TestCheckPluginSupports_RegionSpecificAnswerDeclines(t *testing.T) {
	t.Parallel()

	api := &stubSupportsClient{supports: func(req *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
		return &pbc.SupportsResponse{Supported: req.GetResource().GetRegion() == "eu-west-1"}, nil
	}}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "us-east-1a"}}

	assert.False(t, supportsOnly(e, client, res))
}

// TestCheckPluginSupports_DeclineReasonSurvivesCache ensures the decline
// reason is returned on the cached path too, not just the live RPC answer.
func TestCheckPluginSupports_DeclineReasonSurvivesCache(t *testing.T) {
	t.Parallel()

	const wantReason = "Region not supported by this binary (plugin region: us-east-1)"
	calls := 0
	api := &stubSupportsClient{supports: func(_ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
		calls++
		return &pbc.SupportsResponse{Supported: false, Reason: wantReason}, nil
	}}
	e, client := supportsEngine(api)
	res := ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "eu-west-1a"}}

	for range 2 {
		ok, reason := e.checkPluginSupports(context.Background(), client, res, "ProjectedCosts")
		assert.False(t, ok)
		assert.Equal(t, wantReason, reason)
	}
	assert.Equal(t, 1, calls, "second call must be served from cache")
}

// decliningSupportsClient always declines with a fixed reason, like a
// region-pinned binary answering for a resource outside its region.
type decliningSupportsClient struct {
	mockCostSourceClient

	reason string
}

func (c *decliningSupportsClient) Supports(
	_ context.Context, _ *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: false, Reason: c.reason}, nil
}

// TestGetProjectedCost_DeclineReasonsSurfaced covers issue #1515: when every
// candidate plugin declines Supports() with a distinct reason, the placeholder
// CostResult carries both reasons instead of the bare "no pricing" note.
func TestGetProjectedCost_DeclineReasonsSurfaced(t *testing.T) {
	t.Parallel()

	const reasonA = "Region not supported by this binary (plugin region: us-east-1)"
	const reasonB = "SKU m5.large not priced by this plugin"

	clients := []*pluginhost.Client{
		{Name: "aws-public-use1", API: &decliningSupportsClient{reason: reasonA}},
		{Name: "aws-public-euw1", API: &decliningSupportsClient{reason: reasonB}},
	}
	e := New(clients, nil)

	resources := []ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	results, err := e.GetProjectedCost(context.Background(), resources)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, adapterNone, result.Adapter)
	assert.Contains(t, result.Notes, noteNoPricingInfo)
	assert.Contains(t, result.Notes, "aws-public-use1: "+reasonA)
	assert.Contains(t, result.Notes, "aws-public-euw1: "+reasonB)
	require.NotNil(t, result.Error)
	assert.Equal(t, ErrCodeNoCostData, result.Error.Code)
	assert.Contains(t, result.Error.Message, reasonA)
	assert.Contains(t, result.Error.Message, reasonB)
}

// TestGetProjectedCost_FailOpenNotReportedAsDecline covers issue #1515: a
// plugin whose Supports() RPC fails fails open (stays selected, contributes
// no decline reason), while a genuine decline is still surfaced.
func TestGetProjectedCost_FailOpenNotReportedAsDecline(t *testing.T) {
	t.Parallel()

	const declineReason = "region not served by this binary"

	clients := []*pluginhost.Client{
		{Name: "flaky-plugin", API: &erroringSupportsClient{}},
		{Name: "aws-public", API: &decliningSupportsClient{reason: declineReason}},
	}
	e := New(clients, nil)

	resources := []ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	results, err := e.GetProjectedCost(context.Background(), resources)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, adapterNone, result.Adapter)
	assert.Contains(t, result.Notes, "aws-public: "+declineReason)
	assert.NotContains(t, result.Notes, "flaky-plugin",
		"a fail-open RPC error must not be reported as a decline")
	require.NotNil(t, result.Error)
	assert.NotContains(t, result.Error.Message, "flaky-plugin")
}

// TestGetProjectedCostWithErrors_DeclineReasonsSurfaced applies the same
// surfacing to the error-tracking projected-cost path.
func TestGetProjectedCostWithErrors_DeclineReasonsSurfaced(t *testing.T) {
	t.Parallel()

	const reason = "region not served by this binary"

	clients := []*pluginhost.Client{
		{Name: "aws-public", API: &decliningSupportsClient{reason: reason}},
	}
	e := New(clients, nil)

	resources := []ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	result, err := e.GetProjectedCostWithErrors(context.Background(), resources)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Results, 1)
	assert.Contains(t, result.Results[0].Notes, reason)
}

// TestGetActualCost_DeclineReasonsSurfaced applies the same surfacing to the
// actual-cost path: the placeholder note names the declining plugin.
func TestGetActualCost_DeclineReasonsSurfaced(t *testing.T) {
	t.Parallel()

	const reason = "Region not supported by this binary (plugin region: us-east-1)"

	clients := []*pluginhost.Client{
		{Name: "aws-public", API: &decliningSupportsClient{reason: reason}},
	}
	e := New(clients, nil)

	resources := []ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

	results, err := e.GetActualCost(context.Background(), resources, from, to)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, adapterNone, results[0].Adapter)
	assert.Contains(t, results[0].Notes, "No actual cost data available")
	assert.Contains(t, results[0].Notes, "aws-public: "+reason)
}

// TestGetActualCostWithOptionsAndErrors_DeclineReasonsSurfaced applies the
// same surfacing to the fallback-estimate placeholder path.
func TestGetActualCostWithOptionsAndErrors_DeclineReasonsSurfaced(t *testing.T) {
	t.Parallel()

	const reason = "region not served by this binary"

	clients := []*pluginhost.Client{
		{Name: "aws-public", API: &decliningSupportsClient{reason: reason}},
	}
	e := New(clients, nil)

	request := ActualCostRequest{
		Resources: []ResourceDescriptor{
			{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
		},
		From:             time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:               time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		FallbackEstimate: true,
	}

	result, err := e.GetActualCostWithOptionsAndErrors(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Results, 1)
	assert.Contains(t, result.Results[0].Notes, "aws-public: "+reason)
}

func TestDeclineNotes(t *testing.T) {
	t.Parallel()

	longReason := strings.Repeat("x", maxDeclineReasonLen+10)

	tests := []struct {
		name     string
		declines []pluginDecline
		want     []string
		notWant  []string
	}{
		{
			name:     "no declines returns base unchanged",
			declines: nil,
			want:     []string{"No pricing information available"},
			notWant:  []string{"declined by"},
		},
		{
			name: "single decline with reason",
			declines: []pluginDecline{
				{plugin: "aws-public", reason: "region not served"},
			},
			want: []string{"(declined by aws-public: region not served)"},
		},
		{
			name: "decline without reason names only the plugin",
			declines: []pluginDecline{
				{plugin: "aws-public"},
			},
			want:    []string{"(declined by aws-public)"},
			notWant: []string{"aws-public:"},
		},
		{
			name: "count is capped with overflow hint",
			declines: []pluginDecline{
				{plugin: "p1", reason: "r1"},
				{plugin: "p2", reason: "r2"},
				{plugin: "p3", reason: "r3"},
				{plugin: "p4", reason: "r4"},
				{plugin: "p5", reason: "r5"},
			},
			want:    []string{"p3: r3", "and 2 more"},
			notWant: []string{"p4", "p5"},
		},
		{
			name: "long reasons are truncated",
			declines: []pluginDecline{
				{plugin: "aws-public", reason: longReason},
			},
			want:    []string{strings.Repeat("x", maxDeclineReasonLen) + "..."},
			notWant: []string{longReason},
		},
		{
			name: "multi-byte rune that starts at the cap is kept whole and dropped",
			declines: []pluginDecline{
				{plugin: "aws-public", reason: strings.Repeat("x", maxDeclineReasonLen) + "日extra"},
			},
			want:    []string{strings.Repeat("x", maxDeclineReasonLen) + "..."},
			notWant: []string{"日", "extra"},
		},
		{
			name: "multi-byte rune straddling the cap is not split",
			declines: []pluginDecline{
				{plugin: "aws-public", reason: strings.Repeat("y", maxDeclineReasonLen-1) + "日extra"},
			},
			want:    []string{strings.Repeat("y", maxDeclineReasonLen-1) + "..."},
			notWant: []string{"日", "extra"},
		},
		{
			name: "multi-byte rune that begins inside the cap is dropped whole",
			declines: []pluginDecline{
				{plugin: "aws-public", reason: strings.Repeat("z", maxDeclineReasonLen-2) + "日extra"},
			},
			want:    []string{strings.Repeat("z", maxDeclineReasonLen-2) + "..."},
			notWant: []string{"日", "extra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := declineNotes(noteNoPricingInfo, tt.declines)
			assert.True(t, utf8.ValidString(got))
			for _, want := range tt.want {
				assert.Contains(t, got, want)
			}
			for _, notWant := range tt.notWant {
				assert.NotContains(t, got, notWant)
			}
		})
	}
}

// attrsSupportsClient records every Supports descriptor and answers yes.
type attrsSupportsClient struct {
	mockCostSourceClient

	seen []*pbc.ResourceDescriptor
}

func (c *attrsSupportsClient) Supports(
	_ context.Context, req *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	c.seen = append(c.seen, req.GetResource())
	return &pbc.SupportsResponse{Supported: true}, nil
}

func deployment(replicas float64) ResourceDescriptor {
	return ResourceDescriptor{
		Type: "kubernetes:apps/v1:Deployment", Provider: "kubernetes", ID: "urn:deploy",
		Properties: map[string]any{"spec": map[string]any{"replicas": replicas}},
	}
}

func TestCheckPluginSupports_SendsAttributes(t *testing.T) {
	t.Parallel()

	api := &attrsSupportsClient{}
	e, client := supportsEngine(api)

	assert.True(t, supportsOnly(e, client, deployment(3)))

	require.Len(t, api.seen, 1)
	value, ok := pluginsdk.AttributeValue(api.seen[0].GetAttributes(), "spec.replicas")
	require.True(t, ok)
	assert.InDelta(t, 3.0, value.GetNumberValue(), 1e-9)
}

func TestCheckPluginSupports_CacheKeyIncludesAttributes(t *testing.T) {
	t.Parallel()

	t.Run("different attributes ask again", func(t *testing.T) {
		t.Parallel()
		api := &attrsSupportsClient{}
		e, client := supportsEngine(api)

		supportsOnly(e, client, deployment(3))
		supportsOnly(e, client, deployment(5))

		assert.Len(t, api.seen, 2)
	})

	t.Run("identical attributes share one answer", func(t *testing.T) {
		t.Parallel()
		api := &attrsSupportsClient{}
		e, client := supportsEngine(api)

		supportsOnly(e, client, deployment(3))
		supportsOnly(e, client, deployment(3))

		assert.Len(t, api.seen, 1)
	})

	t.Run("no attributes keeps the old key", func(t *testing.T) {
		t.Parallel()
		api := &attrsSupportsClient{}
		e, client := supportsEngine(api)

		supportsOnly(e, client, ResourceDescriptor{
			Type: "aws:s3/bucket:Bucket", Provider: "aws", ID: "urn:bucket",
		})

		require.Len(t, api.seen, 1)
		assert.Nil(t, api.seen[0].GetAttributes())
		e.supportsMu.RLock()
		defer e.supportsMu.RUnlock()
		assert.Contains(t, e.supportsCache, "aws-public:aws:aws:s3/bucket:Bucket:::ProjectedCosts")
		assert.Len(t, e.supportsCache, 1)
	})
}
