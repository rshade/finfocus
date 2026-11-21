package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/pluginname"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/resourcetype"
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
	// Warnings lists routing entries that need attention: names that match no
	// installed plugin (not simulated) and former names. Omitted when empty.
	Warnings []RouteWarningOutput `json:"warnings,omitempty"`
}

// RouteWarningOutput is a warning about one routing entry in the test output.
type RouteWarningOutput struct {
	Plugin  string `json:"plugin"`
	Message string `json:"message"`
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

// validateOutputFormat checks that format is "table" or "json" and returns a
// validation error for unsupported values.
func validateOutputFormat(ctx context.Context, format string) error {
	if format != outputFormatTable && format != outputFormatJSON {
		return unsupportedOutputError(ctx, format, "table, json")
	}
	return nil
}

// loadRoutingContext loads the effective configuration and determines its source.
// It returns the loaded config and the source ("project" or "global").
func loadRoutingContext(ctx context.Context) (*config.Config, string) {
	projectDir := config.GetResolvedProjectDir()
	source := "global"

	cfg := config.NewWithProjectDir(ctx, projectDir)

	if projectDir != "" {
		projectCfgPath := filepath.Join(projectDir, "config.hujson")
		if _, err := os.Stat(projectCfgPath); err != nil {
			// Fall back to the legacy filename for projects not yet migrated.
			legacyCfgPath := filepath.Join(projectDir, "config.yaml")
			if _, legacyErr := os.Stat(legacyCfgPath); legacyErr == nil {
				projectCfgPath = legacyCfgPath
			}
		}
		if _, err := os.Stat(projectCfgPath); err == nil {
			source = "project"
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
		Use:   cmdNameList,
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
	outputFormat = resolveOutputFormat(cmd, "output", outputFormat)
	if err := validateOutputFormat(cmd.Context(), outputFormat); err != nil {
		return err
	}

	cfg, source := loadRoutingContext(cmd.Context())

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

This command does not contact plugins. It uses configured routing rules,
the installed plugins, and synthetic plugin metadata to show which plugin would
be selected per feature. A routing entry is simulated when it names an
installed plugin by its install name (as 'finfocus plugin list' shows it) or a
former name from the plugin catalog. Other entries are reported as warnings;
run 'finfocus config validate' to check them against the names running
plugins report.`,
		Example: `  # Test routing for EC2 instances
  finfocus config routes test aws:ec2/instance:Instance

  # Include region in selection context
  finfocus config routes test aws:ec2/instance:Instance us-east-1

  # JSON output for automation
  finfocus config routes test aws:ec2/instance:Instance --output json`,
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
	outputFormat = resolveOutputFormat(cmd, "output", outputFormat)
	if err := validateOutputFormat(cmd.Context(), outputFormat); err != nil {
		return err
	}

	cfg, _ := loadRoutingContext(cmd.Context())

	resourceType := args[0]
	region := ""
	if len(args) > 1 {
		region = args[1]
	}
	provider := router.ExtractProviderFromType(resourceType)

	if cfg.Routing == nil {
		if outputFormat == outputFormatJSON {
			return renderRoutesTestJSON(cmd, resourceType, region, provider, "automatic", routesSimulation{})
		}
		cmd.Printf("Plugin selection for %s (provider: %s", resourceType, providerLabel(resourceType, provider))
		if region != "" {
			cmd.Printf(", region: %s", region)
		}
		cmd.Println("):")
		cmd.Println("No routing configured (automatic mode)")
		cmd.Printf("All provider-matching plugins for %q would be queried.\n", provider)
		return nil
	}

	installed, err := installedRoutePlugins()
	if err != nil {
		return err
	}
	sim, err := simulatePluginSelection(cmd.Context(), cfg.Routing, installed, resourceType, region, provider)
	if err != nil {
		return err
	}

	if outputFormat == outputFormatJSON {
		return renderRoutesTestJSON(cmd, resourceType, region, provider, "configured", sim)
	}

	return renderRoutesTestTable(cmd, resourceType, region, provider, sim)
}

// routesSimulation is what `config routes test` reports for one resource.
type routesSimulation struct {
	matches        []router.PluginMatch
	featureMatches map[string]router.PluginMatch
	warnings       []RouteWarningOutput
}

// installedRoutePlugin is an installed plugin as routes test sees it without
// launching it: its install name and its catalog aliases.
type installedRoutePlugin struct {
	name    string
	aliases []string
}

// installedRoutePlugins lists the installed plugins (latest version of each)
// with their catalog aliases. It never launches a plugin.
func installedRoutePlugins() ([]installedRoutePlugin, error) {
	plugins, _, err := registry.NewDefault().ListLatestPlugins()
	if err != nil {
		return nil, fmt.Errorf("listing installed plugins: %w", err)
	}
	installed := make([]installedRoutePlugin, 0, len(plugins))
	for _, plugin := range plugins {
		installed = append(
			installed,
			installedRoutePlugin{name: plugin.Name, aliases: registry.CatalogAliases(plugin.Name)},
		)
	}
	return installed, nil
}

// buildSyntheticClients builds one stand-in client per installed plugin that a
// routing entry names, by install name or catalog alias, and a warning for
// every other entry. A stand-in serves every provider unless all entries
// naming it use patterns.
func buildSyntheticClients(
	routing *config.RoutingConfig,
	installed []installedRoutePlugin,
) ([]*pluginhost.Client, []RouteWarningOutput) {
	if routing == nil {
		return nil, nil
	}

	var clients []*pluginhost.Client
	var warnings []RouteWarningOutput
	byName := make(map[string]*pluginhost.Client)
	for _, pluginCfg := range routing.Plugins {
		plugin, viaAlias, found := findInstalledRoutePlugin(installed, pluginCfg.Name)
		if !found {
			if pluginCfg.Name != "" {
				warnings = append(warnings, RouteWarningOutput{
					Plugin:  pluginCfg.Name,
					Message: notInstalledRouteMessage(installed, pluginCfg.Name),
				})
			}
			continue
		}
		if viaAlias {
			warnings = append(warnings, RouteWarningOutput{
				Plugin:  pluginCfg.Name,
				Message: router.AliasWarning(pluginCfg.Name, plugin.name, true),
			})
		}

		client, exists := byName[plugin.name]
		if !exists {
			client = &pluginhost.Client{
				Name:        plugin.name,
				InstallName: plugin.name,
				Aliases:     plugin.aliases,
				Metadata: &proto.PluginMetadata{
					Name:               plugin.name,
					SupportedProviders: []string{providerPatternOnly},
					Metadata:           map[string]string{},
				},
			}
			byName[plugin.name] = client
			clients = append(clients, client)
		}
		if len(pluginCfg.Patterns) == 0 {
			client.Metadata.SupportedProviders = []string{router.ProviderWildcard}
		}
	}

	return clients, warnings
}

// findInstalledRoutePlugin finds the installed plugin a routing entry names by
// install name, or else by catalog alias.
func findInstalledRoutePlugin(installed []installedRoutePlugin, name string) (installedRoutePlugin, bool, bool) {
	for _, plugin := range installed {
		if plugin.name == name {
			return plugin, false, true
		}
	}
	for _, plugin := range installed {
		if slices.Contains(plugin.aliases, name) {
			return plugin, true, true
		}
	}
	return installedRoutePlugin{}, false, false
}

// notInstalledRouteMessage explains why an entry is not simulated, suggesting
// the install name when the entry is the binary name of an installed plugin.
func notInstalledRouteMessage(installed []installedRoutePlugin, name string) string {
	const prefix = "does not match an installed plugin; not simulated"
	if bare, ok := strings.CutPrefix(name, pluginname.BinaryPrefix); ok {
		if _, _, found := findInstalledRoutePlugin(installed, bare); found {
			return fmt.Sprintf("%s (did you mean %q?)", prefix, bare)
		}
	}
	return prefix + " (run 'finfocus config validate' to check reported plugin names)"
}

func simulatePluginSelection(
	ctx context.Context,
	routing *config.RoutingConfig,
	installed []installedRoutePlugin,
	resourceType, region, provider string,
) (routesSimulation, error) {
	clients, warnings := buildSyntheticClients(routing, installed)
	rt, err := router.NewRouter(
		router.WithConfig(routing),
		router.WithClients(clients),
	)
	if err != nil {
		return routesSimulation{}, fmt.Errorf("creating router: %w", err)
	}

	resource := engine.ResourceDescriptor{
		Type:       resourceType,
		Provider:   provider,
		Properties: map[string]any{},
	}
	if region != "" {
		resource.Properties["region"] = region
	}

	featureMatches := make(map[string]router.PluginMatch)
	seen := make(map[string]struct{})
	uniqueMatches := make([]router.PluginMatch, 0)
	for _, feature := range router.ValidFeatureNames() {
		matches := rt.SelectPlugins(ctx, resource, feature)
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

	return routesSimulation{matches: uniqueMatches, featureMatches: featureMatches, warnings: warnings}, nil
}

// providerLabel returns provider for display, adding the raw package when it
// differs ("aws (package: aws-native)").
func providerLabel(resourceType, provider string) string {
	pkg := resourcetype.ExtractPackage(resourceType)
	if strings.EqualFold(pkg, provider) {
		return provider
	}
	return fmt.Sprintf("%s (package: %s)", provider, pkg)
}

func renderRoutesTestTable(
	cmd *cobra.Command,
	resourceType, region, provider string,
	sim routesSimulation,
) error {
	matches, featureMatches := sim.matches, sim.featureMatches
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Plugin selection for %s (provider: %s", resourceType, providerLabel(resourceType, provider))
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

	if len(sim.warnings) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Warnings:")
		for _, warning := range sim.warnings {
			fmt.Fprintf(out, "  %s: %s\n", warning.Plugin, warning.Message)
		}
	}

	return nil
}

func renderRoutesTestJSON(
	cmd *cobra.Command,
	resourceType, region, provider, mode string,
	sim routesSimulation,
) error {
	output := RoutesTestOutput{
		ResourceType: resourceType,
		Region:       region,
		Provider:     provider,
		Mode:         mode,
		Matches:      []RouteMatchOutput{},
		Features:     make(map[string]string),
		Warnings:     sim.warnings,
	}

	for i, match := range sim.matches {
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
		if featureMatch, ok := sim.featureMatches[feature]; ok {
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
		sort.SliceStable(output.Rules, func(i, j int) bool {
			return output.Rules[i].Priority > output.Rules[j].Priority
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	cmd.Println(string(data))
	return nil
}
