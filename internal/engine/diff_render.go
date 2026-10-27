// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
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
func RenderProjectedDiff(
	w io.Writer,
	format OutputFormat,
	diff *DiffResult,
	showBreakdown bool,
) error {
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

func displayedProfile(profile string) string {
	if profile == "" {
		return "default"
	}
	return profile
}

// NoChangesStackSummary is the explanation for a plan with nothing to price
// when a complete stack total exists. ok is false in every other case.
func NoChangesStackSummary(diff *DiffResult) (string, bool) {
	if diff == nil || !diff.IncludeStack || !diff.EmptyPlan || !diff.Summary.StackComplete {
		return "", false
	}
	return fmt.Sprintf(
		"No changes. Projected monthly cost is zero.\nStack total: %s across %d unchanged resources.\n",
		formatDiffMonthly(diff.Summary.StackTotal, diff.Summary.StackCurrency),
		diff.Summary.StackResources,
	), true
}

func renderDiffTable(w io.Writer, diff *DiffResult, showBreakdown bool) error {
	if summary, ok := NoChangesStackSummary(diff); ok {
		if _, err := io.WriteString(w, summary); err != nil {
			return err
		}
		return writeDiffErrors(w, displayedDiffErrors(diff))
	}
	tw := tabwriter.NewWriter(w, 0, 0, defaultTabPadding, ' ', 0)
	fmt.Fprintf(tw, "Profile: %s\n", displayedProfile(diff.Profile))
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
	if diff.IncludeStack && diff.Summary.StackComplete {
		if len(diff.StackUnchanged) > 0 {
			fmt.Fprintln(tw, "\nUNCHANGED RESOURCES (not in the plan total)")
			fmt.Fprintln(tw, "OP\tRESOURCE\tBEFORE\tAFTER\tCHANGE\tCURRENCY")
			for i := range diff.StackUnchanged {
				writeDiffRow(tw, &diff.StackUnchanged[i], showBreakdown)
			}
		}
		fmt.Fprintf(
			tw,
			"Stack total: %s across %d resources (plan total %s)\n",
			formatDiffMonthly(
				diff.Summary.StackTotal,
				diff.Summary.StackCurrency,
			),
			diff.Summary.StackResources,
			formatDiffMonthly(diff.Summary.TotalAfter, diff.Summary.Currency),
		)
	}
	renderSustainabilitySummary(context.Background(), tw, AggregateResults(diff.AfterCosts()))
	if err := tw.Flush(); err != nil {
		return err
	}
	return writeDiffErrors(w, displayedDiffErrors(diff))
}

// displayedDiffErrors adds state-origin failures only when the stack section is shown.
func displayedDiffErrors(diff *DiffResult) []ErrorDetail {
	if !diff.IncludeStack || !diff.Summary.StackComplete || len(diff.StackErrors) == 0 {
		return diff.Errors
	}
	return append(append([]ErrorDetail{}, diff.Errors...), diff.StackErrors...)
}

func formatDiffMonthly(amount float64, currency string) string {
	if currency == defaultCurrency {
		return fmt.Sprintf("$%.2f/mo", amount)
	}
	if currency == "" {
		return fmt.Sprintf("%.2f (mixed currencies)/mo", amount)
	}
	return fmt.Sprintf("%.2f %s/mo", amount, currency)
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

type projectedStackDocument struct {
	TotalMonthly     float64         `json:"totalMonthly"`
	Currency         string          `json:"currency"`
	Resources        int             `json:"resources"`
	Complete         bool            `json:"complete"`
	IncompleteReason string          `json:"incompleteReason,omitempty"`
	OmittedUnchanged int             `json:"omittedUnchanged,omitempty"`
	Unchanged        []CostResult    `json:"unchanged,omitempty"`
	Errors           []diffErrorJSON `json:"errors,omitempty"`
}

type projectedDiffDocument struct {
	Stack     *projectedStackDocument `json:"stack,omitempty"`
	Summary   CostSummary             `json:"summary"`
	Resources []projectedDiffResource `json:"resources"`
	Diff      DiffSummary             `json:"diff"`
	EmptyPlan bool                    `json:"emptyPlan,omitempty"`
	Profile   string                  `json:"profile"`
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
		EmptyPlan: diff.EmptyPlan,
		Profile:   displayedProfile(diff.Profile),
		Errors:    diffErrorsJSON(diff.Errors),
	}
	if diff.IncludeStack {
		doc.Stack = &projectedStackDocument{
			TotalMonthly: diff.Summary.StackTotal, Currency: diff.Summary.StackCurrency,
			Resources: diff.Summary.StackResources, Complete: diff.Summary.StackComplete,
			IncompleteReason: diff.Summary.StackIncompleteReason,
			OmittedUnchanged: diff.Summary.OmittedUnchanged,
			Errors:           diffErrorsJSON(diff.StackErrors),
		}
		if diff.Summary.StackComplete {
			doc.Stack.IncompleteReason = ""
			for _, entry := range diff.StackUnchanged {
				if entry.After != nil {
					doc.Stack.Unchanged = append(doc.Stack.Unchanged, *entry.After)
				}
			}
		}
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
