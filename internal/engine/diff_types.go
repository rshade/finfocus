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
	// StackTotal includes the plan and unchanged state inventory, each resource once.
	StackTotal float64 `json:"-"`
	// StackCurrency is the common currency of plan and state entries, or empty
	// when their currencies differ. Currency remains the plan-only currency.
	StackCurrency string `json:"-"`
	// StackResources counts the entries included in StackTotal.
	StackResources int `json:"-"`
	// StackComplete reports whether stack state was available. Callers supply
	// the authoritative load status when no state-origin entries were appended.
	StackComplete bool `json:"-"`
	// StackIncompleteReason is a sanitized state availability category.
	StackIncompleteReason string `json:"-"`
	// OmittedUnchanged counts unchanged resources omitted by an incomplete preview.
	OmittedUnchanged int     `json:"-"`
	TotalBefore      float64 `json:"totalBefore"`
	TotalAfter       float64 `json:"totalAfter"`
	TotalDelta       float64 `json:"totalDelta"`
	Currency         string  `json:"currency,omitempty"`
	Creates          int     `json:"creates"`
	Updates          int     `json:"updates"`
	Deletes          int     `json:"deletes"`
	Unchanged        int     `json:"unchanged"`
}

// DiffResult is the projected cost diff for a set of resources.
type DiffResult struct {
	// IncludeStack enables the Pulumi stack metadata object in projected output.
	IncludeStack bool
	// StackUnchanged holds state-origin entries excluded from plan totals.
	StackUnchanged []DiffEntry
	Entries        []DiffEntry
	Summary        DiffSummary
	Errors         []ErrorDetail
	// StackErrors holds pricing failures of state-origin resources. Errors keeps
	// plan resources only, as in v0.5.2; the stack output reports these.
	StackErrors []ErrorDetail
	// Profile is the resolved usage profile for this diff. "default" is no preference.
	Profile string
	// EmptyPlan is set by the caller when the plan held nothing to price, so a
	// $0 total is not mistaken for an estimate.
	EmptyPlan bool
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
