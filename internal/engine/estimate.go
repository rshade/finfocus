package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// EstimateCost performs what-if cost analysis with property overrides.
//
// It first attempts to use the EstimateCost RPC if the plugin implements it.
// If the RPC is unimplemented, it falls back to calling GetProjectedCost twice:
// once with original properties (baseline) and once with overrides applied (modified).
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - request: The estimate request containing resource and property overrides
//
// Returns:
//   - *EstimateResult: The estimation result with baseline, modified costs, and deltas
//   - error: Any error encountered during estimation
//
// The method logs at appropriate levels:
//   - DEBUG: Entry/exit, fallback decisions
//   - INFO: Successful estimations
//   - WARN: Fallback usage
//   - ERROR: Failed estimations
//
//nolint:funlen // Function is logically cohesive with clear sections; splitting would reduce readability.
func (e *Engine) EstimateCost(
	ctx context.Context,
	request *EstimateRequest,
) (*EstimateResult, error) {
	log := logging.FromContext(ctx)
	start := time.Now()

	// Validate request and resource are not nil to prevent nil pointer panic
	if request == nil {
		return nil, errors.New("estimate request cannot be nil")
	}
	if request.Resource == nil {
		return nil, errors.New("estimate request resource cannot be nil")
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "estimate_cost").
		Str("resource_type", request.Resource.Type).
		Str("resource_id", request.Resource.ID).
		Int("override_count", len(request.PropertyOverrides)).
		Msg("starting cost estimation")

	// Validate the resource before processing
	if err := request.Resource.Validate(); err != nil {
		return nil, err
	}

	// Try EstimateCost RPC on available plugins
	for _, client := range e.clients {
		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("plugin", client.Name).
			Msg("attempting EstimateCost RPC")

		// Apply per-resource timeout for plugin calls
		resourceCtx, resourceCancel := context.WithTimeout(ctx, perResourceTimeout)
		result, err := e.tryEstimateCostRPC(resourceCtx, client, request)
		resourceCancel()

		if err != nil {
			// Check if the error is Unimplemented - if so, try fallback
			if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
				log.Info().
					Ctx(ctx).
					Str("component", "engine").
					Str("plugin", client.Name).
					Msg("EstimateCost RPC not implemented, using fallback")
				continue
			}

			// Check for context cancellation - respect user interruption
			if errors.Is(err, context.Canceled) {
				log.Debug().
					Ctx(ctx).
					Str("component", "engine").
					Msg("estimation canceled by user")
				return nil, context.Canceled
			}

			// Check for context timeout specifically for better diagnostics
			if errors.Is(err, context.DeadlineExceeded) {
				log.Warn().
					Ctx(ctx).
					Str("component", "engine").
					Str("plugin", client.Name).
					Dur("timeout", perResourceTimeout).
					Msg("EstimateCost RPC timed out, trying next plugin")
				continue
			}

			// Other errors - log and try next plugin
			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("plugin", client.Name).
				Err(err).
				Msg("EstimateCost RPC failed, trying next plugin")
			continue
		}

		if result != nil {
			log.Info().
				Ctx(ctx).
				Str("component", "engine").
				Str("operation", "estimate_cost").
				Str("plugin", client.Name).
				Float64("total_change", result.TotalChange).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("cost estimation complete via RPC")
			return result, nil
		}
	}

	// Fallback: Use GetProjectedCost twice (baseline + modified)
	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Msg("using fallback strategy: double GetProjectedCost")

	result, err := e.estimateCostFallback(ctx, request)
	if err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "engine").
			Err(err).
			Msg("fallback cost estimation failed")
		return nil, err
	}

	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "estimate_cost").
		Float64("total_change", result.TotalChange).
		Bool("used_fallback", true).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("cost estimation complete via fallback")

	return result, nil
}

// tryEstimateCostRPC attempts to call the EstimateCost RPC on a plugin.
func (e *Engine) tryEstimateCostRPC(
	_ context.Context,
	_ *pluginhost.Client,
	_ *EstimateRequest,
) (*EstimateResult, error) {
	// The EstimateCost RPC is not yet defined in finfocus-spec v0.5.6
	// When the RPC is added to the spec, this method should:
	// 1. Build the proto request using proto.BuildEstimateCostRequest
	// 2. Call client.API.EstimateCost(ctx, protoReq)
	// 3. Convert the response to EstimateResult
	return nil, status.Error(codes.Unimplemented, "EstimateCost RPC not yet implemented in plugins")
}

// estimateCostFallback calculates cost estimation using two GetProjectedCost calls.
//
// This fallback strategy:
// 1. Calls GetProjectedCost with original properties -> baseline
// 2. Merges property overrides into resource properties
// 3. Calls GetProjectedCost with modified properties -> modified
// 4. Calculates the delta
//
// Note: When multiple properties are overridden simultaneously, the fallback
// cannot provide accurate per-property delta breakdown. In this case, it reports
// a single "combined" delta representing the total change.
//
//nolint:funlen // Function has clear sections for baseline, modified, and delta calculations.
func (e *Engine) estimateCostFallback(
	ctx context.Context,
	request *EstimateRequest,
) (*EstimateResult, error) {
	log := logging.FromContext(ctx)

	// Get baseline cost with original properties
	baselineResources := []ResourceDescriptor{*request.Resource}
	baselineResults, err := e.GetProjectedCost(ctx, baselineResources)
	if err != nil {
		return nil, err
	}

	var baseline *CostResult
	if len(baselineResults) > 0 {
		baseline = &baselineResults[0]
	} else {
		baseline = &CostResult{
			ResourceType: request.Resource.Type,
			ResourceID:   request.Resource.ID,
			Currency:     defaultCurrency,
			Monthly:      0,
			Hourly:       0,
			Notes:        "No baseline cost data available",
		}
	}

	// Create modified resource with overrides applied
	// IMPORTANT: Deep copy the properties map to avoid modifying the original
	modifiedResource := *request.Resource
	modifiedResource.Properties = make(map[string]interface{})
	for key, value := range request.Resource.Properties {
		modifiedResource.Properties[key] = value
	}
	for key, value := range request.PropertyOverrides {
		modifiedResource.Properties[key] = value
	}

	// Validate the modified resource to ensure overrides don't violate constraints
	if validateErr := modifiedResource.Validate(); validateErr != nil {
		return nil, fmt.Errorf("modified resource validation failed: %w", validateErr)
	}

	// Get modified cost with overrides applied
	modifiedResources := []ResourceDescriptor{modifiedResource}
	modifiedResults, err := e.GetProjectedCost(ctx, modifiedResources)
	if err != nil {
		return nil, err
	}

	var modified *CostResult
	if len(modifiedResults) > 0 {
		modified = &modifiedResults[0]
	} else {
		modified = &CostResult{
			ResourceType: request.Resource.Type,
			ResourceID:   request.Resource.ID,
			Currency:     defaultCurrency,
			Monthly:      0,
			Hourly:       0,
			Notes:        "No modified cost data available",
		}
	}

	// Calculate total change
	totalChange := modified.Monthly - baseline.Monthly

	// Build deltas - for fallback, we can only report a combined delta
	var deltas []CostDelta
	if len(request.PropertyOverrides) == 1 {
		// Single property change - we can attribute the delta to it
		for key, newValue := range request.PropertyOverrides {
			// Get original value from the ORIGINAL resource (not modified)
			originalValue := ""
			if request.Resource.Properties != nil {
				if v, ok := request.Resource.Properties[key]; ok {
					originalValue = formatPropertyValue(v)
				}
			}
			deltas = append(deltas, CostDelta{
				Property:      key,
				OriginalValue: originalValue,
				NewValue:      newValue,
				CostChange:    totalChange,
			})
		}
	} else if len(request.PropertyOverrides) > 1 {
		// Multiple properties - report as combined
		deltas = append(deltas, CostDelta{
			Property:      "combined",
			OriginalValue: "",
			NewValue:      "",
			CostChange:    totalChange,
		})
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Float64("baseline_monthly", baseline.Monthly).
		Float64("modified_monthly", modified.Monthly).
		Float64("total_change", totalChange).
		Msg("fallback estimation calculated")

	return &EstimateResult{
		Resource:     request.Resource,
		Baseline:     baseline,
		Modified:     modified,
		TotalChange:  totalChange,
		Deltas:       deltas,
		UsedFallback: true,
	}, nil
}

// formatPropertyValue converts a property value to a string representation.
func formatPropertyValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}
