package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// combinedDeltaProperty is the sentinel name used for multi-property deltas
// where per-property attribution is not possible.
const combinedDeltaProperty = "combined"

var (
	// errNilEstimateResponse indicates a plugin returned a nil EstimateCostResponse.
	errNilEstimateResponse = errors.New("plugin returned nil EstimateCost response")

	// errNegativeEstimateCost indicates a plugin returned a negative CostMonthly value.
	errNegativeEstimateCost = errors.New("plugin returned negative EstimateCost cost")

	// errNonFiniteEstimateCost indicates a plugin returned NaN or Inf for CostMonthly.
	errNonFiniteEstimateCost = errors.New("plugin returned non-finite EstimateCost cost (NaN or Inf)")

	// errEmptyEstimateCurrency indicates a plugin returned an empty currency string.
	errEmptyEstimateCurrency = errors.New("plugin returned empty currency in EstimateCost response")

	// errCurrencyMismatch indicates baseline and modified responses have different currencies.
	errCurrencyMismatch = errors.New("currency mismatch between baseline and modified EstimateCost responses")
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

	// Validate that at least one property override is provided
	if len(request.PropertyOverrides) == 0 {
		return nil, errors.New("property overrides are required for cost estimation")
	}

	// Try EstimateCost RPC on available plugins
	for _, client := range e.clients {
		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("plugin", client.Name).
			Msg("attempting EstimateCost RPC")

		result, err := e.tryEstimateCostRPC(ctx, client, request)

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

// validateEstimateResponse checks that a plugin's EstimateCostResponse is usable.
func validateEstimateResponse(resp *pbc.EstimateCostResponse) error {
	if resp == nil {
		return errNilEstimateResponse
	}
	cost := resp.GetCostMonthly()
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		return errNonFiniteEstimateCost
	}
	if cost < 0 {
		return errNegativeEstimateCost
	}
	if resp.GetCurrency() == "" {
		return errEmptyEstimateCurrency
	}
	return nil
}

// tryEstimateCostRPC attempts to call the EstimateCost RPC on a plugin.
// It calls the RPC twice: once with original properties (baseline) and once
// with overrides applied (modified), then computes deltas.
func (e *Engine) tryEstimateCostRPC(
	ctx context.Context,
	client *pluginhost.Client,
	request *EstimateRequest,
) (*EstimateResult, error) {
	log := logging.FromContext(ctx)
	resourceType := request.Resource.Type

	baselineReq, err := proto.BuildEstimateCostRequest(resourceType, request.Resource.Properties)
	if err != nil {
		return nil, fmt.Errorf("build baseline request: %w", err)
	}

	baselineCtx, baselineCancel := context.WithTimeout(ctx, perResourceTimeout)
	baselineResp, err := client.API.EstimateCost(baselineCtx, baselineReq)
	baselineCancel()
	if err != nil {
		return nil, err
	}

	if err = validateEstimateResponse(baselineResp); err != nil {
		log.Warn().Str("plugin", client.Name).Err(err).Msg("invalid baseline response")
		return nil, err
	}

	modifiedProps := mergePropertiesWithOverrides(request.Resource.Properties, request.PropertyOverrides)

	modifiedReq, err := proto.BuildEstimateCostRequest(resourceType, modifiedProps)
	if err != nil {
		return nil, fmt.Errorf("build modified request: %w", err)
	}

	modifiedCtx, modifiedCancel := context.WithTimeout(ctx, perResourceTimeout)
	modifiedResp, err := client.API.EstimateCost(modifiedCtx, modifiedReq)
	modifiedCancel()
	if err != nil {
		return nil, err
	}

	if err = validateEstimateResponse(modifiedResp); err != nil {
		log.Warn().Str("plugin", client.Name).Err(err).Msg("invalid modified response")
		return nil, err
	}

	// Guard against cross-currency delta computation
	if baselineResp.GetCurrency() != modifiedResp.GetCurrency() {
		log.Warn().
			Str("plugin", client.Name).
			Str("baseline_currency", baselineResp.GetCurrency()).
			Str("modified_currency", modifiedResp.GetCurrency()).
			Msg("currency mismatch between baseline and modified responses")
		return nil, errCurrencyMismatch
	}

	baseline := estimateResponseToCostResult(baselineResp, request.Resource)
	modified := estimateResponseToCostResult(modifiedResp, request.Resource)
	totalChange := modified.Monthly - baseline.Monthly

	return &EstimateResult{
		Resource:     request.Resource,
		Baseline:     baseline,
		Modified:     modified,
		TotalChange:  totalChange,
		Deltas:       buildCostDeltas(request.PropertyOverrides, request.Resource.Properties, totalChange),
		UsedFallback: false,
	}, nil
}

// estimateResponseToCostResult converts a pbc.EstimateCostResponse to an engine CostResult.
func estimateResponseToCostResult(resp *pbc.EstimateCostResponse, resource *ResourceDescriptor) *CostResult {
	currency := resp.GetCurrency()
	if currency == "" {
		currency = defaultCurrency
	}

	monthly := resp.GetCostMonthly()
	hourly := monthly / hoursPerMonth

	var notes []string
	if resp.GetPricingCategory() != pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_UNSPECIFIED {
		notes = append(notes, "Pricing: "+resp.GetPricingCategory().String())
	}
	if resp.GetSpotInterruptionRiskScore() > 0 {
		notes = append(notes, fmt.Sprintf("Spot risk: %.2f", resp.GetSpotInterruptionRiskScore()))
	}

	return &CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Currency:     currency,
		Monthly:      monthly,
		Hourly:       hourly,
		Notes:        strings.Join(notes, "; "),
	}
}

// estimateCostFallback calculates cost estimation using two GetProjectedCost calls.
// When multiple properties are overridden simultaneously, it reports a single
// "combined" delta since per-property attribution is not possible via this path.
func (e *Engine) estimateCostFallback(
	ctx context.Context,
	request *EstimateRequest,
) (*EstimateResult, error) {
	log := logging.FromContext(ctx)

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

	modifiedResource := *request.Resource
	modifiedResource.Properties = mergePropertiesWithOverrides(request.Resource.Properties, request.PropertyOverrides)

	if validateErr := modifiedResource.Validate(); validateErr != nil {
		return nil, fmt.Errorf("modified resource validation failed: %w", validateErr)
	}

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

	totalChange := modified.Monthly - baseline.Monthly

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
		Deltas:       buildCostDeltas(request.PropertyOverrides, request.Resource.Properties, totalChange),
		UsedFallback: true,
	}, nil
}

// mergePropertiesWithOverrides creates a shallow copy of properties with string
// overrides applied. The returned map is safe to mutate without affecting the originals.
func mergePropertiesWithOverrides(
	properties map[string]any,
	overrides map[string]string,
) map[string]any {
	merged := make(map[string]any, len(properties)+len(overrides))
	for k, v := range properties {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// buildCostDeltas constructs per-property delta entries from overrides and a total cost change.
// Single-override requests get an attributed delta; multi-override requests get a "combined" entry.
func buildCostDeltas(
	overrides map[string]string,
	originalProperties map[string]any,
	totalChange float64,
) []CostDelta {
	if len(overrides) == 1 {
		for key, newValue := range overrides {
			originalValue := ""
			if originalProperties != nil {
				if v, ok := originalProperties[key]; ok {
					originalValue = ConvertValueToString(v)
				}
			}
			return []CostDelta{{
				Property:      key,
				OriginalValue: originalValue,
				NewValue:      newValue,
				CostChange:    totalChange,
			}}
		}
	}
	if len(overrides) > 1 {
		return []CostDelta{{Property: combinedDeltaProperty, CostChange: totalChange}}
	}
	return nil
}
