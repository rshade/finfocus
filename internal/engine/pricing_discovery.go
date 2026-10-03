package engine

import (
	"context"
	"strings"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

const billingNotImplemented = "not_implemented"

// PricingTierOption is one volume tier from a plugin pricing spec.
type PricingTierOption struct {
	MinQuantity float64
	MaxQuantity float64
	RatePerUnit float64
	Description string
}

// PricingMetricHint names a usage input that changes the price.
type PricingMetricHint struct {
	Metric string
	Unit   string
}

// PricingMode is one selectable billing mode returned by a plugin.
type PricingMode struct {
	BillingMode string
	RatePerUnit float64
	Unit        string
	Currency    string
	Plugin      string
	Assumptions []string
	MetricHints []PricingMetricHint
	Tiers       []PricingTierOption
}

// PricingDiscovery is the pricing metadata for one resource type.
// An empty Modes slice means the estimate can continue without it.
type PricingDiscovery struct {
	ResourceType string
	Modes        []PricingMode
}

// DiscoverPricingSpec asks each plugin for GetPricingSpec and caches the
// answer for this engine by resource type. Plugin errors and a
// not_implemented billing mode leave Modes empty. A cancelled context is not
// cached.
func (e *Engine) DiscoverPricingSpec(ctx context.Context, resource *ResourceDescriptor) PricingDiscovery {
	if e == nil || resource == nil || ctx == nil || ctx.Err() != nil {
		return PricingDiscovery{}
	}
	resourceType := strings.TrimSpace(resource.Type)
	if resourceType == "" {
		return PricingDiscovery{}
	}
	if found, ok := e.cachedPricingDiscovery(resourceType); ok {
		return found
	}
	found := e.collectPricingDiscovery(ctx, resource, resourceType)
	if ctx.Err() != nil {
		return found
	}
	e.storePricingDiscovery(resourceType, found)
	return clonePricingDiscovery(found)
}

func (e *Engine) cachedPricingDiscovery(resourceType string) (PricingDiscovery, bool) {
	e.pricingDiscoveryMu.Lock()
	defer e.pricingDiscoveryMu.Unlock()
	found, ok := e.pricingDiscoveryCache[resourceType]
	if !ok {
		return PricingDiscovery{}, false
	}
	return clonePricingDiscovery(found), true
}

func (e *Engine) storePricingDiscovery(resourceType string, found PricingDiscovery) {
	e.pricingDiscoveryMu.Lock()
	defer e.pricingDiscoveryMu.Unlock()
	if e.pricingDiscoveryCache == nil {
		e.pricingDiscoveryCache = map[string]PricingDiscovery{}
	}
	e.pricingDiscoveryCache[resourceType] = clonePricingDiscovery(found)
}

func (e *Engine) collectPricingDiscovery(
	ctx context.Context,
	resource *ResourceDescriptor,
	resourceType string,
) PricingDiscovery {
	log := logging.FromContext(ctx)
	found := PricingDiscovery{ResourceType: resourceType}
	for _, client := range e.clients {
		mode, ok := e.pricingModeFromClient(ctx, client, *resource)
		if !ok {
			continue
		}
		found.Modes = append(found.Modes, mode)
	}
	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "discover_pricing_spec").
		Str("resource_type", resourceType).
		Int("modes", len(found.Modes)).
		Msg("pricing spec discovery finished")
	return found
}

func (e *Engine) pricingModeFromClient(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
) (PricingMode, bool) {
	if client == nil || client.API == nil {
		return PricingMode{}, false
	}
	spec, err := fetchPluginPricingSpec(ctx, client, resource)
	if err != nil || spec == nil {
		logging.FromContext(ctx).Debug().
			Ctx(ctx).
			Err(err).
			Str("component", "engine").
			Str("operation", "discover_pricing_spec").
			Str("plugin", client.Name).
			Str("resource_type", resource.Type).
			Msg("pricing spec unavailable")
		return PricingMode{}, false
	}
	return pricingModeFromSpec(spec, client.Name)
}

func pricingModeFromSpec(spec *pbc.PricingSpec, pluginName string) (PricingMode, bool) {
	if spec == nil {
		return PricingMode{}, false
	}
	modeName := strings.TrimSpace(spec.GetBillingMode())
	if modeName == "" || strings.EqualFold(modeName, billingNotImplemented) {
		return PricingMode{}, false
	}
	name := strings.TrimSpace(pluginName)
	if name == "" {
		name = "plugin"
	}
	return PricingMode{
		BillingMode: modeName,
		RatePerUnit: spec.GetRatePerUnit(),
		Unit:        spec.GetUnit(),
		Currency:    spec.GetCurrency(),
		Plugin:      name,
		Assumptions: copyAssumptions(spec.GetAssumptions()),
		MetricHints: copyMetricHints(spec.GetMetricHints()),
		Tiers:       copyPricingTiers(spec.GetPricingTiers()),
	}, true
}

func copyAssumptions(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func copyMetricHints(in []*pbc.UsageMetricHint) []PricingMetricHint {
	if len(in) == 0 {
		return nil
	}
	out := make([]PricingMetricHint, 0, len(in))
	for _, hint := range in {
		if hint == nil || strings.TrimSpace(hint.GetMetric()) == "" {
			continue
		}
		out = append(out, PricingMetricHint{
			Metric: strings.TrimSpace(hint.GetMetric()),
			Unit:   strings.TrimSpace(hint.GetUnit()),
		})
	}
	return out
}

func copyPricingTiers(in []*pbc.PricingTier) []PricingTierOption {
	if len(in) == 0 {
		return nil
	}
	out := make([]PricingTierOption, 0, len(in))
	for _, tier := range in {
		if tier == nil {
			continue
		}
		out = append(out, PricingTierOption{
			MinQuantity: tier.GetMinQuantity(),
			MaxQuantity: tier.GetMaxQuantity(),
			RatePerUnit: tier.GetRatePerUnit(),
			Description: strings.TrimSpace(tier.GetDescription()),
		})
	}
	return out
}

func clonePricingDiscovery(in PricingDiscovery) PricingDiscovery {
	out := PricingDiscovery{ResourceType: in.ResourceType}
	if len(in.Modes) == 0 {
		return out
	}
	out.Modes = make([]PricingMode, len(in.Modes))
	for i, mode := range in.Modes {
		mode.Assumptions = append([]string(nil), mode.Assumptions...)
		mode.MetricHints = append([]PricingMetricHint(nil), mode.MetricHints...)
		mode.Tiers = append([]PricingTierOption(nil), mode.Tiers...)
		out.Modes[i] = mode
	}
	return out
}
