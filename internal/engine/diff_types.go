// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

// Pulumi plan operations that cost diff knows how to price.
const (
	// DiffOperationCreate is a new resource, including an empty operation.
	DiffOperationCreate = "create"
	// DiffOperationUpdate is an in-place update or a replacement.
	DiffOperationUpdate = "update"
	// DiffOperationDelete is a resource leaving the stack.
	DiffOperationDelete = "delete"
	// DiffOperationSame is a resource whose price is unchanged.
	DiffOperationSame = "same"
)

// DiffEntry is the before/after projected cost of one resource.
type DiffEntry struct {
	Operation    string           `json:"operation"`
	ResourceType string           `json:"resourceType"`
	ResourceID   string           `json:"resourceId"`
	Before       *CostResult      `json:"before,omitempty"`
	After        *CostResult      `json:"after,omitempty"`
	DeltaMonthly float64          `json:"deltaMonthly"`
	Currency     string           `json:"currency,omitempty"`
	PricingSpec  *PricingSpecView `json:"pricing_spec,omitempty"`
}

// DiffSummary is the plan-level before, after, and delta totals.
type DiffSummary struct {
	TotalBefore float64 `json:"totalBefore"`
	TotalAfter  float64 `json:"totalAfter"`
	TotalDelta  float64 `json:"totalDelta"`
	Currency    string  `json:"currency,omitempty"`
	Creates     int     `json:"creates"`
	Updates     int     `json:"updates"`
	Deletes     int     `json:"deletes"`
	Unchanged   int     `json:"unchanged"`
}

// DiffResult is the projected cost diff for a set of resources.
type DiffResult struct {
	Entries []DiffEntry
	Summary DiffSummary
	Errors  []ErrorDetail
}

// AfterCosts returns the after-change cost of every entry.
// Delete entries contribute a zero cost so the sum matches TotalAfter.
func (d *DiffResult) AfterCosts() []CostResult {
	if d == nil {
		return nil
	}
	out := make([]CostResult, 0, len(d.Entries))
	for i := range d.Entries {
		if d.Entries[i].After != nil {
			out = append(out, *d.Entries[i].After)
		}
	}
	return out
}

func normalizeDiffOperation(op string) string {
	switch op {
	case "", DiffOperationCreate:
		return DiffOperationCreate
	case DiffOperationUpdate, "replace", "create-replacement":
		return DiffOperationUpdate
	case DiffOperationDelete, "delete-replaced":
		return DiffOperationDelete
	case DiffOperationSame:
		return DiffOperationSame
	default:
		return DiffOperationCreate
	}
}

func monthlyOf(cost *CostResult) float64 {
	if cost == nil {
		return 0
	}
	return cost.Monthly
}
