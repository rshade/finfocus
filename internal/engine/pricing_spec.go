package engine

import (
	"context"
	"math"
	"strings"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const (
	adapterPluginSpec = "plugin-spec"

	billingPerHour     = "per_hour"
	billingPerDay      = "per_day"
	billingPerGBMonth  = "per_gb_month"
	billingPerRequest  = "per_request"
	billingFlat        = "flat"
	billingPerCPUHour  = "per_cpu_hour"
	billingTiered      = "tiered"
	pluginSpecNoteLead = "Calculated from plugin pricing spec"

	rawRequest  = "request"
	rawRequests = "requests"
)

// WithPricingSpecFallback enables pricing a projected resource from plugin
// GetPricingSpec after GetProjectedCost misses and before local YAML.
// The default is off, and a disabled engine does not call the RPC.
func (e *Engine) WithPricingSpecFallback(enabled bool) *Engine {
	if e == nil {
		return nil
	}
	e.pricingSpecFallback = enabled
	return e
}

// projectedFallbackResult prices one resource after every selected plugin's
// GetProjectedCost missed. Plugin GetPricingSpec runs only when enabled.
// Local YAML remains the next source, then the no-pricing placeholder.
func (e *Engine) projectedFallbackResult(
	ctx context.Context,
	resource ResourceDescriptor,
	matches []PluginMatch,
	declines []pluginDecline,
) CostResult {
	log := logging.FromContext(ctx)
	if specRes := e.projectedCostFromPluginPricingSpec(ctx, resource, matches); specRes != nil {
		log.Info().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "projected_pricing_spec").
			Str("resource_type", resource.Type).
			Str("resource_id", resource.ID).
			Str("adapter", specRes.Adapter).
			Float64("monthly_cost", specRes.Monthly).
			Msg("projected cost from plugin pricing spec")
		return *specRes
	}

	if e.loader != nil {
		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("resource_type", resource.Type).
			Str("resource_id", resource.ID).
			Msg("no plugin data, trying spec fallback")
		if specRes := e.getProjectedCostFromSpec(ctx, resource); specRes != nil {
			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("resource_type", resource.Type).
				Float64("monthly_cost", specRes.Monthly).
				Msg("spec fallback provided cost data")
			return *specRes
		}
	}

	log.Warn().
		Ctx(ctx).
		Str("component", "engine").
		Str("resource_type", resource.Type).
		Str("resource_id", resource.ID).
		Msg("no pricing data available from plugins or specs")
	notes := declineNotes(noteNoPricingInfo, declines)
	return CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Adapter:      adapterNone,
		Currency:     defaultCurrency,
		Notes:        notes,
		Error: &StructuredError{
			Code:         ErrCodeNoCostData,
			Message:      notes,
			ResourceType: resource.Type,
		},
	}
}

// projectedCostFromPluginPricingSpec returns the first usable plugin pricing
// spec. A disabled engine returns nil before any plugin call.
func (e *Engine) projectedCostFromPluginPricingSpec(
	ctx context.Context,
	resource ResourceDescriptor,
	matches []PluginMatch,
) *CostResult {
	if e == nil || !e.pricingSpecFallback {
		return nil
	}
	for _, match := range matches {
		if result := e.costFromPricingSpecMatch(ctx, resource, match); result != nil {
			return result
		}
	}
	return nil
}

func (e *Engine) costFromPricingSpecMatch(
	ctx context.Context,
	resource ResourceDescriptor,
	match PluginMatch,
) *CostResult {
	if match.Client == nil || match.Client.API == nil {
		return nil
	}
	pluginName := match.Client.Name
	spec, err := fetchPluginPricingSpec(ctx, match.Client, resource)
	if err != nil || spec == nil {
		logging.FromContext(ctx).Debug().
			Ctx(ctx).
			Err(err).
			Str("component", "engine").
			Str("operation", "projected_pricing_spec").
			Str("plugin", pluginName).
			Str("resource_id", resource.ID).
			Msg("plugin pricing spec unavailable")
		return nil
	}
	result, ok := costFromPluginPricingSpec(resource, spec, pluginName)
	if !ok {
		logging.FromContext(ctx).Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "projected_pricing_spec").
			Str("plugin", pluginName).
			Str("resource_id", resource.ID).
			Str("billing_mode", spec.GetBillingMode()).
			Msg("plugin pricing spec could not be priced")
		return nil
	}
	return result
}

func fetchPluginPricingSpec(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
) (*pbc.PricingSpec, error) {
	descriptor := proto.PrepareProjectedDescriptor(
		ctx,
		resource.ID,
		resource.Provider,
		resource.Type,
		ConvertToProto(resource.Properties),
	)
	resp, err := client.API.GetPricingSpec(ctx, &pbc.GetPricingSpecRequest{Resource: descriptor})
	if err != nil {
		return nil, err
	}
	if resp.GetSpec() == nil {
		return nil, ErrNoCostData
	}
	return resp.GetSpec(), nil
}

// costFromPluginPricingSpec converts a plugin PricingSpec into a monthly and
// hourly estimate. It reports false when the spec has no usable rate.
func costFromPluginPricingSpec(
	resource ResourceDescriptor,
	spec *pbc.PricingSpec,
	pluginName string,
) (*CostResult, bool) {
	if spec == nil {
		return nil, false
	}
	rate, mode, ok := pluginSpecRate(spec, resource)
	if !ok || invalidPluginRate(rate) {
		return nil, false
	}
	monthly, hourly, assumed, ok := pluginSpecAmounts(mode, rate, resource)
	if !ok {
		return nil, false
	}

	currency := strings.TrimSpace(spec.GetCurrency())
	if currency == "" {
		currency = defaultCurrency
	}
	notes := pluginSpecNotes(spec, pluginName, mode, assumed)
	return &CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Adapter:      adapterPluginSpec,
		Currency:     currency,
		Monthly:      monthly,
		Hourly:       hourly,
		Notes:        notes,
		Breakdown:    map[string]float64{"base_cost": monthly},
	}, true
}

func pluginSpecNotes(spec *pbc.PricingSpec, pluginName, mode, assumed string) string {
	source := strings.TrimSpace(spec.GetSource())
	if source == "" {
		source = strings.TrimSpace(spec.GetProvider())
	}
	if source == "" {
		source = strings.TrimSpace(pluginName)
	}
	if source == "" {
		source = "plugin"
	}
	label := strings.TrimSpace(spec.GetBillingMode())
	if label == "" {
		label = strings.TrimSpace(spec.GetUnit())
	}
	if label == "" {
		label = mode
	}
	notes := pluginSpecNoteLead + ": " + source + " (" + label + ")"
	if assumed != "" {
		notes += "; assumed " + assumed
	}
	return notes
}

func pluginSpecRate(spec *pbc.PricingSpec, resource ResourceDescriptor) (float64, string, bool) {
	rawMode := strings.ToLower(strings.TrimSpace(spec.GetBillingMode()))
	if rawMode == billingTiered {
		tier := matchingPricingTier(spec.GetPricingTiers(), tierQuantity(spec.GetUnit(), resource))
		if tier == nil {
			return 0, "", false
		}
		mode := normalizeBilling("", spec.GetUnit())
		if mode == "" {
			mode = billingFlat
		}
		return tier.GetRatePerUnit(), mode, true
	}
	mode := normalizeBilling(rawMode, spec.GetUnit())
	if mode == "" {
		return 0, "", false
	}
	return spec.GetRatePerUnit(), mode, true
}

func pluginSpecAmounts(
	mode string, rate float64, resource ResourceDescriptor,
) (float64, float64, string, bool) {
	switch mode {
	case billingPerHour:
		return rate * hoursPerMonth, rate, "", true
	case billingPerDay:
		monthly := rate * daysPerMonth
		return monthly, monthly / hoursPerMonth, "", true
	case billingPerGBMonth:
		qty, assumed := sizedQuantity(resource, "gb")
		monthly := rate * qty
		return monthly, monthly / hoursPerMonth, assumed, true
	case billingPerRequest:
		qty, assumed := sizedQuantity(resource, rawRequest)
		monthly := rate * qty
		return monthly, monthly / hoursPerMonth, assumed, true
	case billingFlat:
		return rate, rate / hoursPerMonth, "", true
	case billingPerCPUHour:
		qty, assumed := sizedQuantity(resource, "cpu")
		hourly := rate * qty
		return hourly * hoursPerMonth, hourly, assumed, true
	default:
		return 0, 0, "", false
	}
}

func invalidPluginRate(rate float64) bool {
	return rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0)
}

func normalizeBilling(mode, unit string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "per_hour", "hourly", "hour":
		return billingPerHour
	case "per_day", "daily", "day":
		return billingPerDay
	case "per_gb_month", "gb-month", "gb_month", "per_gb":
		return billingPerGBMonth
	case billingPerRequest, rawRequest, rawRequests:
		return billingPerRequest
	case billingFlat, string(GroupByMonthly), "per_month":
		return billingFlat
	case "per_cpu_hour", "cpu-hour", "cpu_hour":
		return billingPerCPUHour
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "hour", "hours", "hr":
		return billingPerHour
	case "day", "days":
		return billingPerDay
	case "gb-month", "gb_month", "gb":
		return billingPerGBMonth
	case rawRequest, rawRequests:
		return billingPerRequest
	case "month", string(GroupByMonthly):
		return billingFlat
	case "cpu-hour", "cpu_hour":
		return billingPerCPUHour
	default:
		return ""
	}
}

func matchingPricingTier(tiers []*pbc.PricingTier, qty float64) *pbc.PricingTier {
	var fallback *pbc.PricingTier
	for _, tier := range tiers {
		if tier == nil {
			continue
		}
		if fallback == nil {
			fallback = tier
		}
		maxQty := tier.GetMaxQuantity()
		if qty >= tier.GetMinQuantity() && (maxQty == 0 || qty < maxQty) {
			return tier
		}
	}
	return fallback
}

func tierQuantity(unit string, resource ResourceDescriptor) float64 {
	switch normalizeBilling("", unit) {
	case billingPerGBMonth:
		qty, _ := sizedQuantity(resource, "gb")
		return qty
	case billingPerCPUHour:
		qty, _ := sizedQuantity(resource, "cpu")
		return qty
	case billingPerRequest:
		qty, _ := sizedQuantity(resource, rawRequest)
		return qty
	default:
		return 1
	}
}

func sizedQuantity(resource ResourceDescriptor, kind string) (float64, string) {
	var qty float64
	var ok bool
	var assumed string
	switch kind {
	case "gb":
		qty, ok = getStorageSize(resource)
		assumed = "1 GB"
	case "cpu":
		qty, ok = propertyQuantity(resource, []string{"cpu", "vcpu", "cpuCount"})
		assumed = "1 CPU"
	case rawRequest:
		qty, ok = propertyQuantity(resource, []string{
			rawRequests, "requestCount", "monthlyRequests", "invocations",
		})
		assumed = "1 request"
	default:
		return 1, ""
	}
	if ok && qty > 0 {
		return qty, ""
	}
	return 1, assumed
}

func propertyQuantity(resource ResourceDescriptor, keys []string) (float64, bool) {
	if resource.Properties == nil {
		return 0, false
	}
	for _, key := range keys {
		value, found := resource.Properties[key]
		if !found {
			continue
		}
		if qty, ok := parseFloatValue(value); ok {
			return qty, true
		}
	}
	return 0, false
}
