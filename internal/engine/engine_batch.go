// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
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
	// rateLimited is set when ResourceExhausted survived a batch retry.
	// The caller must not price that resource with a single-resource RPC
	// against the same plugin.
	rateLimited bool
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
		end := min(i+chunkSize, len(resources))
		chunks = append(chunks, resources[i:end])
	}
	return chunks
}

// maxBatchRequestBytes bounds the summed descriptor size of one BatchCost
// request. Core dials plugins with grpc-go's default 4 MiB message limit and no
// MaxCallSendMsgSize option; 100 resources with 64 KiB of attributes each would
// exceed it. The 1 MiB headroom covers the envelope.
const maxBatchRequestBytes = 3 << 20

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

		matches, _ := e.selectPluginMatchesForResource(ctx, resource, batchCostFeature)
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
//nolint:gocognit,funlen // Batch processing with chunking, re-chunking, and result mapping requires this complexity.
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
	// Index-controlled loop (not for-range): when a plugin hints a smaller MaxBatchSize,
	// the remaining resources are re-chunked and appended to chunks. A for-range header
	// captures the slice length at loop start, so appended tail chunks would never run.
	for chunkIdx := 0; chunkIdx < len(chunks); chunkIdx++ {
		chunk := chunks[chunkIdx]
		if ctx.Err() != nil {
			return nil, fmt.Errorf("batch cost cancelled for plugin %s: %w", plugin.Name, ctx.Err())
		}

		built := buildBatchCostRequest(ctx, chunk, opts)
		allResults = append(allResults, built.invalidResults...)

		if parts := splitBatchBySize(built, maxBatchRequestBytes); len(parts) > 1 {
			// Send the first part now and queue the rest as their own chunks, so a
			// later max_batch_size hint re-chunks them like any other chunk. Their
			// rebuild finds no invalid resources: only valid ones were split.
			split := make([][]indexedResource, len(parts))
			for i, part := range parts {
				split[i] = part.validResources
			}
			chunks = slices.Concat(chunks[:chunkIdx], split, chunks[chunkIdx+1:])
			built = parts[0]
			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("operation", "execute_batch").
				Str("plugin", plugin.Name).
				Int("chunk_index", chunkIdx).
				Int("parts", len(parts)).
				Msg("batch chunk split to stay under the request size budget")
		}

		if built.request == nil {
			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("operation", "execute_batch").
				Str("plugin", plugin.Name).
				Int("chunk_index", chunkIdx).
				Int("invalid_count", len(built.invalidResults)).
				Msg("all resources in chunk failed validation, skipping batch RPC")
			continue
		}

		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "execute_batch").
			Str("plugin", plugin.Name).
			Int("chunk_index", chunkIdx).
			Int("chunk_size", len(built.validResources)).
			Int("total_chunks", len(chunks)).
			Int("skipped_invalid", len(built.invalidResults)).
			Msg("sending batch cost request")

		resp, err := e.batchCostWithRateLimit(ctx, plugin, built.request, len(built.validResources))
		if err != nil {
			// On DeadlineExceeded: if the parent context is still valid, the error is
			// batch-specific and the caller can fall back to per-resource queries.
			// If the parent context is done, propagate the cancellation directly so
			// the caller does not waste time on a fallback that will also fail.
			// A rate limit that survives retries keeps prices already collected and
			// marks this chunk, plus every chunk not yet sent, so those resources
			// are not priced one by one on this plugin. Earlier chunks' ordinary
			// errors stay unmarked and can still use a single call.
			if ctx.Err() != nil {
				return allResults, fmt.Errorf("batch cost cancelled for plugin %s: %w", plugin.Name, ctx.Err())
			}
			if errors.Is(err, errBatchRateLimited) {
				log.Warn().
					Ctx(ctx).
					Str("component", "engine").
					Str("operation", "execute_batch").
					Str("plugin", plugin.Name).
					Err(err).
					Int("resource_count", len(built.validResources)).
					Msg("batch cost rate limited, not retrying resources individually")
				allResults = append(allResults, rateLimitedBatchResults(built.validResources, err)...)
				pending := slices.Concat(chunks[chunkIdx+1:]...)
				allResults = append(allResults, rateLimitedBatchResults(pending, err)...)
				return allResults, nil
			}
			return nil, fmt.Errorf("batch cost RPC failed for plugin %s: %w", plugin.Name, err)
		}

		// Validate response count matches request (only validated resources were sent)
		if len(resp.GetResults()) != len(built.validResources) {
			return nil, fmt.Errorf(
				"batch response count mismatch from plugin %s: got %d, expected %d",
				plugin.Name, len(resp.GetResults()), len(built.validResources),
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

		// Resolve the request time window for actual-cost rate derivation
		var from, to time.Time
		if opts.start != nil {
			from = opts.start.AsTime()
		}
		if opts.end != nil {
			to = opts.end.AsTime()
		}

		// Map results based on query type
		var mapped []proto.BatchMappedResult
		if opts.queryType == pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL {
			mapped = proto.MapBatchActualResults(resp)
		} else {
			mapped = proto.MapBatchProjectedResults(resp)
		}

		kept, again := splitMappedBatch(mapped, built.validResources, plugin.Name, from, to)
		allResults = append(allResults, kept...)
		if len(again) == 0 {
			continue
		}
		retried, retryErr := e.retryRateLimitedBatchItems(ctx, plugin, again, opts)
		if retryErr != nil {
			if ctx.Err() != nil {
				return allResults, fmt.Errorf("batch cost cancelled for plugin %s: %w", plugin.Name, ctx.Err())
			}
			// Keep rows already priced. Only the still-limited resources carry
			// this error, so the unary path may call this plugin for them.
			log.Warn().
				Ctx(ctx).
				Str("component", "engine").
				Str("operation", "retry_rate_limited_batch").
				Str("plugin", plugin.Name).
				Int("resource_count", len(again)).
				Err(retryErr).
				Msg("batch rate-limit retry failed, falling back to per-resource queries for those resources")
			for _, ir := range again {
				allResults = append(allResults, batchResult{index: ir.index, err: retryErr})
			}
			continue
		}
		allResults = append(allResults, retried...)
	}
	return allResults, nil
}

// batchCostWithRateLimit calls BatchCost and, on ResourceExhausted, waits and
// retries the same request. The wait uses the parent context. Each attempt
// gets a fresh chunk timeout.
func (e *Engine) batchCostWithRateLimit(
	ctx context.Context,
	plugin *pluginhost.Client,
	request *pbc.BatchCostRequest,
	resourceCount int,
) (*pbc.BatchCostResponse, error) {
	log := logging.FromContext(ctx)
	var last error
	for attempt := range rateLimitMaxAttempts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunkCtx, cancel := context.WithTimeout(ctx, batchChunkTimeout(ctx, resourceCount))
		resp, err := plugin.API.BatchCost(chunkCtx, request)
		cancel()
		if err == nil {
			return resp, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		last = err
		if !isRateLimited(err) || attempt == rateLimitMaxAttempts-1 {
			if isRateLimited(err) {
				return nil, errBatchRateLimited
			}
			return nil, err
		}
		delay := rateLimitDelay(err, attempt)
		log.Info().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "batch_rate_limit_retry").
			Str("plugin", plugin.Name).
			Int("attempt", attempt+1).
			Dur("delay", delay).
			Err(err).
			Msg("plugin rate limited, retrying the same batch")
		if waitErr := waitRateLimit(ctx, delay); waitErr != nil {
			return nil, waitErr
		}
	}
	return nil, last
}

// retryRateLimitedBatchItems sends the rate-limited resources again as one
// batch. Items that are still rate limited are marked so the caller does not
// fan them out into single-resource RPCs.
func (e *Engine) retryRateLimitedBatchItems(
	ctx context.Context,
	plugin *pluginhost.Client,
	resources []indexedResource,
	opts batchOptions,
) ([]batchResult, error) {
	if err := waitRateLimit(ctx, backoffDelay(0)); err != nil {
		return nil, err
	}
	built := buildBatchCostRequest(ctx, resources, opts)
	results := append([]batchResult{}, built.invalidResults...)
	if built.request == nil {
		return results, nil
	}
	resp, err := e.batchCostWithRateLimit(ctx, plugin, built.request, len(built.validResources))
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, errBatchRateLimited) {
			for _, ir := range built.validResources {
				results = append(results, batchResult{index: ir.index, rateLimited: true, err: err})
			}
			if ctx.Err() != nil {
				return results, ctx.Err()
			}
			return results, nil
		}
		return nil, err
	}
	if len(resp.GetResults()) != len(built.validResources) {
		return nil, fmt.Errorf(
			"batch response count mismatch from plugin %s: got %d, expected %d",
			plugin.Name, len(resp.GetResults()), len(built.validResources),
		)
	}
	var from, to time.Time
	if opts.start != nil {
		from = opts.start.AsTime()
	}
	if opts.end != nil {
		to = opts.end.AsTime()
	}
	mapped := mapBatchResponse(resp, opts)
	kept, again := splitMappedBatch(mapped, built.validResources, plugin.Name, from, to)
	results = append(results, kept...)
	for _, ir := range again {
		results = append(results, batchResult{index: ir.index, rateLimited: true, err: errBatchRateLimited})
	}
	return results, nil
}

// rateLimitedBatchResults marks resources that must not be sent to this plugin
// again as single-resource RPCs.
func rateLimitedBatchResults(resources []indexedResource, err error) []batchResult {
	if len(resources) == 0 {
		return nil
	}
	out := make([]batchResult, len(resources))
	for i, ir := range resources {
		out[i] = batchResult{index: ir.index, rateLimited: true, err: err}
	}
	return out
}

func mapBatchResponse(resp *pbc.BatchCostResponse, opts batchOptions) []proto.BatchMappedResult {
	if opts.queryType == pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL {
		return proto.MapBatchActualResults(resp)
	}
	return proto.MapBatchProjectedResults(resp)
}

// splitMappedBatch keeps successful, skipped, and ordinary errors, and returns
// the resources whose error is a rate limit so they can be retried together.
func splitMappedBatch(
	mapped []proto.BatchMappedResult,
	valid []indexedResource,
	pluginName string,
	from, to time.Time,
) ([]batchResult, []indexedResource) {
	kept := make([]batchResult, 0, len(mapped))
	var again []indexedResource
	for i, m := range mapped {
		ir := valid[i]
		if isRateLimited(m.Err) {
			again = append(again, ir)
			continue
		}
		br := batchResult{index: ir.index, skip: m.Skip, err: m.Err}
		if m.Result != nil {
			br.result = mapProtoCostResultToEngine(ir.resource, pluginName, m.Result)
		}
		if m.ActualResult != nil {
			br.actualResult = mapProtoActualCostResultToEngine(ir.resource, pluginName, m.ActualResult, from, to)
		}
		kept = append(kept, br)
	}
	return kept, again
}

// buildBatchResult holds the output of buildBatchCostRequest: a valid BatchCostRequest
// (possibly nil if all resources failed validation), the filtered valid resources for
// result-index mapping, and pre-validated failure placeholders for invalid resources.
type buildBatchResult struct {
	// request is the BatchCostRequest containing only validated resources (nil if all failed).
	request *pbc.BatchCostRequest
	// validResources are the resources included in the request, preserving the 1:1 positional
	// mapping with request.Resources for result reassembly.
	validResources []indexedResource
	// invalidResults are $0/VALIDATION placeholder results for resources that failed pre-flight
	// validation, matching the non-batch adapter behavior.
	invalidResults []batchResult
}

// buildBatchCostRequest constructs a proto BatchCostRequest from indexed resources, validating
// each resource before inclusion. Invalid resources are excluded from the request and returned
// as pre-validated $0/VALIDATION placeholder results matching the non-batch adapter behavior.
func buildBatchCostRequest(
	ctx context.Context, resources []indexedResource, opts batchOptions,
) buildBatchResult {
	log := logging.FromContext(ctx)
	var (
		protoResources []*pbc.ResourceDescriptor
		validResources []indexedResource
		invalidResults []batchResult
	)

	for _, ir := range resources {
		descriptor := proto.PrepareProjectedDescriptor(
			ctx,
			ir.resource.ID,
			ir.resource.Provider,
			ir.resource.Type,
			ConvertToProto(ir.resource.Properties),
			BuildAttributes(ctx, ir.resource.Properties),
		)

		// Pre-flight validation using the same pluginsdk validators as the non-batch path
		if err := validateBatchResource(descriptor, opts); err != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "engine").
				Str("operation", "build_batch_cost_request").
				Str("resource_type", ir.resource.Type).
				Str("resource_id", ir.resource.ID).
				Err(err).
				Msg("pre-flight validation failed, excluding from batch")

			invalidResults = append(invalidResults, newValidationBatchResult(
				ir, err, opts.queryType,
			))
			continue
		}

		protoResources = append(protoResources, descriptor)
		validResources = append(validResources, ir)
	}

	if len(protoResources) == 0 {
		return buildBatchResult{invalidResults: invalidResults}
	}

	return buildBatchResult{
		request: &pbc.BatchCostRequest{
			Resources: protoResources,
			QueryType: opts.queryType,
			Start:     opts.start,
			End:       opts.end,
		},
		validResources: validResources,
		invalidResults: invalidResults,
	}
}

// splitBatchBySize splits a built request into consecutive parts whose summed
// descriptor size stays within budget, keeping each descriptor beside its
// indexedResource. A descriptor larger than the budget forms a part on its own.
// Parts carry no invalid results; the caller records those from built.
func splitBatchBySize(built buildBatchResult, budget int) []buildBatchResult {
	descriptors := built.request.GetResources()
	if len(descriptors) == 0 {
		return []buildBatchResult{built}
	}
	var parts []buildBatchResult
	start, size := 0, 0
	for i, descriptor := range descriptors {
		next := protobuf.Size(descriptor)
		if i > start && size+next > budget {
			parts = append(parts, batchPart(built, start, i))
			start, size = i, 0
		}
		size += next
	}
	if start == 0 {
		return []buildBatchResult{built}
	}
	return append(parts, batchPart(built, start, len(descriptors)))
}

func batchPart(built buildBatchResult, from, to int) buildBatchResult {
	request := &pbc.BatchCostRequest{
		Resources: built.request.GetResources()[from:to],
		QueryType: built.request.GetQueryType(),
		Start:     built.request.GetStart(),
		End:       built.request.GetEnd(),
	}
	return buildBatchResult{request: request, validResources: built.validResources[from:to]}
}

// validateBatchResource validates a single resource descriptor against the pluginsdk validators.
// It constructs a temporary request wrapper matching the query type to reuse the same validation
// logic as the non-batch adapter path.
func validateBatchResource(descriptor *pbc.ResourceDescriptor, opts batchOptions) error {
	if opts.queryType == pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL {
		return pluginsdk.ValidateActualCostRequest(&pbc.GetActualCostRequest{
			ResourceId: descriptor.GetId(),
			Start:      opts.start,
			End:        opts.end,
			Tags:       descriptor.GetTags(),
		})
	}
	return proto.ValidateProjectedDescriptor(descriptor)
}

// newValidationBatchResult creates a batchResult with a $0/VALIDATION placeholder matching
// the non-batch adapter behavior. For projected queries it populates result; for actual
// queries it populates actualResult.
func newValidationBatchResult(ir indexedResource, validationErr error, queryType pbc.CostQueryType) batchResult {
	br := batchResult{index: ir.index}
	placeholder := &CostResult{
		ResourceType: ir.resource.Type,
		ResourceID:   ir.resource.ID,
		Currency:     defaultCurrency,
		Notes:        fmt.Sprintf("VALIDATION: %v", validationErr),
		Error: &StructuredError{
			Code:         ErrCodeValidationError,
			Message:      validationErr.Error(),
			ResourceType: ir.resource.Type,
		},
	}
	if queryType == pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL {
		br.actualResult = placeholder
	} else {
		br.result = placeholder
	}
	return br
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
		GrowthType:     result.GrowthType,
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
// for the actual cost path. When a non-zero request time window ([from, to]) is provided,
// the rate fields (Monthly, Hourly, DailyCosts), Notes, StartDate, EndDate, and CostPeriod
// are derived with the same logic as the per-resource path (getActualCostFromPlugin).
func mapProtoActualCostResultToEngine(
	resource ResourceDescriptor,
	pluginName string,
	result *proto.ActualCostResult,
	from, to time.Time,
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

	if !from.IsZero() && !to.IsZero() {
		monthly, hourly, dailyCosts, notes := deriveActualCostWindow(result.TotalCost, from, to)
		engineResult.Monthly = monthly
		engineResult.Hourly = hourly
		engineResult.DailyCosts = dailyCosts
		engineResult.Notes = notes
		engineResult.StartDate = from
		engineResult.EndDate = to
		engineResult.CostPeriod = FormatPeriod(from, to)
	}

	for k, v := range result.Sustainability {
		engineResult.Sustainability[k] = SustainabilityMetric{
			Value: v.Value,
			Unit:  v.Unit,
		}
	}
	return engineResult
}

// batchChunkTimeout returns the per-chunk BatchCost RPC timeout for a chunk with
// resourceCount resources. It scales with the chunk size, is clamped between
// minBatchTimeout and maxBatchTimeout, and is further capped by the parent
// context's remaining deadline so a single slow chunk cannot consume the entire
// request budget.
func batchChunkTimeout(ctx context.Context, resourceCount int) time.Duration {
	t := max(perResourceTimeout*time.Duration(resourceCount), minBatchTimeout)
	t = min(t, maxBatchTimeout)
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < t {
			t = remaining
		}
	}
	return t
}
