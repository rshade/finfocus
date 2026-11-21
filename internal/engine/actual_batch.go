// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// actualBatchVerdict is what a BatchCost call answered for one resource. The
// named plugin was already asked: either result is set, or err says why it has
// no price. A resource with no verdict is priced one by one.
type actualBatchVerdict struct {
	plugin string
	result *CostResult
	err    error
}

// answers reports whether the verdict stands in for a call to the plugin.
func (v actualBatchVerdict) answers(pluginName string) bool {
	return v.plugin != "" && v.plugin == pluginName
}

// outcome is the verdict as a single-resource call would have returned it. A
// pre-flight validation placeholder is a failure, as the unary call fails.
func (v actualBatchVerdict) outcome() (*CostResult, error) {
	if v.result != nil && v.result.Error != nil {
		return nil, fmt.Errorf("%s: %s", v.result.Error.Code, v.result.Error.Message)
	}
	return v.result, v.err
}

// batchActualCosts prices, with BatchCost, every resource whose primary plugin
// declares batch_cost, and returns a verdict per resource index.
//
// A batch carries one start, so a plugin's resources are sent in one batch per
// start (actualBatchStart). A resource gets no verdict, and so goes to the
// per-resource path, when its plugin has no batch capability, it fails the
// adapter or tag filter, it has no time in the window, its descriptor exceeds
// the SDK limits, the plugin marked its type unsupported or answered it
// Unavailable, or the batch call for it failed. A row with no cost data gets
// ErrNoCostData, the error a unary call returns. A row still rate limited after
// the batch retries gets errBatchRateLimited, as does every later batch for
// that plugin, so the plugin is not asked again.
func (e *Engine) batchActualCosts(ctx context.Context, request ActualCostRequest) map[int]actualBatchVerdict {
	verdicts := make(map[int]actualBatchVerdict)
	if !e.hasBatchCapableClient() {
		return verdicts
	}
	for _, pb := range e.actualBatchPlugins(ctx, request) {
		var limitErr error
		for _, group := range actualBatchGroups(pb.resources, request) {
			if ctx.Err() != nil {
				return verdicts
			}
			if limitErr != nil {
				for _, ir := range group.resources {
					verdicts[ir.index] = actualBatchVerdict{plugin: pb.plugin.Name, err: limitErr}
				}
				continue
			}
			limitErr = e.priceActualBatch(ctx, pb.plugin, group, request, verdicts)
		}
	}
	return verdicts
}

// actualBatchPlugins groups resources by the plugin the per-resource path
// would ask first: the first ActualCosts route match, or the first one named
// by the adapter filter. Only plugins with batch_cost are kept, so a verdict
// always stands in for the first call of that resource's chain.
func (e *Engine) actualBatchPlugins(ctx context.Context, request ActualCostRequest) []*pluginBatch {
	adapterAllows := e.adapterFilter(request.Adapter)
	var groups []*pluginBatch
	for i, resource := range request.Resources {
		matches, _ := e.selectPluginMatchesForResource(ctx, resource, actualCostsFeature)
		j := slices.IndexFunc(matches, func(m PluginMatch) bool {
			return adapterAllows(m.Client)
		})
		if j < 0 || !matches[j].Client.HasCapability(batchCostCapability) {
			continue
		}
		client := matches[j].Client
		k := slices.IndexFunc(groups, func(g *pluginBatch) bool { return g.plugin.Name == client.Name })
		if k < 0 {
			groups = append(groups, &pluginBatch{plugin: client, hasBatch: true})
			k = len(groups) - 1
		}
		groups[k].resources = append(groups[k].resources, indexedResource{index: i, resource: resource})
	}
	return groups
}

// actualBatch is the resources of one plugin that share a batch start.
type actualBatch struct {
	start     time.Time
	resources []indexedResource
}

// actualBatchGroups splits a plugin's resources by batch start, earliest
// first. Resources that fail the tag filter or have no time in the window are
// left out.
func actualBatchGroups(resources []indexedResource, request ActualCostRequest) []actualBatch {
	var groups []actualBatch
	for _, ir := range resources {
		if len(request.Tags) > 0 && !MatchesTags(ir.resource, request.Tags) {
			continue
		}
		start, ok := actualBatchStart(ir.resource, request.From, request.To)
		if !ok {
			continue
		}
		i := slices.IndexFunc(groups, func(g actualBatch) bool { return g.start.Equal(start) })
		if i < 0 {
			groups = append(groups, actualBatch{start: start})
			i = len(groups) - 1
		}
		groups[i].resources = append(groups[i].resources, ir)
	}
	slices.SortStableFunc(groups, func(a, b actualBatch) int { return a.start.Compare(b.start) })
	return groups
}

// actualBatchStart is the start a resource is batched with: the window start,
// or for a resource created inside the window its creation time truncated to
// the hour. A stack's resources are created seconds apart, and a unary call
// would send each exact time; the hour keeps them in one batch at the cost of
// at most an hour of window for a plugin that prices by elapsed time. ok is
// false for a resource with no time in the window, which gets a local $0 on
// the per-resource path.
func actualBatchStart(resource ResourceDescriptor, from, to time.Time) (time.Time, bool) {
	billedFrom, existed := resourceActualStart(resource, from, to)
	if !existed {
		return time.Time{}, false
	}
	if billedFrom.Equal(from) {
		return from, true
	}
	start := billedFrom.Truncate(time.Hour)
	if start.Before(from) {
		start = from
	}
	return start, true
}

// priceActualBatch sends one start group to a plugin and records a verdict for
// each resource it answered. It returns the rate-limit error when the plugin
// still limited the batch after its retries, and nil otherwise.
func (e *Engine) priceActualBatch(
	ctx context.Context,
	plugin *pluginhost.Client,
	group actualBatch,
	request ActualCostRequest,
	verdicts map[int]actualBatchVerdict,
) error {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "batch_actual_cost").
		Str("plugin", plugin.Name).
		Time("start", group.start).
		Int("resource_count", len(group.resources)).
		Msg("executing batch actual cost")

	batchResults, batchErr := e.executeBatchForPlugin(ctx, plugin, group.resources, batchOptions{
		queryType: pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL,
		start:     timestamppb.New(group.start),
		end:       timestamppb.New(request.To),
	})
	if batchErr != nil {
		if ctx.Err() == nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "engine").
				Str("plugin", plugin.Name).
				Err(batchErr).
				Int("resource_count", len(group.resources)).
				Msg("batch actual cost failed, falling back to per-resource queries")
		}
		return nil
	}

	var limitErr error
	for _, br := range batchResults {
		billedFrom, _ := resourceActualStart(request.Resources[br.index], request.From, request.To)
		keepRequestedWindow(br.actualResult, request.From, billedFrom, request.To)
		verdict, ok := actualVerdictFor(ctx, plugin.Name, br, request)
		if !ok {
			continue
		}
		verdicts[br.index] = verdict
		if br.rateLimited {
			limitErr = verdict.err
		}
	}
	return limitErr
}

func actualVerdictFor(
	ctx context.Context,
	pluginName string,
	br batchResult,
	request ActualCostRequest,
) (actualBatchVerdict, bool) {
	log := logging.FromContext(ctx)
	resourceType := request.Resources[br.index].Type
	switch {
	case br.rateLimited:
		if errors.Is(br.err, errBatchRateLimited) {
			return actualBatchVerdict{plugin: pluginName, err: br.err}, true
		}
		return actualBatchVerdict{plugin: pluginName, err: batchRateLimitError(br.err)}, true
	case br.unanswered:
		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("plugin", pluginName).
			Str("resource_type", resourceType).
			Err(br.err).
			Msg("batch did not answer resource, trying per-resource query")
		return actualBatchVerdict{}, false
	case br.skip:
		log.Warn().
			Ctx(ctx).
			Str("component", "engine").
			Str("plugin", pluginName).
			Str("resource_type", resourceType).
			Msg("resource type unsupported in batch actual cost, trying per-resource query")
		return actualBatchVerdict{}, false
	case status.Code(br.err) == codes.Unavailable:
		log.Warn().
			Ctx(ctx).
			Str("component", "engine").
			Str("plugin", pluginName).
			Str("resource_type", resourceType).
			Err(br.err).
			Msg("resource unavailable in batch actual cost, trying per-resource query")
		return actualBatchVerdict{}, false
	case br.err != nil:
		log.Warn().
			Ctx(ctx).
			Str("component", "engine").
			Str("plugin", pluginName).
			Str("resource_type", resourceType).
			Err(br.err).
			Msg("per-resource error in batch actual cost result")
		return actualBatchVerdict{plugin: pluginName, err: pluginStatusError(br.err)}, true
	case br.actualResult != nil:
		return actualBatchVerdict{plugin: pluginName, result: br.actualResult}, true
	default:
		return actualBatchVerdict{plugin: pluginName, err: ErrNoCostData}, true
	}
}
