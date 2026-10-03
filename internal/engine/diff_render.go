// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"
)

// RenderProjectedDiff writes a projected cost diff as a table, JSON, or NDJSON.
// JSON keeps finfocus.summary.totalMonthly as the after total and adds
// finfocus.diff. NDJSON writes one DiffEntry per line. showBreakdown adds
// component sub-rows under the after cost and is ignored for JSON and NDJSON.
func RenderProjectedDiff(w io.Writer, format OutputFormat, diff *DiffResult, showBreakdown bool) error {
	if diff == nil {
		return fmt.Errorf("rendering projected cost diff: %w", ErrEmptyResults)
	}
	switch format {
	case OutputTable:
		return renderDiffTable(w, diff, showBreakdown)
	case OutputJSON:
		return renderDiffJSON(w, diff)
	case OutputNDJSON:
		return renderDiffNDJSON(w, diff.Entries)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderDiffTable(w io.Writer, diff *DiffResult, showBreakdown bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, defaultTabPadding, ' ', 0)
	fmt.Fprintln(tw, "COST DIFF")
	fmt.Fprintf(tw, "Before\t%.2f %s\n", diff.Summary.TotalBefore, diff.Summary.Currency)
	fmt.Fprintf(tw, "After\t%.2f %s\n", diff.Summary.TotalAfter, diff.Summary.Currency)
	fmt.Fprintf(tw, "Change\t%+.2f %s\n", diff.Summary.TotalDelta, diff.Summary.Currency)
	fmt.Fprintf(tw, "Resources\t%d (%d create, %d update, %d delete, %d unchanged)\n",
		len(diff.Entries), diff.Summary.Creates, diff.Summary.Updates,
		diff.Summary.Deletes, diff.Summary.Unchanged)
	fmt.Fprintln(tw)
	fmt.Fprintln(tw, "OP\tRESOURCE\tBEFORE\tAFTER\tCHANGE\tCURRENCY")
	for i := range diff.Entries {
		writeDiffRow(tw, &diff.Entries[i], showBreakdown)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	return writeDiffErrors(w, diff.Errors)
}

func writeDiffRow(w io.Writer, entry *DiffEntry, showBreakdown bool) {
	fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t%+.2f\t%s\n",
		diffOpSymbol(entry.Operation),
		formatResourceName(entry.ResourceType, entry.ResourceID),
		monthlyOf(entry.Before),
		monthlyOf(entry.After),
		entry.DeltaMonthly,
		entry.Currency,
	)
	if note := diffRowNote(entry); note != "" {
		fmt.Fprintf(w, "\t%s\n", note)
	}
	writePricingExplanation(w, entry.PricingSpec)
	if showBreakdown && entry.After != nil {
		writeBreakdownSubrows(w, entry.After.Breakdown, diffTableColumns, diffAfterColumn)
	}
}

// diffRowNote keeps plugin notes, including Supports declines, visible in the
// diff table. The after cost is the current price; a delete's note lives on
// the before cost because its after cost is zero.
func diffRowNote(entry *DiffEntry) string {
	if entry.After != nil {
		if note := formatResourceNotes(*entry.After); note != "" {
			return note
		}
	}
	if entry.Before != nil {
		return formatResourceNotes(*entry.Before)
	}
	return ""
}

const (
	diffTableColumns = 6
	diffAfterColumn  = 3
)

func diffOpSymbol(op string) string {
	switch op {
	case DiffOperationCreate:
		return "+"
	case DiffOperationUpdate:
		return "~"
	case DiffOperationDelete:
		return "-"
	default:
		return "="
	}
}

func writeDiffErrors(w io.Writer, errs []ErrorDetail) error {
	if len(errs) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "ERRORS"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "======"); err != nil {
		return err
	}
	for _, detail := range errs {
		msg := ""
		if detail.Error != nil {
			msg = detail.Error.Error()
		}
		if _, err := fmt.Fprintf(w, "%s %s %s: %s\n",
			detail.ResourceType, detail.ResourceID, detail.PluginName, msg); err != nil {
			return err
		}
	}
	return nil
}

type projectedDiffResource struct {
	CostResult

	Operation     string           `json:"operation"`
	BeforeMonthly float64          `json:"beforeMonthly"`
	DeltaMonthly  float64          `json:"deltaMonthly"`
	PricingSpec   *PricingSpecView `json:"pricing_spec,omitempty"`
}

type projectedDiffDocument struct {
	Summary   CostSummary             `json:"summary"`
	Resources []projectedDiffResource `json:"resources"`
	Diff      DiffSummary             `json:"diff"`
	Errors    []diffErrorJSON         `json:"errors,omitempty"`
}

type diffErrorJSON struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	PluginName   string `json:"pluginName,omitempty"`
	Message      string `json:"message"`
}

func renderDiffJSON(w io.Writer, diff *DiffResult) error {
	after := diff.AfterCosts()
	aggregated := AggregateResults(after)
	aggregated.Summary.TotalMonthly = diff.Summary.TotalAfter
	doc := projectedDiffDocument{
		Summary:   aggregated.Summary,
		Resources: make([]projectedDiffResource, 0, len(diff.Entries)),
		Diff:      diff.Summary,
		Errors:    diffErrorsJSON(diff.Errors),
	}
	for i := range diff.Entries {
		entry := &diff.Entries[i]
		resource := projectedDiffResource{
			Operation:     entry.Operation,
			BeforeMonthly: monthlyOf(entry.Before),
			DeltaMonthly:  entry.DeltaMonthly,
		}
		if entry.After != nil {
			resource.CostResult = *entry.After
		}
		resource.PricingSpec = entry.PricingSpec
		doc.Resources = append(doc.Resources, resource)
	}
	payload := map[string]any{"finfocus": doc}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func diffErrorsJSON(errs []ErrorDetail) []diffErrorJSON {
	if len(errs) == 0 {
		return nil
	}
	out := make([]diffErrorJSON, 0, len(errs))
	for _, detail := range errs {
		msg := ""
		if detail.Error != nil {
			msg = detail.Error.Error()
		}
		out = append(out, diffErrorJSON{
			ResourceType: detail.ResourceType,
			ResourceID:   detail.ResourceID,
			PluginName:   detail.PluginName,
			Message:      msg,
		})
	}
	return out
}

func renderDiffNDJSON(w io.Writer, entries []DiffEntry) error {
	encoder := json.NewEncoder(w)
	for i := range entries {
		if err := encoder.Encode(entries[i]); err != nil {
			return err
		}
	}
	return nil
}

func writePricingExplanation(w io.Writer, spec *PricingSpecView) {
	if spec == nil {
		return
	}
	if spec.BillingMode != "" {
		fmt.Fprintf(w, "\tBilling Mode: %s\n", spec.BillingMode)
	}
	if spec.Unit != "" {
		fmt.Fprintf(w, "\tUnit: %s\n", spec.Unit)
	}
	fmt.Fprintf(w, "\tRate: %s\n", formatExplainRate(spec.RatePerUnit, spec.Unit))
	if spec.Source != "" {
		fmt.Fprintf(w, "\tSource: %s\n", spec.Source)
	}
	for _, assumption := range spec.Assumptions {
		fmt.Fprintf(w, "\t- %s\n", assumption)
	}
	if len(spec.PricingTiers) == 0 {
		fmt.Fprintf(w, "\tPricing Tiers: (none)\n")
		return
	}
	for _, tier := range spec.PricingTiers {
		fmt.Fprintf(w, "\tPricing Tier: %s from %.0f to %.0f\n",
			formatExplainRate(tier.RatePerUnit, spec.Unit), tier.MinQuantity, tier.MaxQuantity)
	}
}

func formatExplainRate(rate float64, unit string) string {
	text := "$" + strconv.FormatFloat(rate, 'f', -1, 64)
	if unit == "" {
		return text
	}
	return text + "/" + unit
}
