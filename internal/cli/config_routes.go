package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/router"
)

const maxTestArgs = 2

// providerPatternOnly is the synthetic provider value assigned to plugins
// that use pattern-based matching instead of provider metadata.
const providerPatternOnly = "__pattern_only__"

// RoutesListOutput is the JSON output structure for the config routes list command.
type RoutesListOutput struct {
	Mode       string            `json:"mode"`
	ConfigPath string            `json:"config_path"`
	Source     string            `json:"source"`
	Rules      []RouteRuleOutput `json:"rules"`
}

// RouteRuleOutput represents a single routing rule in the list output.
type RouteRuleOutput struct {
	Plugin   string   `json:"plugin"`
	Priority int      `json:"priority"`
	Features []string `json:"features"`
	Patterns []string `json:"patterns"`
	Fallback bool     `json:"fallback"`
}

// RoutesTestOutput is the JSON output structure for the config routes test command.
type RoutesTestOutput struct {
	ResourceType string             `json:"resource_type"`
	Region       string             `json:"region,omitempty"`
	Provider     string             `json:"provider"`
	Mode         string             `json:"mode"`
	Matches      []RouteMatchOutput `json:"matches"`
	Features     map[string]string  `json:"features"`
}

// RouteMatchOutput represents a single plugin match in the test output.
type RouteMatchOutput struct {
	Rank        int    `json:"rank"`
	Plugin      string `json:"plugin"`
	Priority    int    `json:"priority"`
	MatchReason string `json:"match_reason"`
	Source      string `json:"source"`
	Fallback    bool   `json:"fallback"`
}

// validateOutputFormat checks that format is "table" or "json" and returns an
// error for unsupported values.
func validateOutputFormat(format string) error {
	if format != outputFormatTable && format != outputFormatJSON {
		return fmt.Errorf("unsupported output format: %s (supported: table, json)", format)
	}
	return nil
}

// loadRoutingContext loads the effective configuration and determines its source.
// It returns the loaded config and the source ("project" or "global").
func loadRoutingContext(ctx context.Context) (*config.Config, string) {
	projectDir := config.GetResolvedProjectDir()
	source := "global"
	if projectDir != "" {
		source = "project"
	}

	cfg := config.NewWithProjectDir(ctx, projectDir)

	if projectDir != "" {
		projectCfgPath := filepath.Join(projectDir, "config.yaml")
		if _, err := os.Stat(projectCfgPath); err == nil {
			cfg.SetConfigPath(projectCfgPath)
		}
	}

	return cfg, source
}

// NewConfigRoutesCmd returns a Cobra command that groups plugin routing subcommands.
// It has no RunE of its own — it delegates to the list and test subcommands.
func NewConfigRoutesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Plugin routing commands",
		Long:  "View and test the plugin routing configuration.",
	}

	cmd.AddCommand(NewConfigRoutesListCmd(), NewConfigRoutesTestCmd())

	return cmd
}

// NewConfigRoutesListCmd returns a Cobra command that displays the effective routing configuration.
// It supports table (default) and JSON output formats via the --output flag.
func NewConfigRoutesListCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Display routing rules",
		Long: `Display the effective plugin routing configuration.

Shows all configured routing rules sorted by priority (highest first),
including plugin names, features, patterns, and fallback status.`,
		Example: `  # Display routing rules as a table
  finfocus config routes list

  # Display as JSON for scripting
  finfocus config routes list --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigRoutesList(cmd, outputFormat)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", outputFormatTable, "output format (table, json)")

	return cmd
}

// runConfigRoutesList executes the config routes list command logic.
func runConfigRoutesList(cmd *cobra.Command, outputFormat string) error {
	cfg, source := loadRoutingContext(cmd.Context())

	if err := validateOutputFormat(outputFormat); err != nil {
		return err
	}

	if cfg.Routing == nil {
		if outputFormat == outputFormatJSON {
			return renderRoutesListJSON(cmd, nil, cfg.ConfigPath(), source)
		}
		cmd.Println("No routing configured (automatic mode)")
		cmd.Println("All plugins are queried based on their SupportedProviders metadata.")
		return nil
	}

	switch outputFormat {
	case outputFormatTable:
		return renderRoutesListTable(cmd, cfg, source)
	case outputFormatJSON:
		return renderRoutesListJSON(cmd, cfg.Routing, cfg.ConfigPath(), source)
	default:
		return nil
	}
}

// NewConfigRoutesTestCmd returns a command that simulates plugin routing for a resource type.
func NewConfigRoutesTestCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "test <resource-type> [region]",
		Short: "Test plugin selection for a resource type",
		Long: `Simulate plugin routing for a resource type and show match details.

This command does not contact plugins. It uses configured routing rules and
synthetic plugin metadata to show which plugin would be selected per feature.`,
		Example: `  # Test routing for EC2 instances
  finfocus config routes test aws:ec2:Instance

  # Include region in selection context
  finfocus config routes test aws:ec2:Instance us-east-1

  # JSON output for automation
  finfocus config routes test aws:ec2:Instance --output json`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("resource-type argument is required")
			}
			if len(args) > maxTestArgs {
				return errors.New("accepts at most 2 args: <resource-type> [region]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigRoutesTest(cmd, args, outputFormat)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", outputFormatTable, "output format (table, json)")

	return cmd
}

func runConfigRoutesTest(cmd *cobra.Command, args []string, outputFormat string) error {
	cfg, _ := loadRoutingContext(cmd.Context())

	if err := validateOutputFormat(outputFormat); err != nil {
		return err
	}

	resourceType := args[0]
	region := ""
	if len(args) > 1 {
		region = args[1]
	}
	provider := router.ExtractProviderFromType(resourceType)

	if cfg.Routing == nil {
		if outputFormat == outputFormatJSON {
			return renderRoutesTestJSON(
				cmd,
				resourceType,
				region,
				provider,
				"automatic",
				nil,
				nil,
			)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Plugin selection for %s (provider: %s", resourceType, provider)
		if region != "" {
			fmt.Fprintf(cmd.OutOrStdout(), ", region: %s", region)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "):")
		fmt.Fprintln(cmd.OutOrStdout(), "No routing configured (automatic mode)")
		fmt.Fprintf(cmd.OutOrStdout(), "All provider-matching plugins for %q would be queried.\n", provider)
		return nil
	}

	matches, featureMatches, err := simulatePluginSelection(cfg.Routing, resourceType, region, provider)
	if err != nil {
		return err
	}

	if outputFormat == outputFormatJSON {
		return renderRoutesTestJSON(
			cmd,
			resourceType,
			region,
			provider,
			"configured",
			matches,
			featureMatches,
		)
	}

	return renderRoutesTestTable(cmd, resourceType, region, provider, matches, featureMatches)
}

func buildSyntheticClients(routing *config.RoutingConfig) []*pluginhost.Client {
	if routing == nil {
		return nil
	}

	clients := make([]*pluginhost.Client, 0, len(routing.Plugins))
	for _, pluginCfg := range routing.Plugins {
		supportedProviders := []string{router.ProviderWildcard}
		if len(pluginCfg.Patterns) > 0 {
			supportedProviders = []string{providerPatternOnly}
		}

		clients = append(clients, &pluginhost.Client{
			Name: pluginCfg.Name,
			Metadata: &proto.PluginMetadata{
				Name:               pluginCfg.Name,
				SupportedProviders: supportedProviders,
				Metadata:           map[string]string{},
			},
		})
	}

	return clients
}

func simulatePluginSelection(
	routing *config.RoutingConfig,
	resourceType, region, provider string,
) ([]router.PluginMatch, map[string]router.PluginMatch, error) {
	clients := buildSyntheticClients(routing)
	rt, err := router.NewRouter(
		router.WithConfig(routing),
		router.WithClients(clients),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating router: %w", err)
	}

	resource := engine.ResourceDescriptor{
		Type:       resourceType,
		Provider:   provider,
		Properties: map[string]interface{}{},
	}
	if region != "" {
		resource.Properties["region"] = region
	}

	featureMatches := make(map[string]router.PluginMatch)
	seen := make(map[string]struct{})
	uniqueMatches := make([]router.PluginMatch, 0)
	for _, feature := range router.ValidFeatureNames() {
		matches := rt.SelectPlugins(context.Background(), resource, feature)
		if len(matches) > 0 {
			featureMatches[feature] = matches[0]
		}
		for _, match := range matches {
			if _, exists := seen[match.Client.Name]; exists {
				continue
			}
			seen[match.Client.Name] = struct{}{}
			uniqueMatches = append(uniqueMatches, match)
		}
	}

	sort.SliceStable(uniqueMatches, func(i, j int) bool {
		return uniqueMatches[i].Priority > uniqueMatches[j].Priority
	})

	return uniqueMatches, featureMatches, nil
}

func renderRoutesTestTable(
	cmd *cobra.Command,
	resourceType, region, provider string,
	matches []router.PluginMatch,
	featureMatches map[string]router.PluginMatch,
) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Plugin selection for %s (provider: %s", resourceType, provider)
	if region != "" {
		fmt.Fprintf(out, ", region: %s", region)
	}
	fmt.Fprintln(out, "):")
	fmt.Fprintln(out)

	w := tabwriter.NewWriter(out, 0, 0, tabPadding, ' ', 0)
	fmt.Fprintln(w, "  #\tPLUGIN\tPRIORITY\tMATCH REASON\tSOURCE")
	fmt.Fprintln(w, "  -\t------\t--------\t------------\t------")
	for i, match := range matches {
		fmt.Fprintf(
			w,
			"  %d\t%s\t%d\t%s\t%s\n",
			i+1,
			match.Client.Name,
			match.Priority,
			match.MatchReason.String(),
			match.Source,
		)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush table writer: %w", err)
	}

	if len(matches) == 0 {
		fmt.Fprintln(out, "No plugins match this resource type.")
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Feature availability:")
	for _, feature := range router.ValidFeatureNames() {
		match, ok := featureMatches[feature]
		if !ok {
			fmt.Fprintf(out, "  %s: (none)\n", feature)
			continue
		}
		fmt.Fprintf(out, "  %s: %s (priority %d)\n", feature, match.Client.Name, match.Priority)
	}

	return nil
}

func renderRoutesTestJSON(
	cmd *cobra.Command,
	resourceType, region, provider, mode string,
	matches []router.PluginMatch,
	featureMatches map[string]router.PluginMatch,
) error {
	output := RoutesTestOutput{
		ResourceType: resourceType,
		Region:       region,
		Provider:     provider,
		Mode:         mode,
		Matches:      []RouteMatchOutput{},
		Features:     make(map[string]string),
	}

	for i, match := range matches {
		output.Matches = append(output.Matches, RouteMatchOutput{
			Rank:        i + 1,
			Plugin:      match.Client.Name,
			Priority:    match.Priority,
			MatchReason: match.MatchReason.String(),
			Source:      match.Source,
			Fallback:    match.Fallback,
		})
	}

	for _, feature := range router.ValidFeatureNames() {
		output.Features[feature] = ""
		if featureMatch, ok := featureMatches[feature]; ok {
			output.Features[feature] = featureMatch.Client.Name
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	cmd.Println(string(data))
	return nil
}

// renderRoutesListTable renders the routing rules as a formatted table to the command output.
func renderRoutesListTable(cmd *cobra.Command, cfg *config.Config, source string) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "PLUGIN ROUTING RULES")
	fmt.Fprintln(out)

	w := tabwriter.NewWriter(out, 0, 0, tabPadding, ' ', 0)
	fmt.Fprintln(w, "  PRIORITY\tPLUGIN\tFEATURES\tPATTERNS\tFALLBACK")
	fmt.Fprintln(w, "  --------\t------\t--------\t--------\t--------")

	if len(cfg.Routing.Plugins) == 0 {
		fmt.Fprintln(w, "  -\t-\t-\t-\t-")
		if err := w.Flush(); err != nil {
			return fmt.Errorf("failed to flush table writer: %w", err)
		}
		fmt.Fprintln(out, "\n  No plugins configured")
		fmt.Fprintf(out, "  Source: %s (%s)\n", cfg.ConfigPath(), source)
		return nil
	}

	plugins := make([]config.PluginRouting, len(cfg.Routing.Plugins))
	copy(plugins, cfg.Routing.Plugins)
	sort.SliceStable(plugins, func(i, j int) bool {
		return plugins[i].Priority > plugins[j].Priority
	})

	for _, p := range plugins {
		features := "(all)"
		if len(p.Features) > 0 {
			features = strings.Join(p.Features, ",")
		}

		patterns := "(all)"
		if len(p.Patterns) > 0 {
			patStrs := make([]string, len(p.Patterns))
			for i, pat := range p.Patterns {
				patStrs[i] = pat.Type + ":" + pat.Pattern
			}
			patterns = strings.Join(patStrs, ",")
		}

		fallback := "yes"
		if !p.FallbackEnabled() {
			fallback = "no"
		}

		fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\n", p.Priority, p.Name, features, patterns, fallback)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush table writer: %w", err)
	}
	fmt.Fprintf(out, "\n  Source: %s (%s)\n", cfg.ConfigPath(), source)
	return nil
}

// renderRoutesListJSON renders the routing rules as JSON to the command output.
func renderRoutesListJSON(cmd *cobra.Command, routing *config.RoutingConfig, configPath, source string) error {
	output := RoutesListOutput{
		ConfigPath: configPath,
		Source:     source,
		Rules:      []RouteRuleOutput{},
	}

	if routing == nil {
		output.Mode = "automatic"
	} else {
		output.Mode = "configured"
		for _, p := range routing.Plugins {
			patterns := make([]string, 0, len(p.Patterns))
			for _, pat := range p.Patterns {
				patterns = append(patterns, pat.Type+":"+pat.Pattern)
			}

			features := p.Features
			if features == nil {
				features = []string{}
			}

			output.Rules = append(output.Rules, RouteRuleOutput{
				Plugin:   p.Name,
				Priority: p.Priority,
				Features: features,
				Patterns: patterns,
				Fallback: p.FallbackEnabled(),
			})
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	cmd.Println(string(data))
	return nil
}
