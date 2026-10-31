// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/config"
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
			Summary: DiffSummary{Currency: defaultCurrency, StackCurrency: defaultCurrency},
			Profile: config.UsageProfileLabel(e.usageProfile),
		}, nil
	}

	resources = uniqueStateInventory(resources)
	resources = CollapseReplacements(resources)
	plan := planDiffPricing(ctx, resources)
	afterCosts, afterErrs, err := e.priceAligned(ctx, plan.after)
	if err != nil {
		return nil, err
	}
	beforeCosts, beforeErrs, err := e.priceAligned(ctx, plan.before)
	if err != nil {
		return nil, err
	}
	// A failed stack repricing keeps the plan price; the plan already reports
	// that resource's errors.
	stackCosts, _, err := e.priceAligned(ctx, plan.stackAfter)
	if err != nil {
		return nil, err
	}
	result := assembleDiff(resources, plan, beforeCosts, afterCosts, append(afterErrs, beforeErrs...))
	result.Profile = config.UsageProfileLabel(e.usageProfile)
	e.attachPricingExplanations(ctx, resources, plan, result)
	splitDiffOrigins(resources, result, costsByResource(plan.stackAfterAt, stackCosts, len(resources)))
	return result, nil
}

// uniqueStateInventory gives preview entries precedence and avoids pricing a
// state resource twice. Plan duplicates retain the existing replacement logic.
func uniqueStateInventory(resources []ResourceDescriptor) []ResourceDescriptor {
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if resource.Origin != OriginState && resource.ID != "" {
			seen[resource.ID] = struct{}{}
		}
	}
	out := make([]ResourceDescriptor, 0, len(resources))
	for _, resource := range resources {
		if resource.Origin == OriginState && resource.ID != "" {
			if _, exists := seen[resource.ID]; exists {
				continue
			}
			seen[resource.ID] = struct{}{}
		}
		out = append(out, resource)
	}
	return out
}

// splitDiffOrigins runs after pricing explanations, while their resource and
// entry indexes still describe the full inventory. Only plan entries feed the
// historical summary and AfterCosts; the stack also includes unchanged state.
// stackAfter, indexed like resources, replaces a plan entry's after cost in the
// stack total when references resolve differently against the full inventory.
func splitDiffOrigins(resources []ResourceDescriptor, result *DiffResult, stackAfter []*CostResult) {
	stackCurrency := summaryCurrency(result.Entries)
	entries := make([]DiffEntry, 0, len(result.Entries))
	stackAdjustment := 0.0
	resourceIndex := 0
	for _, entry := range result.Entries {
		// Internal Pulumi types may have no entry. Match the next retained
		// resource instead of relying on an ID map: anonymous descriptors
		// are valid and can have different origins.
		for resourceIndex < len(resources) &&
			(resources[resourceIndex].ID != entry.ResourceID ||
				resources[resourceIndex].Type != entry.ResourceType) {
			resourceIndex++
		}
		index := resourceIndex
		origin := resources[index].Origin
		resourceIndex++
		if origin == OriginState {
			result.Summary.StackComplete = true
			result.StackUnchanged = append(result.StackUnchanged, entry)
			continue
		}
		if index < len(stackAfter) && stackAfter[index] != nil {
			stackAdjustment += monthlyOf(stackAfter[index]) - monthlyOf(entry.After)
		}
		entries = append(entries, entry)
	}
	splitStackErrors(resources, result)
	complete := result.Summary.StackComplete
	result.Entries = entries
	result.Summary = DiffSummary{StackComplete: complete, StackCurrency: stackCurrency}
	for _, entry := range entries {
		addDiffSummary(&result.Summary, entry)
	}
	result.Summary.TotalDelta = result.Summary.TotalAfter - result.Summary.TotalBefore
	result.Summary.Currency = summaryCurrency(entries)
	result.Summary.StackTotal = result.Summary.TotalAfter + stackAdjustment
	result.Summary.StackResources = len(entries) + len(result.StackUnchanged)
	for _, entry := range result.StackUnchanged {
		result.Summary.StackTotal += monthlyOf(entry.After)
	}
}

// splitStackErrors moves failures of state-origin resources out of Errors. A
// type and ID shared with a plan resource stays a plan error.
func splitStackErrors(resources []ResourceDescriptor, result *DiffResult) {
	type key struct{ typ, id string }
	origins := make(map[key]DescriptorOrigin, len(resources))
	for _, resource := range resources {
		k := key{resource.Type, resource.ID}
		if existing, ok := origins[k]; !ok || existing == OriginState {
			origins[k] = resource.Origin
		}
	}
	planErrors := result.Errors[:0:0]
	for _, detail := range result.Errors {
		if origin, ok := origins[key{detail.ResourceType, detail.ResourceID}]; ok && origin == OriginState {
			result.StackErrors = append(result.StackErrors, detail)
			continue
		}
		planErrors = append(planErrors, detail)
	}
	result.Errors = planErrors
}

type diffPricingPlan struct {
	before   []ResourceDescriptor
	beforeAt []int
	after    []ResourceDescriptor
	afterAt  []int
	ops      []string
	// stackAfter reprices plan resources whose references resolve differently
	// against the full inventory; only the stack total uses those prices.
	stackAfter   []ResourceDescriptor
	stackAfterAt []int
}

func planDiffPricing(ctx context.Context, resources []ResourceDescriptor) diffPricingPlan {
	plan := diffPricingPlan{ops: make([]string, len(resources))}
	refTags, stackTags := resolveOriginDiffRefTags(ctx, resources)
	for i, resource := range resources {
		op := normalizeDiffOperation(resource.Operation)
		plan.ops[i] = op
		switch op {
		case DiffOperationDelete:
			plan.before = append(plan.before, descriptorForDiff(resource, deleteBaseline(resource), refTags[i]))
			plan.beforeAt = append(plan.beforeAt, i)
			continue
		case DiffOperationUpdate:
			plan.before = append(plan.before, descriptorForDiff(resource, resource.OldProperties, refTags[i]))
			plan.beforeAt = append(plan.beforeAt, i)
		}
		plan.after = append(plan.after, descriptorForDiff(resource, resource.Properties, refTags[i]))
		plan.afterAt = append(plan.afterAt, i)
		if stackTags != nil && !reflect.DeepEqual(refTags[i], stackTags[i]) {
			plan.stackAfter = append(plan.stackAfter, descriptorForDiff(resource, resource.Properties, stackTags[i]))
			plan.stackAfterAt = append(plan.stackAfterAt, i)
		}
	}
	return plan
}

// descriptorForDiff copies resource and swaps in props plus the origin-scoped ref
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

// resolveOriginDiffRefTags preserves the original preview's reference scope for
// plan pricing. Appended state targets must not change plan requests or cache
// keys; state-origin resources can resolve against the entire live inventory.
// stackTags, nil without state, resolves every resource against the full
// inventory, so the stack total matches a preview that lists unchanged ones.
func resolveOriginDiffRefTags(
	ctx context.Context, resources []ResourceDescriptor,
) ([]map[string]any, []map[string]any) {
	planResources := make([]ResourceDescriptor, 0, len(resources))
	planIndexes := make([]int, 0, len(resources))
	for i, resource := range resources {
		if resource.Origin != OriginState {
			planResources = append(planResources, resource)
			planIndexes = append(planIndexes, i)
		}
	}
	stackTags := resolveDiffRefTags(ctx, resources)
	if len(planResources) == len(resources) {
		return stackTags, nil
	}
	tags := slices.Clone(stackTags)
	planTags := resolveDiffRefTags(ctx, planResources)
	for i, index := range planIndexes {
		tags[index] = planTags[i]
	}
	return tags, stackTags
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
		if isUnpricedInternalType(resource.Type, before[i], after[i]) {
			continue
		}
		entry := buildDiffEntry(resource, plan.ops[i], before[i], after[i])
		result.Entries = append(result.Entries, entry)
		addDiffSummary(&result.Summary, entry)
	}
	result.Summary.TotalDelta = result.Summary.TotalAfter - result.Summary.TotalBefore
	result.Summary.Currency = summaryCurrency(result.Entries)
	return result
}

// isUnpricedInternalType reports whether a resource is a Pulumi internal type
// (the stack, a provider) that no plugin priced. The engine returns no result
// for these, so they would only show as $0 rows and inflate the counts. A type
// that a routing pattern opted in to has a result and is kept.
func isUnpricedInternalType(resourceType string, before, after *CostResult) bool {
	return strings.HasPrefix(resourceType, pulumiInternalPrefix) && before == nil && after == nil
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
	return finishDiffEntry(resource, op, before, after, monthlyOf(after)-monthlyOf(before))
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
