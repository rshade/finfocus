// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"
)

// GetProjectedCostDiff prices each resource twice when the plan changed it:
// old properties are the baseline and current properties are the result.
// create and an empty operation have a $0 baseline. delete has a $0 result.
// same is priced once and reused. summary totalAfter is the projected bill
// after the plan, which is what cost projected reported before this diff.
func (e *Engine) GetProjectedCostDiff(
	ctx context.Context, resources []ResourceDescriptor,
) (*DiffResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, errors.New("calculating projected cost diff: engine is nil")
	}
	if len(resources) == 0 {
		return &DiffResult{
			Entries: []DiffEntry{},
			Summary: DiffSummary{Currency: defaultCurrency},
		}, nil
	}

	plan := planDiffPricing(ctx, resources)
	afterCosts, afterErrs, err := e.priceAligned(ctx, plan.after)
	if err != nil {
		return nil, err
	}
	beforeCosts, beforeErrs, err := e.priceAligned(ctx, plan.before)
	if err != nil {
		return nil, err
	}
	return assembleDiff(resources, plan, beforeCosts, afterCosts, append(afterErrs, beforeErrs...)), nil
}

type diffPricingPlan struct {
	before   []ResourceDescriptor
	beforeAt []int
	after    []ResourceDescriptor
	afterAt  []int
	ops      []string
}

func planDiffPricing(ctx context.Context, resources []ResourceDescriptor) diffPricingPlan {
	plan := diffPricingPlan{ops: make([]string, len(resources))}
	refTags := resolveDiffRefTags(ctx, resources)
	for i, resource := range resources {
		op := normalizeDiffOperation(resource.Operation)
		plan.ops[i] = op
		switch op {
		case DiffOperationDelete:
			plan.before = append(plan.before, descriptorForDiff(resource, deleteBaseline(resource), refTags[i]))
			plan.beforeAt = append(plan.beforeAt, i)
		case DiffOperationUpdate:
			plan.before = append(plan.before, descriptorForDiff(resource, resource.OldProperties, refTags[i]))
			plan.beforeAt = append(plan.beforeAt, i)
			plan.after = append(plan.after, descriptorForDiff(resource, resource.Properties, refTags[i]))
			plan.afterAt = append(plan.afterAt, i)
		default:
			plan.after = append(plan.after, descriptorForDiff(resource, resource.Properties, refTags[i]))
			plan.afterAt = append(plan.afterAt, i)
		}
	}
	return plan
}

// descriptorForDiff copies resource and swaps in props plus the plan-wide ref
// tags. Refs is cleared because the tags are already resolved: the before and
// after slices hold different resource sets, so resolving inside either one
// would miss targets that are only present in the other.
func descriptorForDiff(resource ResourceDescriptor, props, refTags map[string]any) ResourceDescriptor {
	out := resource
	out.Properties = cloneProperties(props)
	if len(refTags) > 0 {
		if out.Properties == nil {
			out.Properties = make(map[string]any, len(refTags))
		}
		maps.Copy(out.Properties, refTags)
	}
	out.Refs = nil
	return out
}

// resolveDiffRefTags resolves cross-resource references once over the whole
// plan and returns the ref.<property>.* tags each resource earns, by index. The
// diff prices before and after from separate slices, so both sides take these
// tags to stay on the same basis.
func resolveDiffRefTags(ctx context.Context, resources []ResourceDescriptor) []map[string]any {
	tags := make([]map[string]any, len(resources))
	hasRefs := false
	work := make([]ResourceDescriptor, len(resources))
	for i, resource := range resources {
		work[i] = resource
		work[i].Properties = cloneProperties(resource.Properties)
		hasRefs = hasRefs || len(resource.Refs) > 0
	}
	if !hasRefs {
		return tags
	}
	ApplyCrossResourceRefs(ctx, work)
	for i := range work {
		for prop := range work[i].Refs {
			prefix := "ref." + prop + "."
			for key, value := range work[i].Properties {
				if !strings.HasPrefix(key, prefix) {
					continue
				}
				if tags[i] == nil {
					tags[i] = make(map[string]any)
				}
				tags[i][key] = value
			}
		}
	}
	return tags
}

func deleteBaseline(resource ResourceDescriptor) map[string]any {
	if resource.OldProperties != nil {
		return resource.OldProperties
	}
	return resource.Properties
}

func assembleDiff(
	resources []ResourceDescriptor,
	plan diffPricingPlan,
	beforeCosts, afterCosts []*CostResult,
	errs []ErrorDetail,
) *DiffResult {
	before := costsByResource(plan.beforeAt, beforeCosts, len(resources))
	after := costsByResource(plan.afterAt, afterCosts, len(resources))
	result := &DiffResult{
		Entries: make([]DiffEntry, 0, len(resources)),
		Errors:  errs,
	}
	for i, resource := range resources {
		entry := buildDiffEntry(resource, plan.ops[i], before[i], after[i])
		result.Entries = append(result.Entries, entry)
		addDiffSummary(&result.Summary, entry)
	}
	result.Summary.TotalDelta = result.Summary.TotalAfter - result.Summary.TotalBefore
	result.Summary.Currency = summaryCurrency(result.Entries)
	return result
}

func costsByResource(indexes []int, costs []*CostResult, n int) []*CostResult {
	out := make([]*CostResult, n)
	for j, idx := range indexes {
		if idx < 0 || idx >= n || j >= len(costs) {
			continue
		}
		out[idx] = costs[j]
	}
	return out
}

func buildDiffEntry(resource ResourceDescriptor, op string, before, after *CostResult) DiffEntry {
	if op == DiffOperationSame && after != nil {
		copied := *after
		before = &copied
	}
	delta := monthlyOf(after) - monthlyOf(before)
	if op == DiffOperationSame {
		delta = 0
	}
	return finishDiffEntry(resource, op, before, after, delta)
}

func finishDiffEntry(
	resource ResourceDescriptor, op string, before, after *CostResult, delta float64,
) DiffEntry {
	currency := firstCurrency(after, before)
	if before == nil {
		before = zeroProjectedCost(resource, currency)
	}
	if after == nil {
		after = zeroProjectedCost(resource, currency)
	}
	return DiffEntry{
		Operation:    op,
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Before:       before,
		After:        after,
		DeltaMonthly: delta,
		Currency:     currency,
	}
}

func addDiffSummary(summary *DiffSummary, entry DiffEntry) {
	summary.TotalBefore += monthlyOf(entry.Before)
	summary.TotalAfter += monthlyOf(entry.After)
	switch entry.Operation {
	case DiffOperationCreate:
		summary.Creates++
	case DiffOperationUpdate:
		summary.Updates++
	case DiffOperationDelete:
		summary.Deletes++
	default:
		summary.Unchanged++
	}
}

func firstCurrency(costs ...*CostResult) string {
	for _, cost := range costs {
		if cost != nil && cost.Currency != "" {
			return cost.Currency
		}
	}
	return defaultCurrency
}

func summaryCurrency(entries []DiffEntry) string {
	currency := ""
	for i := range entries {
		entryCurrency := entries[i].Currency
		if entryCurrency == "" {
			continue
		}
		if currency == "" {
			currency = entryCurrency
			continue
		}
		if currency != entryCurrency {
			return ""
		}
	}
	if currency == "" {
		return defaultCurrency
	}
	return currency
}

func zeroProjectedCost(resource ResourceDescriptor, currency string) *CostResult {
	if currency == "" {
		currency = defaultCurrency
	}
	return &CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Currency:     currency,
		Adapter:      adapterNone,
	}
}

func (e *Engine) priceAligned(
	ctx context.Context, resources []ResourceDescriptor,
) ([]*CostResult, []ErrorDetail, error) {
	if len(resources) == 0 {
		return nil, nil, nil
	}
	if !uniqueResourceIDs(resources) {
		return e.priceEach(ctx, resources)
	}
	got, err := e.GetProjectedCostWithErrors(ctx, resources)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return e.priceEach(ctx, resources)
	}
	if got == nil || !resultIDsAlign(resources, got.Results) {
		return e.priceEach(ctx, resources)
	}
	return alignProjectedCosts(resources, got.Results), got.Errors, nil
}

func (e *Engine) priceEach(
	ctx context.Context, resources []ResourceDescriptor,
) ([]*CostResult, []ErrorDetail, error) {
	costs := make([]*CostResult, len(resources))
	var errs []ErrorDetail
	for i := range resources {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		one, oneErrs, err := e.priceOne(ctx, resources[i])
		if err != nil {
			return nil, nil, err
		}
		costs[i] = one
		errs = append(errs, oneErrs...)
	}
	return costs, errs, nil
}

func (e *Engine) priceOne(
	ctx context.Context, resource ResourceDescriptor,
) (*CostResult, []ErrorDetail, error) {
	got, err := e.GetProjectedCostWithErrors(ctx, []ResourceDescriptor{resource})
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, []ErrorDetail{{
			ResourceType: resource.Type,
			ResourceID:   resource.ID,
			Error:        err,
			Timestamp:    time.Now(),
		}}, nil
	}
	if got == nil || len(got.Results) == 0 {
		if got == nil {
			return nil, nil, nil
		}
		return nil, got.Errors, nil
	}
	combined := combineProjectedCosts(got.Results)
	return &combined, got.Errors, nil
}

func uniqueResourceIDs(resources []ResourceDescriptor) bool {
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if resource.ID == "" {
			return false
		}
		if _, ok := seen[resource.ID]; ok {
			return false
		}
		seen[resource.ID] = struct{}{}
	}
	return true
}

func resultIDsAlign(resources []ResourceDescriptor, results []CostResult) bool {
	ids := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		ids[resource.ID] = struct{}{}
	}
	for _, result := range results {
		if _, ok := ids[result.ResourceID]; !ok {
			return false
		}
	}
	return true
}

func alignProjectedCosts(resources []ResourceDescriptor, results []CostResult) []*CostResult {
	grouped := make(map[string][]CostResult, len(resources))
	for _, result := range results {
		grouped[result.ResourceID] = append(grouped[result.ResourceID], result)
	}
	out := make([]*CostResult, len(resources))
	for i, resource := range resources {
		group := grouped[resource.ID]
		if len(group) == 0 {
			continue
		}
		combined := combineProjectedCosts(group)
		out[i] = &combined
	}
	return out
}

func combineProjectedCosts(results []CostResult) CostResult {
	combined := results[0]
	for _, extra := range results[1:] {
		combined.Monthly += extra.Monthly
		combined.Hourly += extra.Hourly
	}
	return combined
}
