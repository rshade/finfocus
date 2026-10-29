package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/forecast"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/resourcetype"
	"github.com/rshade/finfocus/internal/spec"
)

const (
	forecastOutputPlain  = "plain"
	forecastOutputJSON   = "json"
	forecastOutputNDJSON = "ndjson"
)

// forecastDeps replaces the plugin price and the history read in tests.
type forecastDeps struct {
	now         time.Time
	width       int
	cfg         *config.Config
	skipPrice   bool
	costs       []engine.CostResult
	costErr     error
	skipHistory bool
	history     []history.CostSnapshot
	historyErr  error
}

// NewCostForecastCmd returns `cost forecast`.
// It prices the same inputs as `cost projected` and projects each resource
// with the plugin growth model. The series is timestamped so a later
// interactive history chart can plot it without a second projection.
func NewCostForecastCmd() *cobra.Command {
	return newCostForecastCmd(forecastDeps{})
}

func newCostForecastCmd(deps forecastDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forecast",
		Short: "Project future monthly spend from current cost and plugin growth",
		Long: `Project the after-change monthly cost forward by month.

The current price comes from the same plan or state as cost projected.
A Terraform directory runs OpenTofu plan and show without init.
--terraform-plan reads saved show -json output; --terraform-state keeps reading
state without planning. Infrastructure input flags are mutually exclusive.
--include-workspace overrides cost.terraform.include_workspace for this run's
Terraform history identity. Pulumi ignores the setting.

Each plugin reports a growth model (none, linear, or exponential). Pass
--growth-rate to apply that model. The monthly price itself does not change.

Plain output is an ASCII chart. JSON and NDJSON emit timestamped series,
including stored cost history for --stack when that database exists and
uses the same currency. A missing history file does not fail the command.`,
		Example: `  finfocus cost forecast --pulumi-json plan.json --growth-type linear --growth-rate 0.10
  finfocus cost forecast --stack prod --months 6 --growth-rate 0.05 --split-providers
  finfocus cost forecast --pulumi-json plan.json --growth-rate 0.05 --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForecast(cmd, deps)
		},
	}
	cmd.Flags().
		String("pulumi-json", "", "Path to Pulumi preview JSON output (optional; auto-detected from Pulumi project if omitted)")
	cmd.Flags().
		String("terraform-state", "", "Path to a Terraform state file (terraform.tfstate); mutually exclusive with --pulumi-json")
	cmd.Flags().
		String("terraform-plan", "", "Path to tofu show -json output; does not run tofu")
	cmd.Flags().String("pulumi-state", "", "Path to Pulumi stack export JSON")
	addIncludeWorkspaceFlag(cmd)
	cmd.Flags().String("spec-dir", "", "Directory containing pricing spec files")
	cmd.Flags().String("adapter", "", "Use only the specified adapter plugin")
	cmd.Flags().
		StringArray("filter", nil, "Resource filter expressions (e.g., 'type=aws:ec2/instance')")
	cmd.Flags().IntP("jobs", "j", 0, "Number of parallel workers (0 = auto based on CPU count)")
	cmd.Flags().
		Int("months", forecast.DefaultMonths, "Months ahead to project, from 1 to 36, including the current month as the start")
	cmd.Flags().String("growth-type", "", "Override every resource: none, linear, or exponential")
	cmd.Flags().
		String("growth-rate", "", "Growth per month as a decimal (0.10 is 10%). Required with linear or exponential")
	cmd.Flags().Bool("split-providers", false, "Draw one series per provider on the chart")
	cmd.Flags().Bool("no-budget", false, "Hide the global budget line")
	cmd.Flags().Bool("no-history", false, "Do not read cost history for --stack")
	cmd.Flags().Int("height", defaultChartHeight, "Chart height in rows")
	cmd.Flags().Int("width", 0, "Chart width in columns (0 = terminal width)")
	cmd.Flags().String("output", forecastOutputPlain, "Output format: plain, json, or ndjson")
	cmd.Flags().Bool("plain", false, "Force the ASCII chart")
	cmd.Flags().Bool("no-color", false, "Disable ANSI colors on the chart")
	return cmd
}

func runForecast(cmd *cobra.Command, deps forecastDeps) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	format, err := forecastOutputFormat(cmd)
	if err != nil {
		return err
	}
	settings, err := forecastFlagSettings(cmd, deps)
	if err != nil {
		return err
	}

	audit := newAuditContext(ctx, "cost forecast", map[string]string{auditKeyOutput: format})
	costs, err := forecastCosts(ctx, cmd, deps, audit)
	if err != nil {
		audit.logFailure(ctx, err)
		return err
	}
	if len(costs) == 0 && terraformProjectionSelected(cmd) {
		return projectedUnchanged(ctx, cmd, audit)
	}
	built, err := buildForecast(costs, settings, func() ([]history.CostSnapshot, error) {
		return forecastHistorySnapshots(cmd, deps)
	})
	if err != nil {
		audit.logFailure(ctx, err)
		return err
	}
	if err = writeForecast(cmd, format, built, deps); err != nil {
		audit.logFailure(ctx, err)
		return err
	}
	total := 0.0
	if n := len(built.projection.Forecast.Points); n > 0 {
		total = built.projection.Forecast.Points[n-1].Value
	}
	audit.logSuccess(ctx, len(costs), total)
	return nil
}

// forecastFlagSettings reads the forecast flags in the order the command has
// always validated them: growth, then months.
func forecastFlagSettings(cmd *cobra.Command, deps forecastDeps) (forecastSettings, error) {
	kind, err := cmd.Flags().GetString("growth-type")
	if err != nil {
		return forecastSettings{}, err
	}
	rawRate, err := cmd.Flags().GetString("growth-rate")
	if err != nil {
		return forecastSettings{}, err
	}
	growthType, rate, err := forecast.ParseGrowth(kind, rawRate)
	if err != nil {
		return forecastSettings{}, err
	}
	months, err := cmd.Flags().GetInt("months")
	if err != nil {
		return forecastSettings{}, err
	}
	if err = forecast.ValidateMonths(months); err != nil {
		return forecastSettings{}, err
	}
	noHistory, err := cmd.Flags().GetBool("no-history")
	if err != nil {
		return forecastSettings{}, err
	}
	noBudget, err := cmd.Flags().GetBool("no-budget")
	if err != nil {
		return forecastSettings{}, err
	}
	return forecastSettings{
		months:     months,
		growthType: growthType,
		rate:       rate,
		noHistory:  noHistory,
		noBudget:   noBudget,
		now:        deps.now,
		cfg:        deps.cfg,
	}, nil
}

func forecastOutputFormat(cmd *cobra.Command) (string, error) {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	format := resolveOutputFormat(cmd, "output", output)
	plain, err := cmd.Flags().GetBool("plain")
	if err != nil {
		return "", err
	}
	if plain && cmd.Flags().Changed("plain") {
		format = forecastOutputPlain
	}
	switch format {
	case forecastOutputPlain, forecastOutputJSON, forecastOutputNDJSON:
		return format, nil
	default:
		return "", fmt.Errorf(
			"unsupported output format: %s (supported: plain, json, ndjson)",
			format,
		)
	}
}

func forecastCosts(
	ctx context.Context,
	cmd *cobra.Command,
	deps forecastDeps,
	audit *auditContext,
) ([]engine.CostResult, error) {
	if deps.skipPrice {
		return deps.costs, deps.costErr
	}
	plan, err := cmd.Flags().GetString("pulumi-json")
	if err != nil {
		return nil, err
	}
	tfState, err := cmd.Flags().GetString("terraform-state")
	if err != nil {
		return nil, err
	}
	specDir, err := cmd.Flags().GetString("spec-dir")
	if err != nil {
		return nil, err
	}
	adapter, err := cmd.Flags().GetString("adapter")
	if err != nil {
		return nil, err
	}
	filter, err := cmd.Flags().GetStringArray("filter")
	if err != nil {
		return nil, err
	}
	jobs, err := cmd.Flags().GetInt("jobs")
	if err != nil {
		return nil, err
	}
	tfPlan, _ := cmd.Flags().GetString("terraform-plan")
	pulumiState, _ := cmd.Flags().GetString("pulumi-state")
	params := costProjectedParams{
		terraformPlan:  tfPlan,
		pulumiState:    pulumiState,
		pulumiJSONPath: plan,
		terraformState: tfState,
		specDir:        specDir,
		adapter:        adapter,
		filter:         filter,
		jobs:           jobs,
	}
	if err = validateCostProjectedParams(params); err != nil {
		return nil, toValidationError(ctx, err)
	}
	resources, unchanged, err := loadProjectedResources(ctx, cmd, params, audit)
	if err != nil {
		if errors.Is(err, ErrNoProjectMarker) || params.terraformState != "" || params.pulumiJSONPath != "" ||
			params.terraformPlan != "" ||
			params.pulumiState != "" {
			return nil, toValidationError(ctx, err)
		}
		return nil, err
	}
	if unchanged {
		return nil, nil
	}
	resources, err = ApplyFilters(ctx, planOriginResources(resources), params.filter)
	if err != nil {
		return nil, toValidationError(ctx, fmt.Errorf("applying filters: %w", err))
	}
	cfg, specDir := forecastPricingConfig(specDir)
	clients, cleanup, err := openPlugins(ctx, adapter, audit)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	eng, cacheStore, cacheCleanup := newEngineWithCache(
		ctx,
		cmd,
		clients,
		spec.NewLoader(specDir),
		cfg,
	)
	defer cacheCleanup()
	if profileErr := applyUsageProfile(cmd, eng, cfg); profileErr != nil {
		return nil, toValidationError(ctx, profileErr)
	}
	eng = eng.WithJobs(jobs)
	resources = prepareForecastResources(
		ctx,
		cmd,
		params.terraformState,
		clients,
		cacheStore,
		resources,
	)
	diff, err := eng.GetProjectedCostDiff(ctx, resources)
	if err != nil {
		return nil, fmt.Errorf("calculating projected costs: %w", err)
	}
	return diff.AfterCosts(), nil
}

// prepareForecastResources matches cost projected: warn when no plugin can
// resolve Terraform types, then resolve them before pricing. Pulumi tokens
// already contain a colon and are left unchanged.
func prepareForecastResources(
	ctx context.Context,
	cmd *cobra.Command,
	terraformState string,
	clients []*pluginhost.Client,
	store cache.Cache,
	resources []engine.ResourceDescriptor,
) []engine.ResourceDescriptor {
	warnNoTypeResolvingPlugin(cmd, terraformState, clients)
	return resolveResourceTypes(ctx, clients, store, resources)
}

func forecastResources(
	costs []engine.CostResult,
	growthType string,
	rate *float64,
) ([]forecast.Resource, []string) {
	resources := make([]forecast.Resource, 0, len(costs))
	var warnings []string
	applied := rate
	if growthType == forecast.GrowthNone {
		applied = nil
	}
	for _, cost := range costs {
		if reason, skip := forecastSkipReason(cost); skip {
			warnings = append(warnings, fmt.Sprintf("%s: %s", forecastCostLabel(cost), reason))
			continue
		}
		kind := cost.GrowthType
		resourceRate := applied
		if growthType != "" {
			kind = growthType
		} else if !forecastGrowthNeedsRate(kind) {
			resourceRate = nil
		}
		resources = append(resources, forecast.Resource{
			ID:         cost.ResourceID,
			Provider:   resourcetype.ExtractProvider(cost.ResourceType),
			Monthly:    cost.Monthly,
			Currency:   cost.Currency,
			GrowthType: kind,
			GrowthRate: resourceRate,
		})
	}
	return resources, warnings
}

func forecastGrowthNeedsRate(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case forecast.GrowthLinear, forecast.GrowthExponential:
		return true
	default:
		return false
	}
}

func forecastSkipReason(cost engine.CostResult) (string, bool) {
	if cost.Error != nil {
		if cost.Error.Message != "" {
			return cost.Error.Message, true
		}
		return cost.Error.Code, true
	}
	notes := strings.TrimSpace(cost.Notes)
	if strings.HasPrefix(notes, "ERROR:") || strings.HasPrefix(notes, "VALIDATION:") {
		return notes, true
	}
	if strings.Contains(strings.ToLower(notes), "no cost data available") {
		return notes, true
	}
	return "", false
}

func forecastCostLabel(cost engine.CostResult) string {
	if cost.ResourceID != "" {
		return cost.ResourceID
	}
	if cost.ResourceType != "" {
		return cost.ResourceType
	}
	return "resource"
}

func forecastHistorySnapshots(
	cmd *cobra.Command,
	deps forecastDeps,
) ([]history.CostSnapshot, error) {
	if deps.skipHistory {
		return deps.history, deps.historyErr
	}
	stack, err := optionalStack(cmd)
	if err != nil {
		return nil, err
	}
	return readForecastHistory(stack)
}

func optionalStack(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Lookup("stack") == nil {
		return "", nil
	}
	stack, err := cmd.Flags().GetString("stack")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stack), nil
}

func writeForecast(
	cmd *cobra.Command,
	format string,
	built forecastBuild,
	deps forecastDeps,
) error {
	switch format {
	case forecastOutputJSON:
		return writeJSON(cmd, built.document())
	case forecastOutputNDJSON:
		return writeForecastNDJSON(cmd, built.document())
	default:
		return writeForecastPlain(cmd, built.projection, built.history, built.budget, deps)
	}
}

type forecastLine struct {
	Kind     string     `json:"kind"`
	Series   string     `json:"series,omitempty"`
	Time     *time.Time `json:"time,omitempty"`
	Value    *float64   `json:"value,omitempty"`
	Currency string     `json:"currency,omitempty"`
	Months   int        `json:"months,omitempty"`
	Budget   *float64   `json:"budget,omitempty"`
	Message  string     `json:"message,omitempty"`
}

func writeForecastNDJSON(cmd *cobra.Command, doc forecast.Document) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	meta := forecastLine{Kind: "meta", Currency: doc.Currency, Months: doc.Months, Budget: doc.Budget}
	if err := enc.Encode(meta); err != nil {
		return err
	}
	for _, item := range doc.Series {
		for _, point := range item.Points {
			when := point.Time.UTC()
			value := point.Value
			line := forecastLine{Kind: "point", Series: item.Name, Time: &when, Value: &value}
			if err := enc.Encode(line); err != nil {
				return err
			}
		}
	}
	for _, warning := range doc.Warnings {
		if err := enc.Encode(forecastLine{Kind: forecast.LineWarning, Message: warning}); err != nil {
			return err
		}
	}
	return nil
}

func writeForecastPlain(
	cmd *cobra.Command,
	projection forecast.Projection,
	historySeries forecast.Series,
	budget float64,
	deps forecastDeps,
) error {
	height, err := cmd.Flags().GetInt("height")
	if err != nil {
		return err
	}
	width, err := cmd.Flags().GetInt("width")
	if err != nil {
		return err
	}
	if width <= 0 {
		width = deps.width
	}
	if width <= 0 {
		width = terminalWidth()
	}
	split, err := cmd.Flags().GetBool("split-providers")
	if err != nil {
		return err
	}
	cmd.Print(forecast.Render(projection, forecast.ChartOptions{
		Height:         height,
		Width:          width,
		SplitProviders: split,
		Budget:         budget,
		NoColor:        forecastNoColor(cmd),
	}))
	if len(historySeries.Points) > 0 {
		first := historySeries.Points[0].Time.UTC().Format("2006-01-02")
		last := historySeries.Points[len(historySeries.Points)-1].Time.UTC().Format("2006-01-02")
		cmd.Printf("History: %d snapshots, %s – %s (%s).\n",
			len(historySeries.Points), first, last, projection.Currency)
	}
	if len(projection.Warnings) > 0 {
		cmd.Println("Warnings:")
		for _, warning := range projection.Warnings {
			cmd.Printf("  %s\n", warning)
		}
	}
	return nil
}

func forecastNoColor(cmd *cobra.Command) bool {
	plain, _ := cmd.Flags().GetBool("plain")
	noColor, _ := cmd.Flags().GetBool("no-color")
	return plain || noColor || os.Getenv("NO_COLOR") != ""
}

func forecastPricingConfig(specDir string) (*config.Config, string) {
	cfg := config.GetGlobalConfig()
	if specDir == "" && cfg != nil {
		specDir = cfg.SpecDir
	}
	return cfg, specDir
}

// planOriginResources drops unchanged state: forecasts keep plan membership
// (#1903), so pricing those resources would only be discarded.
func planOriginResources(resources []engine.ResourceDescriptor) []engine.ResourceDescriptor {
	return slices.DeleteFunc(resources, func(r engine.ResourceDescriptor) bool {
		return r.Origin == engine.OriginState
	})
}
