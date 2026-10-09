package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// ErrInvalidPricingMode identifies a selector absent from this resource's discovery.
var ErrInvalidPricingMode = errors.New("invalid pricing mode")

// ID returns an opaque, stable provider and billing-mode identifier.
func (m PricingMode) ID() string {
	value, _ := json.Marshal([2]string{m.Plugin, m.BillingMode})
	return string(value)
}

func (e *Engine) estimateSelectedProvider(ctx context.Context, request *EstimateRequest) (*EstimateResult, error) {
	client, err := e.selectedEstimateClient(ctx, request)
	if err != nil {
		return nil, err
	}
	result, err := e.tryEstimateCostRPC(ctx, client, request)
	if err == nil {
		return result, nil
	}
	if status.Code(err) != codes.Unimplemented {
		return nil, pluginStatusError(err)
	}
	return e.estimateSelectedFallback(ctx, client, request)
}

func (e *Engine) selectedEstimateClient(ctx context.Context, request *EstimateRequest) (*pluginhost.Client, error) {
	discovery := e.DiscoverPricingSpec(ctx, request.Resource)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, mode := range discovery.Modes {
		if mode.ID() != request.PricingMode {
			continue
		}
		for _, client := range e.clients {
			if client != nil && strings.TrimSpace(client.Name) == mode.Plugin {
				return client, nil
			}
		}
	}
	return nil, ErrInvalidPricingMode
}

// Explicit selections bypass the resource-only projected cache. No other plugin
// or local spec may supply a value for a selected provider's billing mode.
func (e *Engine) estimateSelectedFallback(
	ctx context.Context,
	client *pluginhost.Client,
	request *EstimateRequest,
) (*EstimateResult, error) {
	baseline, err := e.selectedProjectedCost(ctx, client, *request.Resource)
	if err != nil {
		return nil, fmt.Errorf("estimate baseline: %w", err)
	}
	modifiedResource := *request.Resource
	modifiedResource.Properties = mergePropertiesWithOverrides(request.Resource.Properties, request.PropertyOverrides)
	validationResource := modifiedResource
	validationResource.Properties = redactedProperties(ctx, modifiedResource.Properties)
	if err = validationResource.Validate(); err != nil {
		return nil, err
	}
	modified, err := e.selectedProjectedCost(ctx, client, modifiedResource)
	if err != nil {
		return nil, fmt.Errorf("estimate modified: %w", err)
	}
	if baseline.Currency != modified.Currency {
		return nil, errCurrencyMismatch
	}
	change := modified.Monthly - baseline.Monthly
	return &EstimateResult{
		Resource:     request.Resource,
		Baseline:     baseline,
		Modified:     modified,
		TotalChange:  change,
		Deltas:       buildCostDeltas(request.PropertyOverrides, request.Resource.Properties, change),
		UsedFallback: true,
	}, nil
}

func (e *Engine) selectedProjectedCost(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
) (*CostResult, error) {
	ctx, cancel := context.WithTimeout(ctx, perResourceTimeout)
	defer cancel()
	resource.Properties = redactedProperties(ctx, resource.Properties)
	result, err := e.getProjectedCostFromPlugin(ctx, client, resource)
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, errors.New("selected provider returned a pricing error")
	}
	if err = validateEstimateResponse(
		&pbc.EstimateCostResponse{CostMonthly: result.Monthly, Currency: result.Currency},
	); err != nil {
		return nil, err
	}
	return result, nil
}
