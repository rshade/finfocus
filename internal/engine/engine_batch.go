// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// pluginBatch groups resources targeted at a single plugin for batch processing.
type pluginBatch struct {
	// plugin is the target plugin client.
	plugin *pluginhost.Client
	// resources are the resources with their original indices.
	resources []indexedResource
	// hasBatch indicates whether the plugin supports the batch_cost capability.
	hasBatch bool
}

// indexedResource preserves the original index for result reassembly after batch processing.
type indexedResource struct {
	// index is the original position in the input resource slice.
	index int
	// resource is the resource descriptor.
	resource ResourceDescriptor
}

// batchResult holds the result of a batch cost call for a single resource.
type batchResult struct {
	// index is the original position in the input resource slice.
	index int
	// result holds the projected cost result (nil if error or actual query).
	result *CostResult
	// actualResult holds the actual cost result (nil if error or projected query).
	actualResult *CostResult
	// err holds a per-resource error (nil on success).
	err error
	// skip is true when the resource type is unsupported.
	skip bool
}

// batchOptions holds configuration for a batch cost request.
type batchOptions struct {
	// queryType is the cost query type (projected or actual).
	queryType pbc.CostQueryType
	// start is the start timestamp for actual cost queries (nil for projected).
	start *timestamppb.Timestamp
	// end is the end timestamp for actual cost queries (nil for projected).
	end *timestamppb.Timestamp
}

// hasBatchCapableClient returns true if any registered plugin client has the batch_cost capability.
// Used as a fast pre-check to avoid unnecessary grouping/routing when no plugin supports batch.
func (e *Engine) hasBatchCapableClient() bool {
	for _, c := range e.clients {
		if c.HasCapability(batchCostCapability) {
			return true
		}
	}
	return false
}

// chunkResources splits resources into chunks of at most chunkSize elements.
// If chunkSize <= 0, it defaults to batchProcessingThreshold (100).
// The order of resources is preserved.
func chunkResources(resources []indexedResource, chunkSize int) [][]indexedResource {
	if chunkSize <= 0 {
		chunkSize = batchProcessingThreshold
	}
	if len(resources) == 0 {
		return nil
	}

	numChunks := (len(resources) + chunkSize - 1) / chunkSize
	chunks := make([][]indexedResource, 0, numChunks)

	for i := 0; i < len(resources); i += chunkSize {
		end := i + chunkSize
		if end > len(resources) {
			end = len(resources)
		}
		chunks = append(chunks, resources[i:end])
	}
	return chunks
}

const (
	// batchCostFeature is the feature name used for batch cost plugin matching.
	batchCostFeature = "BatchCost"
	// batchCostCapability is the capability string reported by batch-capable plugins.
	batchCostCapability = "batch_cost"
)

// groupResourcesByPlugin groups resources by their primary plugin match for batch cost.
// Resources matching no plugin are excluded (same behavior as current per-resource path for
// internal Pulumi types). Each plugin group records whether it supports batch.
func (e *Engine) groupResourcesByPlugin(
	ctx context.Context,
	resources []ResourceDescriptor,
) map[string]*pluginBatch {
	groups := make(map[string]*pluginBatch)
	log := logging.FromContext(ctx)

	for i, resource := range resources {
		// Skip internal Pulumi types (same filter as per-resource path)
		if strings.HasPrefix(resource.Type, pulumiInternalPrefix) {
			continue
		}

		matches := e.selectPluginMatchesForResource(ctx, resource, batchCostFeature)
		if len(matches) == 0 {
			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("operation", "group_resources_by_plugin").
				Str("resource_type", resource.Type).
				Msg("no plugin match for resource, skipping batch grouping")
			continue
		}

		// Use the highest-priority match (first in the sorted list)
		primary := matches[0]
		name := primary.Client.Name

		if _, ok := groups[name]; !ok {
			groups[name] = &pluginBatch{
				plugin:   primary.Client,
				hasBatch: primary.Client.HasCapability(batchCostCapability),
			}
		}

		groups[name].resources = append(groups[name].resources, indexedResource{
			index:    i,
			resource: resource,
		})
	}
	return groups
}

// executeBatchForPlugin sends batch cost requests to a single plugin, chunking by
// max_batch_size. It maps results back to batchResult entries preserving original indices.
// On batch-level gRPC errors, it returns the error for the caller to handle fallback.
//
//nolint:gocognit // Batch processing with chunking, re-chunking, and result mapping requires this complexity.
func (e *Engine) executeBatchForPlugin(
	ctx context.Context,
	plugin *pluginhost.Client,
	resources []indexedResource,
	opts batchOptions,
) ([]batchResult, error) {
	log := logging.FromContext(ctx)
	chunkSize := batchProcessingThreshold
	allResults := make([]batchResult, 0, len(resources))

	chunks := chunkResources(resources, chunkSize)
	for chunkIdx, chunk := range chunks {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("batch cost cancelled for plugin %s: %w", plugin.Name, ctx.Err())
		}

		req := buildBatchCostRequest(ctx, chunk, opts)

		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "execute_batch").
			Str("plugin", plugin.Name).
			Int("chunk_index", chunkIdx).
			Int("chunk_size", len(chunk)).
			Int("total_chunks", len(chunks)).
			Msg("sending batch cost request")

		resp, err := plugin.API.BatchCost(ctx, req)
		if err != nil {
			// On DeadlineExceeded: if the parent context is still valid, the error is
			// batch-specific and the caller can fall back to per-resource queries.
			// If the parent context is done, propagate the cancellation directly so
			// the caller does not waste time on a fallback that will also fail.
			if ctx.Err() != nil {
				return nil, fmt.Errorf("batch cost cancelled for plugin %s: %w", plugin.Name, ctx.Err())
			}
			return nil, fmt.Errorf("batch cost RPC failed for plugin %s: %w", plugin.Name, err)
		}

		// Validate response count matches request
		if len(resp.GetResults()) != len(chunk) {
			return nil, fmt.Errorf(
				"batch response count mismatch from plugin %s: got %d, expected %d",
				plugin.Name, len(resp.GetResults()), len(chunk),
			)
		}

		if resp.GetMaxBatchSize() > 0 && int(resp.GetMaxBatchSize()) < chunkSize {
			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("plugin", plugin.Name).
				Int32("max_batch_size", resp.GetMaxBatchSize()).
				Int("previous_chunk_size", chunkSize).
				Msg("adjusting chunk size based on plugin hint")
			chunkSize = int(resp.GetMaxBatchSize())
			// Re-chunk remaining resources with the new size
			remaining := make([]indexedResource, 0)
			for _, futureChunk := range chunks[chunkIdx+1:] {
				remaining = append(remaining, futureChunk...)
			}
			if len(remaining) > 0 {
				chunks = append(chunks[:chunkIdx+1], chunkResources(remaining, chunkSize)...)
			}
		}

		// Map results based on query type
		var mapped []proto.BatchMappedResult
		if opts.queryType == pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL {
			mapped = proto.MapBatchActualResults(resp)
		} else {
			mapped = proto.MapBatchProjectedResults(resp)
		}

		for i, m := range mapped {
			br := batchResult{
				index: chunk[i].index,
				skip:  m.Skip,
				err:   m.Err,
			}
			if m.Result != nil {
				br.result = mapProtoCostResultToEngine(chunk[i].resource, plugin.Name, m.Result)
			}
			if m.ActualResult != nil {
				br.actualResult = mapProtoActualCostResultToEngine(chunk[i].resource, plugin.Name, m.ActualResult)
			}
			allResults = append(allResults, br)
		}
	}
	return allResults, nil
}

// buildBatchCostRequest constructs a proto BatchCostRequest from indexed resources.
func buildBatchCostRequest(ctx context.Context, resources []indexedResource, opts batchOptions) *pbc.BatchCostRequest {
	protoResources := make([]*pbc.ResourceDescriptor, len(resources))
	for i, ir := range resources {
		props := ConvertToProto(ir.resource.Properties)
		sku, region := proto.ResolveSKUAndRegion(
			ctx,
			ir.resource.Provider,
			ir.resource.Type,
			props,
		)
		protoResources[i] = &pbc.ResourceDescriptor{
			Id:           ir.resource.ID,
			Provider:     ir.resource.Provider,
			ResourceType: ir.resource.Type,
			Sku:          sku,
			Region:       region,
			Tags:         props,
		}
	}

	return &pbc.BatchCostRequest{
		Resources: protoResources,
		QueryType: opts.queryType,
		Start:     opts.start,
		End:       opts.end,
		DryRun:    false,
	}
}

// mapProtoCostResultToEngine converts a proto CostResult to an engine CostResult.
func mapProtoCostResultToEngine(
	resource ResourceDescriptor,
	pluginName string,
	result *proto.CostResult,
) *CostResult {
	engineResult := &CostResult{
		ResourceType:   resource.Type,
		ResourceID:     resource.ID,
		Adapter:        pluginName,
		Currency:       result.Currency,
		Monthly:        result.MonthlyCost,
		Hourly:         result.HourlyCost,
		Notes:          result.Notes,
		Breakdown:      result.CostBreakdown,
		Sustainability: make(map[string]SustainabilityMetric),
	}

	engineResult.ExpiresAt = result.ExpiresAt

	if result.StructuredError != nil {
		engineResult.Error = &StructuredError{
			Code:         result.StructuredError.Code,
			Message:      result.StructuredError.Message,
			ResourceType: result.StructuredError.ResourceType,
		}
	}

	for k, v := range result.Sustainability {
		engineResult.Sustainability[k] = SustainabilityMetric{
			Value: v.Value,
			Unit:  v.Unit,
		}
	}
	return engineResult
}

// mapProtoActualCostResultToEngine converts a proto ActualCostResult to an engine CostResult
// for the actual cost path.
func mapProtoActualCostResultToEngine(
	resource ResourceDescriptor,
	pluginName string,
	result *proto.ActualCostResult,
) *CostResult {
	engineResult := &CostResult{
		ResourceType:   resource.Type,
		ResourceID:     resource.ID,
		Adapter:        pluginName,
		Currency:       result.Currency,
		TotalCost:      result.TotalCost,
		Breakdown:      result.CostBreakdown,
		Sustainability: make(map[string]SustainabilityMetric),
	}

	engineResult.ExpiresAt = result.ExpiresAt

	for k, v := range result.Sustainability {
		engineResult.Sustainability[k] = SustainabilityMetric{
			Value: v.Value,
			Unit:  v.Unit,
		}
	}
	return engineResult
}
