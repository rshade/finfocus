package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/logging"
)

// OutputFormat specifies the output format for cost results (table, JSON, NDJSON).
type OutputFormat string

const (
	// OutputTable renders results in a formatted table.
	OutputTable OutputFormat = "table"
	// OutputJSON renders results as pretty-printed JSON.
	OutputJSON OutputFormat = "json"
	// OutputNDJSON renders results as newline-delimited JSON for streaming.
	OutputNDJSON OutputFormat = "ndjson"
)

const (
	// defaultTabPadding is the number of spaces to use for padding in table output.
	defaultTabPadding = 2
	// maxResourceDisplayLen is the maximum length of the resource name in table output before truncation.
	maxResourceDisplayLen = 40
	// truncationEllipsis is the string to append when truncating resource names.
	truncationEllipsis = "..."
)

// RenderResults renders the given cost results using the specified output format.
// RenderResults aggregates the results for table and JSON summary outputs, emits NDJSON as individual records, and writes the output to the specified writer.
// The writer parameter specifies where the output should be written.
// The format parameter selects one of the supported OutputFormat values (OutputTable, OutputJSON, OutputNDJSON).
// The results parameter is the slice of CostResult to be rendered.
// It returns an error if rendering fails or if the provided format is unsupported.
func RenderResults(writer io.Writer, format OutputFormat, results []CostResult) error {
	return RenderResultsWithContext(context.Background(), writer, format, results)
}

// RenderResultsWithContext renders the given cost results using the specified output format with context.
// The ctx parameter enables trace ID propagation for debug logging.
func RenderResultsWithContext(ctx context.Context, writer io.Writer, format OutputFormat, results []CostResult) error {
	// Aggregate results for enhanced reporting
	aggregated := AggregateResults(results)

	switch format {
	case OutputTable:
		return renderTable(ctx, writer, aggregated)
	case OutputJSON:
		return renderJSON(writer, aggregated)
	case OutputNDJSON:
		return renderNDJSON(writer, results) // NDJSON doesn't need aggregation
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// RenderActualCostResults renders actual cost results using the specified output format.
// It dispatches to the table, pretty JSON, or NDJSON renderer based on the provided format.
// Supported formats are OutputTable, OutputJSON, and OutputNDJSON; an error is returned for unsupported formats.
// Parameters:
//   - format: the desired OutputFormat (e.g., OutputTable, OutputJSON, OutputNDJSON).
//   - results: the slice of CostResult values to render.
//
// RenderActualCostResults dispatches actual cost results to the renderer that corresponds
// to the provided output format.
//
// The format parameter selects the output format and must be one of OutputTable,
// OutputJSON, or OutputNDJSON. The results parameter is the slice of actual cost
// results to be rendered.
//
// It returns an error if the selected renderer fails or if the format is unsupported.
func RenderActualCostResults(writer io.Writer, format OutputFormat, results []CostResult, showConfidence bool) error {
	switch format {
	case OutputTable:
		return renderActualCostTable(writer, results, showConfidence)
	case OutputJSON:
		return RenderActualCostJSON(writer, results, showConfidence)
	case OutputNDJSON:
		return RenderActualCostNDJSON(writer, results, showConfidence)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// RenderActualCostJSON renders actual cost results as a JSON array with optional confidence field.
func RenderActualCostJSON(writer io.Writer, results []CostResult, showConfidence bool) error {
	// If not showing confidence, clear it from all results before encoding
	if !showConfidence {
		cleanedResults := make([]CostResult, len(results))
		for i, result := range results {
			cleanedResults[i] = result
			cleanedResults[i].Confidence = ConfidenceUnknown
		}
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(cleanedResults)
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

// RenderActualCostNDJSON renders actual cost results as NDJSON with optional confidence field.
func RenderActualCostNDJSON(writer io.Writer, results []CostResult, showConfidence bool) error {
	encoder := json.NewEncoder(writer)
	for _, result := range results {
		if !showConfidence {
			resultCopy := result
			resultCopy.Confidence = ConfidenceUnknown
			if err := encoder.Encode(resultCopy); err != nil {
				return err
			}
		} else {
			if err := encoder.Encode(result); err != nil {
				return err
			}
		}
	}
	return nil
}

// RenderCrossProviderAggregation renders cross-provider aggregation data using the specified output format.
// It supports table, JSON, and NDJSON formats and dispatches to the corresponding renderer.
//
// Parameters:
//   - format: desired output format (e.g., OutputTable, OutputJSON, OutputNDJSON).
//   - aggregations: slice of cross-provider aggregation records to render.
//   - groupBy: grouping granularity used for table output (e.g., daily or monthly).
//
// RenderCrossProviderAggregation dispatches cross-provider aggregation data to the renderer
// corresponding to the given OutputFormat.
//
// The `format` parameter selects the output renderer: table, JSON, or NDJSON.
// `aggregations` is the slice of cross-provider aggregation records to render.
// `groupBy` controls the temporal grouping used by table renderers (for example, daily vs monthly).
//
// It returns an error if the specified format is not supported or if the selected renderer fails.
func RenderCrossProviderAggregation(
	writer io.Writer,
	format OutputFormat,
	aggregations []CrossProviderAggregation,
	groupBy GroupBy,
) error {
	switch format {
	case OutputTable:
		return renderCrossProviderTable(writer, aggregations, groupBy)
	case OutputJSON:
		return renderJSONCrossProvider(writer, aggregations)
	case OutputNDJSON:
		return renderNDJSONCrossProvider(writer, aggregations)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// renderTable writes a human-readable cost table for the given aggregated results to stdout.
// It prints a cost summary (total monthly/hourly and resource count), optional breakdowns by
// provider, service, and adapter, and a detailed per-resource table with columns for Resource,
// Adapter, Monthly, Hourly, Currency, and Notes.
// The aggregated parameter provides the summary totals, breakdown maps, and the slice of per-resource results to render.
// renderTable writes a tab-separated cost summary and detailed resource table for the
// provided aggregated results to standard output. It includes total monthly/hourly
// costs, counts, optional breakdowns by provider, service, and adapter, and a per-
// resource list with monthly/hourly costs and notes.
// aggregated is the precomputed aggregation to render.
// renderTable writes the aggregated cost results to w in a human-readable tabular format.
// It creates an internal tab writer and emits the following sections in order: cost summary,
// breakdowns (by provider/service/adapter), sustainability summary, and resource details.
// ctx enables trace ID propagation for debug logging.
// writer is the destination for the rendered table. aggregated contains the precomputed
// results to render.
// Returns an error if writing to or flushing the tabulated output fails.
func renderTable(ctx context.Context, writer io.Writer, aggregated *AggregatedResults) error {
	w := tabwriter.NewWriter(writer, 0, 0, defaultTabPadding, ' ', 0)

	renderSummary(w, aggregated)
	renderBreakdowns(w, aggregated)
	renderSustainabilitySummary(ctx, w, aggregated)
	renderResourceDetails(w, aggregated)

	return w.Flush()
}

// renderSummary writes a COST SUMMARY section to w containing the total monthly cost,
// total hourly cost, and the total number of resources from aggregated, followed by a blank line.
//
// Parameters:
//   - w: destination writer for the formatted summary.
//   - aggregated: aggregated results whose Summary (TotalMonthly, TotalHourly, Currency)
//     and Resources are used to populate the output.
func renderSummary(w io.Writer, aggregated *AggregatedResults) {
	fmt.Fprintf(w, "COST SUMMARY\n")
	fmt.Fprintf(w, "============\n")
	fmt.Fprintf(w, "Total Monthly Cost:\t%.2f %s\n", aggregated.Summary.TotalMonthly, aggregated.Summary.Currency)
	fmt.Fprintf(w, "Total Hourly Cost:\t%.2f %s\n", aggregated.Summary.TotalHourly, aggregated.Summary.Currency)
	fmt.Fprintf(w, "Total Resources:\t%d\n", len(aggregated.Resources))
	fmt.Fprintf(w, "\n")
}

// renderBreakdowns writes provider, service, and adapter cost breakdown sections to w
// using the maps found in aggregated.Summary. For each non-empty breakdown it prints a
// section header followed by lines in the form "name:\t<cost> <currency>" with costs
// formatted to two decimal places and a blank line after the section. The writer w
// receives the formatted output and aggregated provides the Summary (ByProvider,
// ByService, ByAdapter and Currency) used for the breakdowns.
func renderBreakdowns(w io.Writer, aggregated *AggregatedResults) {
	// Print breakdown by provider (sorted for deterministic output - SC-003 fix)
	if len(aggregated.Summary.ByProvider) > 0 {
		fmt.Fprintf(w, "BY PROVIDER\n")
		fmt.Fprintf(w, "-----------\n")
		providers := make([]string, 0, len(aggregated.Summary.ByProvider))
		for provider := range aggregated.Summary.ByProvider {
			providers = append(providers, provider)
		}
		sort.Strings(providers)
		for _, provider := range providers {
			cost := aggregated.Summary.ByProvider[provider]
			fmt.Fprintf(w, "%s:\t%.2f %s\n", provider, cost, aggregated.Summary.Currency)
		}
		fmt.Fprintf(w, "\n")
	}

	// Print breakdown by service (sorted for deterministic output - SC-003 fix)
	if len(aggregated.Summary.ByService) > 0 {
		fmt.Fprintf(w, "BY SERVICE\n")
		fmt.Fprintf(w, "----------\n")
		services := make([]string, 0, len(aggregated.Summary.ByService))
		for service := range aggregated.Summary.ByService {
			services = append(services, service)
		}
		sort.Strings(services)
		for _, service := range services {
			cost := aggregated.Summary.ByService[service]
			fmt.Fprintf(w, "%s:\t%.2f %s\n", service, cost, aggregated.Summary.Currency)
		}
		fmt.Fprintf(w, "\n")
	}

	// Print breakdown by adapter (sorted for deterministic output - SC-003 fix)
	if len(aggregated.Summary.ByAdapter) > 0 {
		fmt.Fprintf(w, "BY ADAPTER\n")
		fmt.Fprintf(w, "----------\n")
		adapters := make([]string, 0, len(aggregated.Summary.ByAdapter))
		for adapter := range aggregated.Summary.ByAdapter {
			adapters = append(adapters, adapter)
		}
		sort.Strings(adapters)
		for _, adapter := range adapters {
			cost := aggregated.Summary.ByAdapter[adapter]
			fmt.Fprintf(w, "%s:\t%.2f %s\n", adapter, cost, aggregated.Summary.Currency)
		}
		fmt.Fprintf(w, "\n")
	}
}

// renderSustainabilitySummary aggregates sustainability metrics across all resources
// in aggregated and writes a sorted summary to w. For each metric key it writes a
// line containing the key, the summed value formatted with two decimal places, and
// the metric unit. If no sustainability metrics are present, it writes nothing.
//
// When carbon_footprint metrics are present and above the display threshold,
// it also displays real-world equivalencies (miles driven, smartphones charged)
// using EPA-published conversion factors.
//
// renderSustainabilitySummary writes a SUSTAINABILITY SUMMARY section to w showing aggregated sustainability metrics.
// It sums metrics with the same key across all resources (assuming the same unit is used for a given metric key),
// prints each metric as "key: value unit" sorted by key, and, when a carbon_footprint metric is present,
// also displays derived carbon equivalencies.
// ctx enables trace ID propagation for debug logging.
// w is the destination for formatted output.
// aggregated provides the resources whose Sustainability metrics will be aggregated and displayed.
func renderSustainabilitySummary(ctx context.Context, w io.Writer, aggregated *AggregatedResults) {
	sustainTotals := make(map[string]SustainabilityMetric)
	for _, r := range aggregated.Resources {
		for k, m := range r.Sustainability {
			total := sustainTotals[k]
			// For carbon metrics, normalize to kg before summing to handle mixed units
			if k == greenops.CarbonMetricKey || k == greenops.DeprecatedCarbonKey {
				kg, err := greenops.NormalizeToKg(m.Value, m.Unit)
				if err != nil {
					logging.FromContext(ctx).Warn().
						Ctx(ctx).
						Str("component", "engine").
						Str("operation", "render_sustainability_summary").
						Str("metric_key", k).
						Str("unit", m.Unit).
						Err(err).
						Msg("skipped resource with NormalizeToKg error")
					continue
				}
				total.Value += kg
				total.Unit = "kg"
			} else {
				total.Value += m.Value
				total.Unit = m.Unit
			}
			sustainTotals[k] = total
		}
	}

	if len(sustainTotals) > 0 {
		fmt.Fprintf(w, "SUSTAINABILITY SUMMARY\n")
		fmt.Fprintf(w, "======================\n")
		var keys []string
		for k := range sustainTotals {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			m := sustainTotals[k]
			fmt.Fprintf(w, "%s:\t%.2f %s\n", k, m.Value, m.Unit)
		}

		// Calculate and display carbon equivalencies if carbon_footprint is present
		renderCarbonEquivalencies(ctx, w, sustainTotals)

		fmt.Fprintf(w, "\n")
	}
}

// renderCarbonEquivalencies calculates and displays real-world equivalencies
// for carbon emissions using EPA conversion factors.
//
// If carbon_footprint is present and above the minimum threshold (1.0 kg),
// renderCarbonEquivalencies writes a human-readable carbon equivalencies line to w when equivalencies can be calculated from the provided sustainability totals.
// ctx enables trace ID propagation for debug logging.
// w is the destination writer. sustainTotals maps metric keys to SustainabilityMetric values used to compute equivalencies.
// If equivalencies are produced, the function writes a single indented line containing the equivalency display text and emits a debug log with the input kilograms and the number of resulting equivalencies.
func renderCarbonEquivalencies(ctx context.Context, w io.Writer, sustainTotals map[string]SustainabilityMetric) {
	// Convert engine.SustainabilityMetric to greenops.SustainabilityMetric
	greenopsMetrics := make(map[string]greenops.SustainabilityMetric, len(sustainTotals))
	for k, m := range sustainTotals {
		greenopsMetrics[k] = greenops.SustainabilityMetric{
			Value: m.Value,
			Unit:  m.Unit,
		}
	}

	// Merge deprecated key into canonical key to ensure both contribute
	if deprecated, hasDeprecated := greenopsMetrics[greenops.DeprecatedCarbonKey]; hasDeprecated {
		if canonical, hasCanonical := greenopsMetrics[greenops.CarbonMetricKey]; hasCanonical {
			// Both keys present: sum their values (both should already be in kg)
			greenopsMetrics[greenops.CarbonMetricKey] = greenops.SustainabilityMetric{
				Value: canonical.Value + deprecated.Value,
				Unit:  canonical.Unit,
			}
		} else {
			// Only deprecated key: promote to canonical
			greenopsMetrics[greenops.CarbonMetricKey] = deprecated
		}
		// Remove deprecated key to avoid double-counting in CalculateFromMap
		delete(greenopsMetrics, greenops.DeprecatedCarbonKey)
	}

	// Calculate equivalencies (pass ctx for contextual logging)
	output := greenops.CalculateFromMap(ctx, greenopsMetrics)

	// Display equivalencies if available
	if !output.IsEmpty {
		fmt.Fprintf(w, "  %s\n", output.DisplayText)
		logging.FromContext(ctx).Debug().
			Ctx(ctx).
			Float64("input_kg", output.InputKg).
			Int("result_count", len(output.Results)).
			Msg("rendered carbon equivalencies")
	}
}

// renderResourceDetails writes the "RESOURCE DETAILS" section to w, listing each aggregated resource
// with columns for Resource, Adapter, Monthly, Hourly, Currency, and Notes.
// The Resource column is formatted as "ResourceType/ResourceID" and is truncated with an ellipsis
// if it exceeds maxResourceDisplayLen. Notes include the resource's existing notes and any
// sustainability metrics.
// Parameters:
//   - w: destination writer for the rendered table.
//   - aggregated: aggregated results containing the resources to render.
func renderResourceDetails(w io.Writer, aggregated *AggregatedResults) {
	fmt.Fprintf(w, "RESOURCE DETAILS\n")
	fmt.Fprintf(w, "================\n")
	fmt.Fprintln(w, "Resource\tAdapter\tMonthly\tHourly\tCurrency\tNotes")
	fmt.Fprintln(w, "--------\t-------\t-------\t------\t--------\t-----")

	for _, result := range aggregated.Resources {
		resource := fmt.Sprintf("%s/%s", result.ResourceType, result.ResourceID)
		if len(resource) > maxResourceDisplayLen {
			resource = resource[:maxResourceDisplayLen-len(truncationEllipsis)] + truncationEllipsis
		}

		notes := formatResourceNotes(result)

		fmt.Fprintf(w, "%s\t%s\t%.2f\t%.4f\t%s\t%s\n",
			resource,
			result.Adapter,
			result.Monthly,
			result.Hourly,
			result.Currency,
			notes,
		)
	}
}

// otherwise the notes consist solely of the sustainability list.
func formatResourceNotes(result CostResult) string {
	notes := result.Notes
	if len(result.Sustainability) > 0 {
		var metrics []string
		var keys []string
		for k := range result.Sustainability {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			m := result.Sustainability[k]
			metrics = append(metrics, fmt.Sprintf("%s: %.2f %s", k, m.Value, m.Unit))
		}
		sustainStr := "[" + strings.Join(metrics, ", ") + "]"
		if notes != "" {
			notes += " " + sustainStr
		} else {
			notes = sustainStr
		}
	}
	return notes
}

// renderActualCostTable writes an actual-cost table representation of the provided
// CostResult slice to the given writer. It inspects results to determine whether
// any entries contain actual cost data (TotalCost > 0 or a non-empty CostPeriod)
// and selects an appropriate header, then renders a row for each result and
// flushes the internal tabwriter.
//
// Parameters:
//   - writer: destination for rendered table output.
//   - results: slice of CostResult values to render.
//   - showConfidence: whether to include confidence column.
//
// Returns an error if flushing the tabwriter fails.
func renderActualCostTable(writer io.Writer, results []CostResult, showConfidence bool) error {
	w := tabwriter.NewWriter(writer, 0, 0, defaultTabPadding, ' ', 0)

	// Check if we have actual cost data to determine appropriate headers
	hasActualCosts := false
	for _, result := range results {
		if result.TotalCost > 0 || result.CostPeriod != "" {
			hasActualCosts = true
			break
		}
	}

	renderActualCostHeader(w, hasActualCosts, showConfidence)

	for _, result := range results {
		renderActualCostRow(w, result, hasActualCosts, showConfidence)
	}

	return w.Flush()
}

// renderActualCostHeader writes the table header for actual-cost output to w.
// If hasActualCosts is true it writes columns for Total Cost and Period;
// otherwise it writes a header for Projected Monthly values.
// If showConfidence is true, a Confidence column is added.
func renderActualCostHeader(w io.Writer, hasActualCosts bool, showConfidence bool) {
	headers, separators := buildActualCostHeaderColumns(hasActualCosts, showConfidence)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(separators, "\t"))
}

// buildActualCostHeaderColumns returns the header labels and separator lines
// for actual cost table output based on the display options.
func buildActualCostHeaderColumns(hasActualCosts bool, showConfidence bool) ([]string, []string) {
	headers := []string{"Resource", "Adapter"}
	separators := []string{"--------", "-------"}

	if hasActualCosts {
		headers = append(headers, "Total Cost", "Period")
		separators = append(separators, "----------", "------")
	} else {
		headers = append(headers, "Projected Monthly")
		separators = append(separators, "-----------------")
	}

	if showConfidence {
		headers = append(headers, "Confidence")
		separators = append(separators, "----------")
	}

	headers = append(headers, "Currency", "Notes")
	separators = append(separators, "--------", "-----")

	return headers, separators
}

// renderActualCostRow writes a single row for a cost result into the actual-cost table.
// It formats the resource as "ResourceType/ResourceID" (truncated with an ellipsis if too long),
// appends formatted notes (including any sustainability metrics), and emits either actual-cost
// columns or projected-monthly columns depending on hasActualCosts.
//
// Parameters:
//   - w: destination writer to receive the formatted table row.
//   - result: the CostResult to render.
//   - hasActualCosts: when true, the row contains Total Cost and Period columns; when false,
//     the row contains the Projected Monthly column.
//   - showConfidence: when true, includes a Confidence column in the output.
//
// Behavior details:
//   - If hasActualCosts is true, the Total Cost column shows result.TotalCost formatted with
//     two decimals. If TotalCost is zero but result.Monthly > 0, the Total Cost column shows
//     the monthly value with " (est)" appended. The Period column uses result.CostPeriod or
//     defaults to "monthly (est)" when empty.
//   - If hasActualCosts is false, the row shows result.Monthly formatted with two decimals.
//   - The Currency and Notes columns are always emitted. Notes include existing notes and a
//     bracketed list of sustainability metrics when present.
func renderActualCostRow(w io.Writer, result CostResult, hasActualCosts bool, showConfidence bool) {
	resource := formatResourceName(result.ResourceType, result.ResourceID)
	notes := formatResourceNotes(result)
	columns := buildActualCostRowColumns(result, resource, notes, hasActualCosts, showConfidence)
	fmt.Fprintln(w, strings.Join(columns, "\t"))
}

// formatResourceName formats the resource name as "ResourceType/ResourceID",
// truncating with ellipsis if it exceeds maxResourceDisplayLen.
func formatResourceName(resourceType, resourceID string) string {
	resource := fmt.Sprintf("%s/%s", resourceType, resourceID)
	if len(resource) > maxResourceDisplayLen {
		resource = resource[:maxResourceDisplayLen-len(truncationEllipsis)] + truncationEllipsis
	}
	return resource
}

// buildActualCostRowColumns constructs the column values for an actual cost row
// based on the display options.
func buildActualCostRowColumns(
	result CostResult,
	resource, notes string,
	hasActualCosts, showConfidence bool,
) []string {
	columns := []string{resource, result.Adapter}

	if hasActualCosts {
		columns = append(columns, formatCostDisplay(result), formatPeriodDisplay(result))
	} else {
		columns = append(columns, fmt.Sprintf("%.2f", result.Monthly))
	}

	if showConfidence {
		columns = append(columns, result.Confidence.DisplayLabel())
	}

	columns = append(columns, result.Currency, notes)
	return columns
}

// formatCostDisplay formats the cost value for display,
// using estimated value if actual cost is zero but monthly is available.
func formatCostDisplay(result CostResult) string {
	if result.TotalCost == 0 && result.Monthly > 0 {
		return fmt.Sprintf("%.2f (est)", result.Monthly)
	}
	return fmt.Sprintf("%.2f", result.TotalCost)
}

// formatPeriodDisplay returns the period string for display,
// defaulting to "monthly (est)" if empty.
func formatPeriodDisplay(result CostResult) string {
	if result.CostPeriod == "" {
		return "monthly (est)"
	}
	return result.CostPeriod
}

// renderJSON writes the aggregated results as indented JSON to the provided writer.
func renderJSON(writer io.Writer, aggregated *AggregatedResults) error {
	output := map[string]interface{}{
		"finfocus": aggregated,
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// renderNDJSON writes each CostResult in results as a separate JSON object on its own line to stdout,
// renderNDJSON encodes each CostResult in results as a single JSON object per line and writes them to stdout.
// It produces newline-delimited JSON (NDJSON).
//
// The results parameter is the slice of CostResult objects to encode.
// It returns any encoding error encountered while writing.
func renderNDJSON(writer io.Writer, results []CostResult) error {
	encoder := json.NewEncoder(writer)
	for _, result := range results {
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
	return nil
}

// renderCrossProviderTable writes a cross-provider cost table to stdout.
// It formats one row per aggregation period and one column per provider, with the first
// column labeled "Date" when groupBy is GroupByDaily or "Month" otherwise.
// The function sorts provider names alphabetically to produce a consistent column order,
// prefixes monetary values with the currency symbol from each aggregation, and formats
// amounts with two decimal places.
//
// Parameters:
//   - aggregations: slice of CrossProviderAggregation where each element represents a
//     period's total and per-provider costs along with the currency code.
//   - groupBy: controls whether the period column is presented as a date (GroupByDaily)
//     or as a month.
//
// Returns an error if writing to stdout or flushing the table writer fails. If
// aggregations is empty the function prints a "No cost data available..." message and
// renderCrossProviderTable prints a tabular cross-provider cost report to stdout.
//
// aggregations is a slice of CrossProviderAggregation describing per-period totals and per-provider breakdowns.
// groupBy controls the period label used in the table header (e.g., daily uses "Date", otherwise "Month").
//
// renderCrossProviderTable writes a tabular cross-provider cost aggregation to the provided writer.
// It outputs a header (Date or Month and Total Cost), a separator row, and one row per aggregation
// containing the period label, total cost, and per-provider costs. The columns for providers are
// ordered alphabetically for deterministic output. If `aggregations` is empty, a single-line message
// "No cost data available for cross-provider aggregation" is written instead.
// Parameters:
//   - writer: destination for the formatted table output.
//   - aggregations: slice of CrossProviderAggregation values to render as rows.
//   - groupBy: controls whether the first column is labeled "Date" (GroupByDaily) or "Month".
//
// The function returns any error encountered while writing to the writer or flushing the tabwriter.
func renderCrossProviderTable(
	writer io.Writer,
	aggregations []CrossProviderAggregation,
	groupBy GroupBy,
) error {
	if len(aggregations) == 0 {
		_, err := fmt.Fprintln(writer, "No cost data available for cross-provider aggregation")
		return err
	}

	w := tabwriter.NewWriter(writer, 0, 0, defaultTabPadding, ' ', 0)

	// Collect all unique providers
	providerSet := make(map[string]bool)
	for _, agg := range aggregations {
		for provider := range agg.Providers {
			providerSet[provider] = true
		}
	}

	// Create sorted provider list
	var providers []string
	for provider := range providerSet {
		providers = append(providers, provider)
	}
	sort.Strings(providers) // Sort alphabetically for consistent ordering

	// Print header
	if groupBy == GroupByDaily {
		fmt.Fprintf(w, "Date\tTotal Cost")
	} else {
		fmt.Fprintf(w, "Month\tTotal Cost")
	}

	for _, provider := range providers {
		fmt.Fprintf(w, "\t%s", provider)
	}
	fmt.Fprintf(w, "\n")

	// Print separator
	if groupBy == GroupByDaily {
		fmt.Fprintf(w, "----\t----------")
	} else {
		fmt.Fprintf(w, "-----\t----------")
	}

	for range providers {
		fmt.Fprintf(w, "\t--------")
	}
	fmt.Fprintf(w, "\n")

	// Print data rows
	for _, agg := range aggregations {
		currencySymbol := getCurrencySymbol(agg.Currency)
		fmt.Fprintf(w, "%s\t%s%.2f", agg.Period, currencySymbol, agg.Total)
		for _, provider := range providers {
			cost := agg.Providers[provider]
			if cost > 0 {
				fmt.Fprintf(w, "\t%s%.2f", currencySymbol, cost)
			} else {
				fmt.Fprintf(w, "\t%s0.00", currencySymbol)
			}
		}
		fmt.Fprintf(w, "\n")
	}

	return w.Flush()
}

// renderJSONCrossProvider writes the provided cross-provider aggregations to stdout
// as pretty-printed (indented) JSON.
// renderJSONCrossProvider encodes the given cross-provider aggregations as indented JSON to stdout.
// The provided slice is written as a pretty-printed JSON array; any encoding error is returned.
func renderJSONCrossProvider(writer io.Writer, aggregations []CrossProviderAggregation) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(aggregations)
}

// renderNDJSONCrossProvider writes each CrossProviderAggregation in aggregations to stdout
// as newline-delimited JSON (NDJSON). It returns the first encoding error encountered, or
// renderNDJSONCrossProvider writes each CrossProviderAggregation as a separate NDJSON object to stdout.
// It returns an error if encoding any aggregation fails.
func renderNDJSONCrossProvider(writer io.Writer, aggregations []CrossProviderAggregation) error {
	encoder := json.NewEncoder(writer)
	for _, agg := range aggregations {
		if err := encoder.Encode(agg); err != nil {
			return err
		}
	}
	return nil
}

// getCurrencySymbol returns the currency symbol for the given ISO currency code.
// getCurrencySymbol returns the currency symbol for common ISO currency codes.
// It maps "USD" -> "$", "EUR" -> "€", "GBP" -> "£", "JPY" -> "¥", "CAD" -> "C$", and "AUD" -> "A$".
// For unknown or unmapped codes, it returns the original currency code.
func getCurrencySymbol(currency string) string {
	switch currency {
	case defaultCurrency: // "USD"
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	case "CAD":
		return "C$"
	case "AUD":
		return "A$"
	default:
		return currency // Fall back to currency code if symbol is unknown
	}
}
