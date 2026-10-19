package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

const carbonFeature = "Carbon"

// isCarbonOnly distinguishes an impact source from a monetary pricing source.
// Unknown capabilities keep the legacy monetary routing behavior.
func isCarbonOnly(client *pluginhost.Client) bool {
	return client.HasCapability("carbon") &&
		!client.HasCapability("projected_costs") && !client.HasCapability("actual_costs") &&
		!client.HasCapability("pricing_spec") && !client.HasCapability("batch_cost")
}

func allowsCarbonFeature(client *pluginhost.Client, feature string) bool {
	if client == nil {
		return false
	}
	if feature == carbonFeature {
		return client.HasCapability("carbon")
	}
	switch feature {
	case "ProjectedCosts", "ActualCosts", "BatchCost", "PricingSpec":
		return !isCarbonOnly(client)
	default:
		return true
	}
}

// enrichProjectedCarbon runs after monetary cache reads/writes, so a carbon
// source's process-local assumptions never enter the monetary cache. It mutates
// only the existing row's missing carbon and notes; monetary fields are retained.
func (e *Engine) enrichProjectedCarbon(
	ctx context.Context,
	resource ResourceDescriptor,
	results []CostResult,
) []ErrorDetail {
	if len(results) == 0 || hasCarbon(results[0].Sustainability) || !e.hasCarbonSource() {
		return nil
	}
	carbonCtx, cancel := context.WithTimeout(ctx, perResourceTimeout)
	defer cancel()
	matches, declines := e.selectPluginMatchesForResource(carbonCtx, resource, carbonFeature)
	for _, decline := range declines {
		addCarbonNote(&results[0], fmt.Sprintf("%s declined: %s", decline.plugin, decline.reason))
	}
	var failures []ErrorDetail
	for _, match := range matches {
		err := e.mergeCarbon(carbonCtx, resource, &results[0], match.Client)
		if err == nil {
			return failures
		}
		addCarbonNote(&results[0], fmt.Sprintf("%s unavailable: %s", match.Client.Name, err))
		logging.FromContext(ctx).Warn().Err(err).Str("component", "engine").
			Str("operation", "enrich_projected_carbon").
			Str("plugin", match.Client.Name).Msg("carbon enrichment unavailable")
		failures = append(failures, ErrorDetail{
			ResourceType: resource.Type, ResourceID: resource.ID, PluginName: match.Client.Name,
			Error: fmt.Errorf("carbon enrichment failed: %w", err), Timestamp: time.Now(),
		})
		if !match.Fallback || carbonCtx.Err() != nil {
			break
		}
	}
	return failures
}

func (e *Engine) mergeCarbon(
	ctx context.Context,
	resource ResourceDescriptor,
	result *CostResult,
	client *pluginhost.Client,
) error {
	carbon, err := e.getProjectedCostFromPlugin(ctx, client, resource, e.usageProfile)
	if err != nil {
		return err
	}
	metric, ok := carbon.Sustainability[greenops.CarbonMetricKey]
	if !ok {
		metric, ok = carbon.Sustainability[greenops.DeprecatedCarbonKey]
	}
	if !ok {
		return errors.New("response has no carbon measurement")
	}
	if _, err = greenops.NormalizeToKg(metric.Value, metric.Unit); err != nil {
		return fmt.Errorf("invalid carbon measurement: %w", err)
	}
	result.Sustainability = maps.Clone(result.Sustainability)
	if result.Sustainability == nil {
		result.Sustainability = make(map[string]SustainabilityMetric)
	}
	result.Sustainability[greenops.CarbonMetricKey] = metric
	addCarbonNote(result, fmt.Sprintf("%s: %s", client.Name, carbon.Notes))
	return nil
}

func hasCarbon(metrics map[string]SustainabilityMetric) bool {
	_, canonical := metrics[greenops.CarbonMetricKey]
	_, legacy := metrics[greenops.DeprecatedCarbonKey]
	return canonical || legacy
}

func addCarbonNote(result *CostResult, note string) {
	if result.Notes != "" {
		result.Notes += "; "
	}
	result.Notes += "carbon " + note
}

func (e *Engine) hasCarbonSource() bool {
	for _, client := range e.clients {
		if client != nil && client.HasCapability("carbon") {
			return true
		}
	}
	return false
}

// enrichBatchProjectedCarbon waits until monetary workers have completed before
// starting optional carbon queries. Slow impact sources cannot delay monetary
// work or serialize every batch row behind one RPC timeout.
func (e *Engine) enrichBatchProjectedCarbon(
	ctx context.Context, resources []ResourceDescriptor, batches map[int][]CostResult,
) {
	if !e.hasCarbonSource() || len(batches) == 0 {
		return
	}
	jobs := make(chan int, len(batches))
	for index := range batches {
		jobs <- index
	}
	close(jobs)
	var workers sync.WaitGroup
	for range e.getWorkerCount(len(batches)) {
		workers.Go(func() {
			for index := range jobs {
				e.enrichProjectedCarbon(ctx, resources[index], batches[index])
			}
		})
	}
	workers.Wait()
}
