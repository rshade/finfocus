// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const (
	jevDeclineReason        = "jev plugin scores recommendations only"
	kubernetesDeclineReason = "kubernetes plugin provides usage and allocation only"
)

// callCountClient records cost RPCs. A scorer-only plugin must stay at zero
// for Supports, GetProjectedCost, GetActualCost, and BatchCost.
type callCountClient struct {
	mockCostSourceClient

	mu        sync.Mutex
	supports  int
	projected int
	actual    int
	batch     int
	supported bool
	reason    string
}

func newCallClient(supported bool, reason string) *callCountClient {
	return &callCountClient{supported: supported, reason: reason}
}

func (c *callCountClient) Supports(
	_ context.Context, _ *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	c.mu.Lock()
	c.supports++
	c.mu.Unlock()
	return &pbc.SupportsResponse{Supported: c.supported, Reason: c.reason}, nil
}

func (c *callCountClient) GetProjectedCost(
	_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	c.mu.Lock()
	c.projected++
	c.mu.Unlock()
	return &proto.GetProjectedCostResponse{}, nil
}

func (c *callCountClient) GetActualCost(
	_ context.Context, _ *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	c.mu.Lock()
	c.actual++
	c.mu.Unlock()
	return &proto.GetActualCostResponse{}, nil
}

func (c *callCountClient) BatchCost(
	_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption,
) (*pbc.BatchCostResponse, error) {
	c.mu.Lock()
	c.batch++
	c.mu.Unlock()
	return c.mockCostSourceClient.BatchCost(context.Background(), nil)
}

func (c *callCountClient) counts() (int, int, int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.supports, c.projected, c.actual, c.batch
}

func assertNoCostRPCs(t *testing.T, c *callCountClient) {
	t.Helper()
	supports, projected, actual, batch := c.counts()
	assert.Zero(t, supports, "Supports")
	assert.Zero(t, projected, "GetProjectedCost")
	assert.Zero(t, actual, "GetActualCost")
	assert.Zero(t, batch, "BatchCost")
}

func clientWithMeta(name string, meta *proto.PluginMetadata, api *callCountClient) *pluginhost.Client {
	return &pluginhost.Client{Name: name, Metadata: meta, API: api}
}

func scorerMeta(extra ...string) *proto.PluginMetadata {
	caps := append([]string{pluginhost.CapabilityRecommendationScoring}, extra...)
	return &proto.PluginMetadata{Capabilities: caps}
}

func pricedResource() ResourceDescriptor {
	return ResourceDescriptor{
		Type:     "aws:ec2/instance:Instance",
		ID:       "i-001",
		Provider: "aws",
		Properties: map[string]interface{}{
			"instanceType":     "m5.large",
			"availabilityZone": "us-east-1a",
		},
	}
}

func matchNames(matches []PluginMatch) []string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.Client != nil {
			names = append(names, match.Client.Name)
		}
	}
	return names
}

func TestScorerOnly_ProjectedNoteOmitsDecliningScorer(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(false, jevDeclineReason)
	kubeAPI := newCallClient(false, kubernetesDeclineReason)
	jev := clientWithMeta("jev", scorerMeta(), jevAPI)
	kube := clientWithMeta("kubernetes", nil, kubeAPI)
	e := New([]*pluginhost.Client{jev, kube}, nil)

	results, err := e.GetProjectedCost(context.Background(), []ResourceDescriptor{pricedResource()})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Contains(t, results[0].Notes, "kubernetes: "+kubernetesDeclineReason)
	assert.NotContains(t, results[0].Notes, "jev")
	require.NotNil(t, results[0].Error)
	assert.NotContains(t, results[0].Error.Message, "jev")
	assertNoCostRPCs(t, jevAPI)
	kubeSupports, _, _, _ := kubeAPI.counts()
	assert.Positive(t, kubeSupports)
}

func TestScorerOnly_SolePluginHasNoDeclineNote(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(false, jevDeclineReason)
	e := New([]*pluginhost.Client{clientWithMeta("jev", scorerMeta(), jevAPI)}, nil)

	results, err := e.GetProjectedCost(context.Background(), []ResourceDescriptor{pricedResource()})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, noteNoPricingInfo, results[0].Notes)
	require.NotNil(t, results[0].Error)
	assert.Equal(t, noteNoPricingInfo, results[0].Error.Message)
	assertNoCostRPCs(t, jevAPI)
}

func TestScorerOnly_AcceptingScorerIsNotQueried(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(true, "")
	meta := scorerMeta(batchCostCapability)
	e := New([]*pluginhost.Client{clientWithMeta("jev", meta, jevAPI)}, nil)

	results, err := e.GetProjectedCost(context.Background(), []ResourceDescriptor{pricedResource()})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, noteNoPricingInfo, results[0].Notes)
	assertNoCostRPCs(t, jevAPI)
}

func TestScorerOnly_RouterFallbackSkipsScorer(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(false, jevDeclineReason)
	kubeAPI := newCallClient(false, kubernetesDeclineReason)
	jev := clientWithMeta("jev", scorerMeta(), jevAPI)
	kube := clientWithMeta("kubernetes", nil, kubeAPI)
	router := &mockRouter{
		selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
			return nil
		},
	}
	e := New([]*pluginhost.Client{jev, kube}, nil).WithRouter(router)

	results, err := e.GetProjectedCost(context.Background(), []ResourceDescriptor{pricedResource()})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Notes, "kubernetes: "+kubernetesDeclineReason)
	assert.NotContains(t, results[0].Notes, "jev")
	assertNoCostRPCs(t, jevAPI)
	supports, _, _, _ := kubeAPI.counts()
	assert.Positive(t, supports)
}

func TestScorerOnly_ExplicitRouteDoesNotFallBack(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(true, "")
	otherAPI := newCallClient(true, "")
	jev := clientWithMeta("jev", scorerMeta(), jevAPI)
	other := clientWithMeta("aws", nil, otherAPI)
	router := &mockRouter{
		selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
			return []PluginMatch{{Client: jev, Priority: 10, Fallback: false}}
		},
	}
	e := New([]*pluginhost.Client{jev, other}, nil).WithRouter(router)

	matches, declines := e.selectPluginMatchesForResource(
		context.Background(), pricedResource(), "ProjectedCosts",
	)
	require.NotNil(t, matches)
	assert.Empty(t, matches)
	assert.Empty(t, declines)
	assertNoCostRPCs(t, jevAPI)
	assertNoCostRPCs(t, otherAPI)
}

func TestScorerOnly_MixedRouteKeepsPricer(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(true, "")
	awsAPI := newCallClient(true, "")
	jev := clientWithMeta("jev", scorerMeta(), jevAPI)
	aws := clientWithMeta("aws", nil, awsAPI)
	router := &mockRouter{
		selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
			return []PluginMatch{
				{Client: jev, Priority: 1, Fallback: true},
				{Client: aws, Priority: 2, Fallback: true},
			}
		},
	}
	e := New([]*pluginhost.Client{jev, aws}, nil).WithRouter(router)

	matches, _ := e.selectPluginMatchesForResource(
		context.Background(), pricedResource(), "ProjectedCosts",
	)
	assert.Equal(t, []string{"aws"}, matchNames(matches))
	assertNoCostRPCs(t, jevAPI)
	supports, _, _, _ := awsAPI.counts()
	assert.Positive(t, supports)
}

func TestScorerOnly_DualCapabilityStaysOffered(t *testing.T) {
	t.Parallel()

	api := newCallClient(true, "")
	meta := &proto.PluginMetadata{Capabilities: []string{
		pluginhost.CapabilityRecommendationScoring,
		capabilityRecommendations,
	}}
	client := clientWithMeta("hybrid", meta, api)
	e := New([]*pluginhost.Client{client}, nil)

	matches, _ := e.selectPluginMatchesForResource(
		context.Background(), pricedResource(), "ProjectedCosts",
	)
	assert.Equal(t, []string{"hybrid"}, matchNames(matches))
	supports, _, _, _ := api.counts()
	assert.Positive(t, supports)
}

func TestScorerOnly_NoCapabilityStaysOffered(t *testing.T) {
	t.Parallel()

	nilMeta := newCallClient(true, "")
	emptyCaps := newCallClient(true, "")
	clients := []*pluginhost.Client{
		clientWithMeta("legacy", nil, nilMeta),
		clientWithMeta("empty", &proto.PluginMetadata{Capabilities: []string{}}, emptyCaps),
	}
	e := New(clients, nil)

	matches, _ := e.selectPluginMatchesForResource(
		context.Background(), pricedResource(), "ProjectedCosts",
	)
	assert.Equal(t, []string{"legacy", "empty"}, matchNames(matches))
	nilSupports, _, _, _ := nilMeta.counts()
	emptySupports, _, _, _ := emptyCaps.counts()
	assert.Positive(t, nilSupports)
	assert.Positive(t, emptySupports)
}

func TestScorerOnly_RecommendationsSelectionUnchanged(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(true, "")
	sourceAPI := newCallClient(true, "")
	jev := clientWithMeta("jev", scorerMeta(), jevAPI)
	source := clientWithMeta("aws", &proto.PluginMetadata{
		Capabilities: []string{capabilityRecommendations},
	}, sourceAPI)
	e := New([]*pluginhost.Client{source, jev}, nil)

	matches, _ := e.selectPluginMatchesForResource(
		context.Background(), pricedResource(), recommendationsFeature,
	)
	assert.ElementsMatch(t, []string{"aws", "jev"}, matchNames(matches))

	targets := e.routeRecommendationTargets(context.Background(), []ResourceDescriptor{pricedResource()})
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.client.Name)
	}
	assert.Equal(t, []string{"aws"}, names)
}

func TestScorerOnly_ActualAndBatchOmitScorer(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(true, "")
	jev := clientWithMeta("jev", scorerMeta(batchCostCapability), jevAPI)
	e := New([]*pluginhost.Client{jev}, nil)
	resource := pricedResource()

	for _, feature := range []string{"ActualCosts", batchCostFeature} {
		matches, declines := e.selectPluginMatchesForResource(context.Background(), resource, feature)
		require.NotNil(t, matches, feature)
		assert.Empty(t, matches, feature)
		assert.Empty(t, declines, feature)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	results, err := e.GetActualCostWithOptions(context.Background(), ActualCostRequest{
		Resources:        []ResourceDescriptor{resource},
		From:             from,
		To:               to,
		FallbackEstimate: true,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, noteNoActualCostData, results[0].Notes)
	assert.NotContains(t, results[0].Notes, "jev")
	assertNoCostRPCs(t, jevAPI)

	groups := e.groupResourcesByPlugin(context.Background(), []ResourceDescriptor{resource})
	assert.Empty(t, groups)
	assertNoCostRPCs(t, jevAPI)
}

func TestScorerOnly_StateEstimateDoesNotQueryScorer(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(true, "")
	jev := clientWithMeta("jev", scorerMeta(), jevAPI)
	e := New([]*pluginhost.Client{jev}, nil)

	resource := pricedResource()
	resource.Properties[PropertyPulumiCreated] = "2026-01-01T00:00:00Z"
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	_, err := e.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources:        []ResourceDescriptor{resource},
		From:             from,
		To:               to,
		FallbackEstimate: true,
	})
	require.NoError(t, err)
	assertNoCostRPCs(t, jevAPI)
}

func TestScorerOnly_InternalPulumiTypeStaysFiltered(t *testing.T) {
	t.Parallel()

	jevAPI := newCallClient(true, "")
	e := New([]*pluginhost.Client{clientWithMeta("jev", scorerMeta(), jevAPI)}, nil)
	internal := ResourceDescriptor{Type: "pulumi:pulumi:Stack", ID: "stack", Provider: "pulumi"}

	matches, declines := e.selectPluginMatchesForResource(context.Background(), internal, "ProjectedCosts")
	assert.Nil(t, matches)
	assert.Nil(t, declines)
	assertNoCostRPCs(t, jevAPI)
}
