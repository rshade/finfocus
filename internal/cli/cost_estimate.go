package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/resourcetype"
	"github.com/rshade/finfocus/internal/spec"
	"github.com/rshade/finfocus/internal/tui"
)

// CostEstimateParams holds the parameters for the estimate cost command execution.
// Exported for testing.
type CostEstimateParams struct {
	// Single-resource mode flags
	Provider     string
	ResourceType string
	Properties   []string // key=value format
	Region       string

	// Plan-based mode flags
	PlanPath string
	Modify   []string // resource:key=value format

	// Common flags
	Interactive bool
	Output      string
	Adapter     string
}

// NewCostEstimateCmd creates the "estimate" subcommand for what-if cost analysis.
//
// The command supports two mutually exclusive modes:
//
// 1. Single-resource mode:
//   - --provider (required), --resource-type (required), --property (optional, repeatable)
//   - Estimates cost impact of property changes on a single resource
//
// 2. Plan-based mode:
//   - --pulumi-json (required), --modify (optional, repeatable)
//   - Estimates cost impact of modifications to resources in an existing Pulumi plan
//
// Common flags:
//   - --output: Output format (table, json, ndjson)
//   - --interactive: Launch interactive TUI mode
//   - --region: Region for cost calculation
//   - --adapter: Use specific plugin adapter
//
// Returns:
//   - *cobra.Command: The configured estimate subcommand
func NewCostEstimateCmd() *cobra.Command {
	var params CostEstimateParams

	cmd := &cobra.Command{
		Use:   "estimate",
		Short: "Estimate costs for what-if scenarios",
		Long: `Perform what-if cost analysis on resources without modifying Pulumi code.

Supports two modes:
  - Single-resource: Specify provider, type, and property overrides
  - Plan-based: Load a Pulumi plan and apply modifications

Examples:
  # Single resource estimation
  finfocus cost estimate --provider aws \
    --resource-type aws:ec2/instance:Instance \
    --property instanceType=m5.large

  # Plan-based estimation. The selector is a logical name, a full URN, or a type.
  finfocus cost estimate --pulumi-json plan.json \
    --modify "web-server:instanceType=m5.large"

  # Interactive mode
  finfocus cost estimate --interactive`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeCostEstimate(cmd, params)
		},
	}

	// Single-resource mode flags
	cmd.Flags().StringVar(&params.Provider, "provider", "", "Cloud provider (aws, gcp, azure)")
	cmd.Flags().StringVar(&params.ResourceType, "resource-type", "", "Resource type (e.g., aws:ec2/instance:Instance)")
	cmd.Flags().StringArrayVar(&params.Properties, "property", nil, "Property override key=value (repeatable)")
	cmd.Flags().StringVar(&params.Region, "region", "", "Region for cost calculation")

	// Plan-based mode flags
	cmd.Flags().StringVar(&params.PlanPath, "pulumi-json", "", "Path to Pulumi preview JSON file")
	cmd.Flags().StringArrayVar(
		&params.Modify, "modify", nil,
		"Resource modification selector:key=value; selector is a name, URN, or type (repeatable)",
	)

	// Common flags
	cmd.Flags().BoolVar(&params.Interactive, "interactive", false, "Launch interactive TUI mode")
	cmd.Flags().StringVar(
		&params.Output, "output", "",
		"Output format: table, json, or ndjson (default from output.default_format, else table)")
	cmd.Flags().StringVar(&params.Adapter, "adapter", "", "Specific plugin adapter to use")

	return cmd
}

// ValidateEstimateFlags validates that the estimate command flags are consistent.
// Exported for testing.
//
// Rules:
//   - Single-resource mode requires both --provider and --resource-type
//   - Plan-based mode requires --pulumi-json
//   - --property is only valid in single-resource mode
//   - --modify is only valid in plan-based mode
//   - Modes are mutually exclusive
//
// Returns an error describing the validation failure, or nil if valid.
func ValidateEstimateFlags(params *CostEstimateParams) error {
	hasSingleResource := params.Provider != "" || params.ResourceType != "" || len(params.Properties) > 0
	hasPlanBased := params.PlanPath != "" || len(params.Modify) > 0

	// Check for mutually exclusive modes
	if hasSingleResource && hasPlanBased {
		return errors.New(
			"cannot mix single-resource flags (--provider, --resource-type, --property) " +
				"with plan-based flags (--pulumi-json, --modify)")
	}

	// Validate single-resource mode requirements
	if hasSingleResource {
		if params.Provider == "" {
			return errors.New("--provider is required for single-resource estimation")
		}
		if params.ResourceType == "" {
			return errors.New("--resource-type is required for single-resource estimation")
		}
	}

	// Validate plan-based mode requirements
	if hasPlanBased && params.PlanPath == "" {
		return errors.New("--pulumi-json is required for plan-based estimation")
	}

	// If neither mode specified and not interactive, require some input
	if !hasSingleResource && !hasPlanBased && !params.Interactive {
		return errors.New(
			"either single-resource mode (--provider, --resource-type) or " +
				"plan-based mode (--pulumi-json) is required")
	}

	return nil
}

// keyValueParts is the expected number of parts when splitting key=value strings.
const keyValueParts = 2

// DoS protection limits for property parsing.
const (
	maxPropertyOverrides = 100       // Maximum number of --property flags
	maxModifications     = 1000      // Maximum number of --modify flags
	maxPropertyValueLen  = 10 * 1024 // Maximum property value length (10KB, same as engine)
	maxPropertyKeyLen    = 128       // Maximum property key length (matches engine.maxPropertyKeyLen)
)

// ParsePropertyOverrides parses --property key=value flags into a map.
// Exported for testing.
//
// Parameters:
//   - props: Slice of strings in "key=value" format
//
// Returns:
//   - map[string]string: Parsed key-value pairs
//   - error: If any property has invalid format or exceeds limits
func ParsePropertyOverrides(props []string) (map[string]string, error) {
	// DoS protection: limit number of properties
	if len(props) > maxPropertyOverrides {
		return nil, fmt.Errorf("too many property overrides: %d (max %d)", len(props), maxPropertyOverrides)
	}

	overrides := make(map[string]string, len(props))
	for _, p := range props {
		parts := strings.SplitN(p, "=", keyValueParts)
		if len(parts) != keyValueParts {
			return nil, fmt.Errorf("invalid property format %q: expected key=value", p)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("property key cannot be empty in %q", p)
		}
		// DoS protection: limit key length
		if len(key) > maxPropertyKeyLen {
			return nil, fmt.Errorf("property key too long: %d bytes (max %d)", len(key), maxPropertyKeyLen)
		}
		// DoS protection: limit value length
		if len(value) > maxPropertyValueLen {
			return nil, fmt.Errorf("property value too large for key %q: %d bytes (max %d)",
				key, len(value), maxPropertyValueLen)
		}
		overrides[key] = value
	}
	return overrides, nil
}

// ParseModifications parses --modify selector:key=value flags into a map.
// The selector may be a logical name or a Pulumi URN, so the split is the
// last colon before the first "=". Exported for testing.
//
// Parameters:
//   - mods: Slice of strings in "resource:key=value" format
//
// Returns:
//   - map[string]map[string]string: Map of resource name to property overrides
//   - error: If any modification has invalid format or exceeds limits
func ParseModifications(mods []string) (map[string]map[string]string, error) {
	// DoS protection: limit number of modifications
	if len(mods) > maxModifications {
		return nil, fmt.Errorf("too many modifications: %d (max %d)", len(mods), maxModifications)
	}

	result := make(map[string]map[string]string, len(mods))
	for _, m := range mods {
		resourceName, key, value, ok := splitModify(m)
		if !ok {
			return nil, fmt.Errorf("invalid modify format %q: expected resource:key=value", m)
		}

		if resourceName == "" {
			return nil, fmt.Errorf("resource name cannot be empty in %q", m)
		}
		if key == "" {
			return nil, fmt.Errorf("property key cannot be empty in %q", m)
		}
		// DoS protection: limit key length
		if len(key) > maxPropertyKeyLen {
			return nil, fmt.Errorf("property key too long: %d bytes (max %d)", len(key), maxPropertyKeyLen)
		}
		// DoS protection: limit value length
		if len(value) > maxPropertyValueLen {
			return nil, fmt.Errorf("property value too large for key %q: %d bytes (max %d)",
				key, len(value), maxPropertyValueLen)
		}

		if result[resourceName] == nil {
			result[resourceName] = make(map[string]string)
		}
		result[resourceName][key] = value
	}
	return result, nil
}

// splitModify separates selector:key=value. The separator is the last colon
// before the first "=", so a URN selector keeps its colons.
func splitModify(mod string) (string, string, string, bool) {
	head, value, ok := strings.Cut(mod, "=")
	if !ok {
		return "", "", "", false
	}
	selector, key, found := strings.CutLast(head, ":")
	if !found {
		return "", "", "", false
	}
	return selector, key, value, true
}

// executeCostEstimate runs the cost estimate workflow.
//
// It validates flags, determines the mode (single-resource or plan-based),
// executes the appropriate estimation, and renders results.
//
// Parameters:
//   - cmd: The Cobra command with context and output streams
//   - params: The parsed command parameters
//
// Returns an error if validation fails, estimation fails, or rendering fails.
func executeCostEstimate(cmd *cobra.Command, params CostEstimateParams) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)
	start := time.Now()

	params.Output = resolveOutputFormatWithConfig(cmd, params.Output)
	if !isValidOutputFormat(engine.OutputFormat(config.GetOutputFormat(params.Output))) {
		return fmt.Errorf("unsupported output format: %s (supported: table, json, ndjson)", params.Output)
	}
	// A TUI would take over the stream an agent or MCP client reads JSON from.
	if params.Interactive && machineOutputRequested(cmd) {
		return errors.New(
			"--interactive is not available when machine output is requested (--format json or AGENT_MODE)",
		)
	}

	// Validate flags
	if err := ValidateEstimateFlags(&params); err != nil {
		return err
	}

	log.Debug().Ctx(ctx).
		Str("operation", "cost_estimate").
		Str("provider", params.Provider).
		Str("resource_type", params.ResourceType).
		Str("plan_path", params.PlanPath).
		Bool("interactive", params.Interactive).
		Msg("starting cost estimation")

	// Load config once for all execution paths
	cfg := config.New()

	// Determine mode and execute
	var err error
	switch {
	case params.Interactive:
		err = executeInteractiveEstimate(cmd, params, cfg)
	case params.PlanPath != "":
		err = executePlanBasedEstimate(cmd, params, cfg)
	default:
		err = executeSingleResourceEstimate(cmd, params, cfg)
	}

	if err != nil {
		return err
	}

	// Log completion
	log.Info().Ctx(ctx).
		Str("operation", "cost_estimate").
		Dur("duration_ms", time.Since(start)).
		Msg("cost estimation complete")

	return nil
}

// executeSingleResourceEstimate parses property overrides, builds a resource descriptor,
// requests a cost estimate via plugins or spec fallback, and writes output.
func executeSingleResourceEstimate(cmd *cobra.Command, params CostEstimateParams, cfg *config.Config) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	// Parse property overrides
	overrides, err := ParsePropertyOverrides(params.Properties)
	if err != nil {
		return fmt.Errorf("parsing properties: %w", err)
	}
	if typeErr := validateEstimateResourceType(params.Provider, params.ResourceType); typeErr != nil {
		return toValidationError(ctx, typeErr)
	}

	log.Debug().Ctx(ctx).
		Str("provider", params.Provider).
		Str("resource_type", params.ResourceType).
		Int("override_count", len(overrides)).
		Msg("executing single-resource estimation")

	// Build resource descriptor
	resource := &engine.ResourceDescriptor{
		Provider:   params.Provider,
		Type:       params.ResourceType,
		ID:         "estimate-resource",
		Properties: map[string]any{},
	}

	// Add region if specified
	if params.Region != "" {
		resource.Properties["region"] = params.Region
	}

	// Open plugins
	clients, cleanup, err := openPlugins(ctx, params.Adapter, nil)
	// IMPORTANT: Register cleanup immediately before any error handling to prevent resource leaks
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		// Continue without plugins - fallback to spec
		log.Warn().Ctx(ctx).Err(err).Msg("failed to open plugins, using spec fallback")
		clients = nil
	}

	// Create engine and estimate
	eng := engine.New(clients, spec.NewLoader(cfg.SpecDir)).
		WithRouter(createRouterForEngine(ctx, cfg, clients))
	if profileErr := applyUsageProfile(cmd, eng, usageProfileSource(cfg)); profileErr != nil {
		return toValidationError(ctx, profileErr)
	}
	request := &engine.EstimateRequest{
		Resource:          resource,
		PropertyOverrides: overrides,
	}

	result, err := eng.EstimateCost(ctx, request)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("cost estimation failed")
		return fmt.Errorf("estimating cost: %w", err)
	}

	// Render result
	return renderEstimateResult(cmd.OutOrStdout(), params.Output, result)
}

// executePlanBasedEstimate loads resources from a Pulumi plan, applies modifications,
// estimates costs via plugins or spec fallback, and renders combined results.
func executePlanBasedEstimate(cmd *cobra.Command, params CostEstimateParams, cfg *config.Config) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	// Parse modifications
	modifications, err := ParseModifications(params.Modify)
	if err != nil {
		return fmt.Errorf("parsing modifications: %w", err)
	}

	log.Debug().Ctx(ctx).
		Str("plan_path", params.PlanPath).
		Int("modification_count", len(modifications)).
		Msg("executing plan-based estimation")

	// Load and map resources using common helper
	resources, err := loadAndMapResources(ctx, params.PlanPath, nil)
	if err != nil {
		return err
	}

	if len(resources) == 0 {
		cmd.Println("No resources found in plan")
		return nil
	}
	if missing := unmatchedModifySelectors(resources, modifications); len(missing) > 0 {
		return toValidationError(ctx, unmatchedModifyError(missing, resources))
	}

	// Open plugins
	clients, cleanup, err := openPlugins(ctx, params.Adapter, nil)
	// IMPORTANT: Register cleanup immediately before any error handling to prevent resource leaks
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		log.Warn().Ctx(ctx).Err(err).Msg("failed to open plugins, using spec fallback")
		clients = nil
	}

	// Create engine
	eng := engine.New(clients, spec.NewLoader(cfg.SpecDir)).
		WithRouter(createRouterForEngine(ctx, cfg, clients))
	if profileErr := applyUsageProfile(cmd, eng, usageProfileSource(cfg)); profileErr != nil {
		return toValidationError(ctx, profileErr)
	}

	// Process each resource with modifications
	var results []*engine.EstimateResult
	for _, resource := range resources {
		// Check if this resource has modifications
		resourceMods := findModificationsForResource(resource, modifications)

		// If no modifications for this resource, include with zero delta
		if len(resourceMods) == 0 && len(modifications) > 0 {
			continue // Skip resources without modifications when modifications are specified
		}

		engineResource := &engine.ResourceDescriptor{
			Provider:   resource.Provider,
			Type:       resource.Type,
			ID:         resource.ID,
			Properties: resource.Properties,
		}

		request := &engine.EstimateRequest{
			Resource:          engineResource,
			PropertyOverrides: resourceMods,
		}

		result, estErr := eng.EstimateCost(ctx, request)
		if estErr != nil {
			log.Warn().Ctx(ctx).
				Str("resource_id", resource.ID).
				Err(estErr).
				Msg("estimation failed for resource, continuing with others")
			continue
		}

		results = append(results, result)
	}

	// Render results
	return renderMultipleEstimateResults(cmd.OutOrStdout(), params.Output, results)
}

// selectorKeys lists the selectors that address a resource, most specific first:
// the resource ID (the Pulumi URN for plan resources), the logical name (the
// last "::" segment), then the resource type.
func selectorKeys(resource engine.ResourceDescriptor) []string {
	var keys []string
	if resource.ID != "" {
		keys = append(keys, resource.ID)
	}
	if name := logicalResourceName(resource.ID); name != "" {
		keys = append(keys, name)
	}
	if resource.Type != "" {
		keys = append(keys, resource.Type)
	}
	return keys
}

// findModificationsForResource merges the modifications of every selector that
// matches the resource. When selectors set the same property, the more specific
// selector wins: ID over logical name over type. The caller's maps are not mutated.
func findModificationsForResource(
	resource engine.ResourceDescriptor,
	mods map[string]map[string]string,
) map[string]string {
	keys := selectorKeys(resource)
	var merged map[string]string
	for _, key := range slices.Backward(keys) {
		props, ok := mods[key]
		if !ok {
			continue
		}
		if merged == nil {
			merged = make(map[string]string, len(props))
		}
		maps.Copy(merged, props)
	}
	return merged
}

func modificationSelectorMatches(selector string, resource engine.ResourceDescriptor) bool {
	return slices.Contains(selectorKeys(resource), selector)
}

func logicalResourceName(id string) string {
	if i := strings.LastIndex(id, "::"); i >= 0 && i+2 < len(id) {
		return id[i+2:]
	}
	return ""
}

func unmatchedModifySelectors(
	resources []engine.ResourceDescriptor,
	mods map[string]map[string]string,
) []string {
	if len(mods) == 0 {
		return nil
	}
	missing := make([]string, 0)
	for selector := range mods {
		matched := false
		for _, resource := range resources {
			if modificationSelectorMatches(selector, resource) {
				matched = true
				break
			}
		}
		if !matched {
			missing = append(missing, selector)
		}
	}
	sort.Strings(missing)
	return missing
}

func unmatchedModifyError(missing []string, resources []engine.ResourceDescriptor) error {
	labels := make([]string, 0, len(resources))
	for _, resource := range resources {
		name := logicalResourceName(resource.ID)
		if name == "" {
			name = resource.ID
		}
		if resource.Type != "" {
			name += " (" + resource.Type + ")"
		}
		labels = append(labels, name)
	}
	sort.Strings(labels)
	quoted := make([]string, len(missing))
	for i, selector := range missing {
		quoted[i] = fmt.Sprintf("%q", selector)
	}
	return fmt.Errorf(
		"no resources matched --modify %s; available resources: %s",
		strings.Join(quoted, ", "),
		strings.Join(labels, ", "),
	)
}

func validateEstimateResourceType(provider, resourceType string) error {
	if !resourcetype.IsFullTypeToken(resourceType) {
		return fmt.Errorf("resource type %q is not a full type token (package:module:Type)", resourceType)
	}
	got := resourcetype.ExtractProvider(resourceType)
	want := resourcetype.NormalizeProvider(provider)
	if got != want {
		return fmt.Errorf("resource type %q does not match provider %q", resourceType, provider)
	}
	return nil
}

// renderEstimateResult renders a single estimate result to the output.
func renderEstimateResult(w io.Writer, format string, result *engine.EstimateResult) error {
	switch format {
	case outputFormatJSON:
		return renderEstimateResultJSON(w, result)
	case outputFormatNDJSON:
		return renderEstimateResultNDJSON(w, result)
	default:
		return renderEstimateResultTable(w, result)
	}
}

// renderMultipleEstimateResults renders multiple estimate results.
func renderMultipleEstimateResults(w io.Writer, format string, results []*engine.EstimateResult) error {
	switch format {
	case outputFormatJSON:
		return renderMultipleEstimateResultsJSON(w, results)
	case outputFormatNDJSON:
		return renderMultipleEstimateResultsNDJSON(w, results)
	default:
		return renderMultipleEstimateResultsTable(w, results)
	}
}

// renderEstimateResultTable renders a single estimate result as a table.
func renderEstimateResultTable(w io.Writer, result *engine.EstimateResult) error {
	// Header
	fmt.Fprintln(w, "What-If Cost Analysis")
	fmt.Fprintln(w, "=====================")
	fmt.Fprintln(w)

	// Resource info
	if result.Resource != nil {
		fmt.Fprintf(w, "Resource: %s (%s)\n", result.Resource.Type, result.Resource.Provider)
		if result.Resource.ID != "" {
			fmt.Fprintf(w, "ID: %s\n", result.Resource.ID)
		}
	}
	fmt.Fprintln(w)

	// Cost summary
	baselineMonthly := 0.0
	modifiedMonthly := 0.0
	currency := "USD"

	if result.Baseline != nil {
		baselineMonthly = result.Baseline.Monthly
		if result.Baseline.Currency != "" {
			currency = result.Baseline.Currency
		}
	}
	if result.Modified != nil {
		modifiedMonthly = result.Modified.Monthly
	}

	fmt.Fprintf(w, "Baseline:  $%.2f/mo (%s)\n", baselineMonthly, currency)
	fmt.Fprintf(w, "Modified:  $%.2f/mo (%s)\n", modifiedMonthly, currency)
	fmt.Fprintln(w)

	// Total change with color indication
	changeSign := ""
	if result.TotalChange > 0 {
		changeSign = "+"
	}
	fmt.Fprintf(w, "Change:    %s$%.2f/mo\n", changeSign, result.TotalChange)
	fmt.Fprintln(w)

	// Property deltas
	if len(result.Deltas) > 0 {
		fmt.Fprintln(w, "Property Changes:")
		fmt.Fprintln(w, "-----------------")
		for _, delta := range result.Deltas {
			sign := ""
			if delta.CostChange > 0 {
				sign = "+"
			}
			fmt.Fprintf(w, "  %s: %s -> %s (%s$%.2f/mo)\n",
				delta.Property,
				delta.OriginalValue,
				delta.NewValue,
				sign,
				delta.CostChange,
			)
		}
		fmt.Fprintln(w)
	}

	// Fallback note
	if result.UsedFallback {
		fmt.Fprintln(w, "Note: Cost estimated using fallback strategy (plugin EstimateCost not available)")
	}

	return nil
}

// renderEstimateResultJSON renders a single estimate result as JSON.
func renderEstimateResultJSON(w io.Writer, result *engine.EstimateResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// renderEstimateResultNDJSON renders a single estimate result as NDJSON.
func renderEstimateResultNDJSON(w io.Writer, result *engine.EstimateResult) error {
	enc := json.NewEncoder(w)
	return enc.Encode(result)
}

// renderMultipleEstimateResultsTable renders multiple estimate results as a table.
func renderMultipleEstimateResultsTable(w io.Writer, results []*engine.EstimateResult) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No estimation results")
		return nil
	}

	fmt.Fprintln(w, "What-If Cost Analysis")
	fmt.Fprintln(w, "=====================")
	fmt.Fprintln(w)

	// Summary
	var totalBaseline, totalModified, totalChange float64
	for _, result := range results {
		if result.Baseline != nil {
			totalBaseline += result.Baseline.Monthly
		}
		if result.Modified != nil {
			totalModified += result.Modified.Monthly
		}
		totalChange += result.TotalChange
	}

	fmt.Fprintf(w, "Total Baseline:  $%.2f/mo\n", totalBaseline)
	fmt.Fprintf(w, "Total Modified:  $%.2f/mo\n", totalModified)
	changeSign := ""
	if totalChange > 0 {
		changeSign = "+"
	}
	fmt.Fprintf(w, "Total Change:    %s$%.2f/mo\n", changeSign, totalChange)
	fmt.Fprintln(w)

	// Per-resource details
	fmt.Fprintln(w, "Resource Details:")
	fmt.Fprintln(w, "-----------------")
	for _, result := range results {
		resourceName := ""
		if result.Resource != nil {
			resourceName = result.Resource.ID
			if resourceName == "" {
				resourceName = result.Resource.Type
			}
		}

		sign := ""
		if result.TotalChange > 0 {
			sign = "+"
		}
		fmt.Fprintf(w, "  %s: %s$%.2f/mo\n", resourceName, sign, result.TotalChange)

		for _, delta := range result.Deltas {
			deltaSign := ""
			if delta.CostChange > 0 {
				deltaSign = "+"
			}
			fmt.Fprintf(w, "    %s: %s -> %s (%s$%.2f)\n",
				delta.Property, delta.OriginalValue, delta.NewValue, deltaSign, delta.CostChange)
		}
	}

	return nil
}

// renderMultipleEstimateResultsJSON renders multiple estimate results as JSON.
func renderMultipleEstimateResultsJSON(w io.Writer, results []*engine.EstimateResult) error {
	// Wrap in a response object
	response := struct {
		Results     []*engine.EstimateResult `json:"results"`
		TotalChange float64                  `json:"totalChange"`
	}{
		Results: results,
	}

	for _, r := range results {
		response.TotalChange += r.TotalChange
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(response)
}

// renderMultipleEstimateResultsNDJSON renders multiple estimate results as NDJSON.
func renderMultipleEstimateResultsNDJSON(w io.Writer, results []*engine.EstimateResult) error {
	enc := json.NewEncoder(w)
	for _, result := range results {
		if err := enc.Encode(result); err != nil {
			return err
		}
	}
	return nil
}

// buildResourceFromParams creates a ResourceDescriptor from CLI parameters.
func buildResourceFromParams(provider, resourceType, region string) *engine.ResourceDescriptor {
	props := make(map[string]any)
	if region != "" {
		props["region"] = region
	}
	return &engine.ResourceDescriptor{
		Provider:   resourcetype.NormalizeProvider(provider),
		Type:       resourceType,
		ID:         "interactive-resource",
		Properties: props,
	}
}

// executeInteractiveEstimate launches and runs the interactive TUI for cost estimation.
// It obtains the command context and I/O from cmd and determines the initial resource from
// params (uses the first resource from params.PlanPath if provided, otherwise builds a
// resource from params.Provider and params.ResourceType). The function opens adapter
// plugins (falling back to spec-only mode if plugins fail), constructs an engine with a
// router, and runs a TUI that calls back into the engine to recalculate estimates as the
// user edits properties. When the TUI exits, the final estimate (if any) is printed to the
// command output using the configured output format.
//
// It returns an error when required interactive inputs are missing, when loading/parsing a
// supplied Pulumi plan fails, when running the TUI fails, or when an unexpected TUI model
// type is returned. Any error produced by rendering the final estimate is also returned.
func executeInteractiveEstimate(cmd *cobra.Command, params CostEstimateParams, cfg *config.Config) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	log.Debug().Ctx(ctx).
		Str("provider", params.Provider).
		Str("resource_type", params.ResourceType).
		Msg("launching interactive TUI")

	// Build resource from params (single-resource mode) or load from plan
	var resource *engine.ResourceDescriptor

	switch {
	case params.PlanPath != "":
		// Load resources from plan
		resources, err := loadAndMapResources(ctx, params.PlanPath, nil)
		if err != nil {
			return err
		}
		if len(resources) == 0 {
			cmd.Println("No resources found in plan")
			return nil
		}
		// Use the first resource for interactive mode
		first := resources[0]
		resource = &engine.ResourceDescriptor{
			Provider:   first.Provider,
			Type:       first.Type,
			ID:         first.ID,
			Properties: first.Properties,
		}
	case params.Provider != "" && params.ResourceType != "":
		resource = buildResourceFromParams(params.Provider, params.ResourceType, params.Region)
	default:
		return errors.New("interactive mode requires --provider and --resource-type, or --pulumi-json")
	}

	// Create engine
	clients, cleanup, err := openPlugins(ctx, params.Adapter, nil)
	// IMPORTANT: Register cleanup immediately before any error handling to prevent resource leaks
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		log.Warn().Ctx(ctx).Err(err).Msg("failed to open plugins, using spec fallback")
		clients = nil
	}

	eng := engine.New(clients, spec.NewLoader(cfg.SpecDir)).
		WithRouter(createRouterForEngine(ctx, cfg, clients))
	if profileErr := applyUsageProfile(cmd, eng, usageProfileSource(cfg)); profileErr != nil {
		return toValidationError(ctx, profileErr)
	}

	model := newInteractiveEstimateModel(ctx, eng, resource)
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("running interactive TUI: %w", err)
	}

	// After TUI exits, print final result if available
	estModel, ok := finalModel.(*tui.EstimateModel)
	if !ok {
		// This should not happen unless the TUI library changes
		return fmt.Errorf("unexpected model type: %T, expected *tui.EstimateModel", finalModel)
	}

	result := estModel.GetResult()
	if result != nil && (result.Baseline != nil || result.Modified != nil) {
		cmd.Println("\nFinal Estimate:")
		return renderEstimateResult(cmd.OutOrStdout(), params.Output, result)
	}

	return nil
}

// newInteractiveEstimateModel binds the shared engine's initial baseline,
// pricing discovery, edited requests, and reverted-property baseline path.
func newInteractiveEstimateModel(
	ctx context.Context,
	eng *engine.Engine,
	resource *engine.ResourceDescriptor,
) *tui.EstimateModel {
	log := logging.FromContext(ctx)
	// Get the unchanged baseline through the same RPC path used for edits.
	initialResult, err := eng.EstimateBaseline(ctx, resource, "")
	if err != nil {
		log.Warn().Ctx(ctx).Str("resource_id", resource.ID).Err(err).Msg("failed to get initial estimate")
	}
	return tui.NewEstimateModel(ctx, resource, initialResult).
		WithPricingDiscovery(eng.DiscoverPricingSpec).
		WithEstimateCallback(func(recalcCtx context.Context, request *engine.EstimateRequest) (*engine.EstimateResult, error) {
			if len(request.PropertyOverrides) == 0 {
				return eng.EstimateBaseline(recalcCtx, request.Resource, request.PricingMode)
			}
			return eng.EstimateCost(recalcCtx, request)
		})
}
