package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/registry"
)

// NewPluginListCmd creates a Cobra "list" command for displaying plugins.
// The command lists the highest semver of each installed plugin by default.
// --all lists every installed version, --verbose shows capability details,
// and --available lists plugins from the registry.
// It returns the configured *cobra.Command.
func NewPluginListCmd() *cobra.Command {
	var (
		verbose   bool
		listAll   bool
		available bool
		output    string
	)

	cmd := &cobra.Command{
		Use:   cmdNameList,
		Short: "List installed plugins",
		Long: "List installed plugins. By default each plugin appears once, at its " +
			"highest semver. Use --all to show every installed version.",
		Example: `  # List the latest installed version of each plugin
  finfocus plugin list

  # List every installed version and mark the one core uses
  finfocus plugin list --all

  # List plugins with detailed information
  finfocus plugin list --verbose

  # List available plugins from registry
  finfocus plugin list --available

  # List plugins as JSON for machine consumption
  finfocus plugin list --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format := resolveOutputFormat(cmd, "output", output)
			if format != outputFormatTable && format != outputFormatJSON {
				return fmt.Errorf("unsupported output format: %s (supported: table, json)", format)
			}
			if available {
				return runPluginListAvailable(cmd, format)
			}
			return runPluginListCmd(cmd, verbose, listAll, format)
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed plugin information")
	cmd.Flags().BoolVar(&listAll, "all", false, "List every installed version and mark the one core uses")
	cmd.Flags().BoolVar(&available, "available", false, "List available plugins from registry")
	cmd.Flags().StringVar(&output, "output", outputFormatTable, "Output format (table, json)")

	return cmd
}

// runPluginListAvailable lists plugins available in the registry. With JSON output it
// writes the registry entries as a JSON array (an empty array when there are none);
// otherwise it writes a tabulated table (Name, Description, Repository, Security).
//
// If the registry cannot be loaded the function returns an error wrapping the underlying
// cause. If no entries exist the table form prints "No plugins available in registry."
// and returns nil. For entries with an empty security level the security column
// defaults to "community".
func runPluginListAvailable(cmd *cobra.Command, output string) error {
	entries, err := registry.GetAllPluginEntries()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	if output == outputFormatJSON {
		if entries == nil {
			entries = []registry.RegistryEntry{}
		}
		return writeJSON(cmd, entries)
	}

	if len(entries) == 0 {
		cmd.Println("No plugins available in registry.")
		return nil
	}

	const tabPadding = 2
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)

	fmt.Fprintln(w, "Name\tDescription\tRepository\tSecurity")
	fmt.Fprintln(w, "----\t-----------\t----------\t--------")

	for _, entry := range entries {
		security := entry.SecurityLevel
		if security == "" {
			security = "community"
		}
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\n",
			entry.Name,
			entry.Description,
			entry.Repository,
			security,
		)
	}
	return w.Flush()
}

type enrichedPluginInfo struct {
	registry.PluginInfo

	// Metadata
	SpecVersion        string   `json:"specVersion"`
	RuntimeVersion     string   `json:"runtimeVersion"`
	SupportedProviders []string `json:"supportedProviders,omitempty"`
	Capabilities       []string `json:"capabilities,omitempty"` // Inferred from plugin methods
	Notes              string   `json:"notes,omitempty"`        // Error or status notes
	// Active is true when this row is the version core opens.
	Active bool `json:"active"`
	// probed is true when this process was started to read metadata.
	probed bool
}

// pluginListLauncher builds the launcher that reads plugin metadata.
// Tests replace it to record which binaries would start.
//
//nolint:gochecknoglobals // Test seam; production assigns it once.
var pluginListLauncher = func() pluginhost.Launcher {
	return pluginhost.NewProcessLauncher()
}

const notAvailable = "N/A"

// runPluginListCmd lists installed plugins and writes a tabulated listing to the provided Cobra command output.
// It checks whether the configured plugin directory exists and prints a message and returns nil if it does not.
// If no plugins are installed it prints 'No plugins found.' and returns nil.
//
// By default each plugin appears once, at the highest semver (the version core
// opens). listAll includes every installed version and marks that one. A
// directory whose name is not semver is warned about and left out of the
// default listing. Plugin metadata is fetched concurrently using errgroup with
// a concurrency limit of [runtime.NumCPU]. Older versions are not started
// unless verbose is set. Results are sorted by plugin name ascending, then
// installed version descending as semver. A version that is not semver falls
// back to ascending string comparison. Path breaks remaining ties, so metadata
// completion order cannot reorder rows.
//
// cmd is the Cobra command used for printing. verbose controls whether plugin details are shown.
// Returns an error if querying the registry for installed plugins fails; otherwise nil.
func runPluginListCmd(cmd *cobra.Command, verbose, listAll bool, output string) error {
	cfg := config.New()
	if _, err := os.Stat(cfg.PluginDir); os.IsNotExist(err) {
		if output == outputFormatJSON {
			return renderPluginsJSON(cmd.OutOrStdout(), nil)
		}
		cmd.Printf("Plugin directory does not exist: %s\n", cfg.PluginDir)
		cmd.Println("No plugins installed.")
		return nil
	}

	reg := registry.NewDefault()
	plugins, active, err := pluginsForList(reg, listAll)
	if err != nil {
		return err
	}
	for _, warning := range active.warnings {
		cmd.PrintErrf("warning: %s\n", warning)
	}

	if len(plugins) == 0 {
		if output == outputFormatJSON {
			return renderPluginsJSON(cmd.OutOrStdout(), nil)
		}
		cmd.Println("No plugins found.")
		return nil
	}

	enriched := enrichPlugins(cmd.Context(), plugins, active.paths, verbose)

	// Name, then semver, then path. Completion order must not change the rows.
	sort.Slice(enriched, func(i, j int) bool {
		if enriched[i].Name != enriched[j].Name {
			return enriched[i].Name < enriched[j].Name
		}
		if cmp := comparePluginVersions(enriched[i].Version, enriched[j].Version); cmp != 0 {
			return cmp < 0
		}
		return enriched[i].Path < enriched[j].Path
	})

	if output == outputFormatJSON {
		return renderPluginsJSON(cmd.OutOrStdout(), enriched)
	}
	return displayPlugins(cmd, enriched, verbose, listAll)
}

// pluginListSelection is the version core opens, plus semver warnings.
type pluginListSelection struct {
	paths    map[string]struct{}
	warnings []string
}

// pluginsForList returns the rows to print. The default is the highest semver
// of each plugin. listAll returns every installed version. paths marks the
// version core opens. Invalid semver names are warnings in either mode.
func pluginsForList(reg *registry.Registry, listAll bool) ([]registry.PluginInfo, pluginListSelection, error) {
	latest, warnings, err := reg.ListLatestPlugins()
	if err != nil {
		return nil, pluginListSelection{}, fmt.Errorf("listing plugins: %w", err)
	}
	selected := pluginListSelection{
		paths:    make(map[string]struct{}, len(latest)),
		warnings: warnings,
	}
	for _, plugin := range latest {
		selected.paths[plugin.Path] = struct{}{}
	}
	if !listAll {
		return latest, selected, nil
	}

	plugins, err := reg.ListPlugins()
	if err != nil {
		return nil, pluginListSelection{}, fmt.Errorf("listing plugins: %w", err)
	}
	return plugins, selected, nil
}

// enrichPlugins reads metadata for the versions that should be started.
// Older versions stay unprobed unless verbose is set, so listing them does
// not launch their binaries.
func enrichPlugins(
	ctx context.Context,
	plugins []registry.PluginInfo,
	active map[string]struct{},
	verbose bool,
) []enrichedPluginInfo {
	probe := make([]registry.PluginInfo, 0, len(plugins))
	var skipped []registry.PluginInfo
	for _, plugin := range plugins {
		_, isActive := active[plugin.Path]
		if verbose || isActive {
			probe = append(probe, plugin)
			continue
		}
		skipped = append(skipped, plugin)
	}

	enriched := fetchPluginMetadataParallel(ctx, probe)
	for i := range enriched {
		_, enriched[i].Active = active[enriched[i].Path]
	}
	for _, plugin := range skipped {
		enriched = append(enriched, enrichedPluginInfo{
			PluginInfo:     plugin,
			SpecVersion:    notAvailable,
			RuntimeVersion: notAvailable,
		})
	}
	return enriched
}

// fetchPluginMetadataParallel fetches metadata from all plugins concurrently.
// It uses errgroup with a concurrency limit of [runtime.NumCPU] to prevent resource exhaustion.
// A plugin that fails to respond is still included, with the failure in Notes.
func fetchPluginMetadataParallel(ctx context.Context, plugins []registry.PluginInfo) []enrichedPluginInfo {
	launcher := pluginListLauncher()

	// Thread-safe collection of results
	var mu sync.Mutex
	var enriched []enrichedPluginInfo

	// Use errgroup with concurrency limit
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for _, p := range plugins {
		g.Go(func() error {
			info := fetchSinglePluginMetadata(gCtx, launcher, p)
			// Always add result - fetchSinglePluginMetadata never returns nil
			mu.Lock()
			enriched = append(enriched, *info)
			mu.Unlock()
			// Always return nil - we don't want one plugin failure to cancel others
			return nil
		})
	}

	// Wait for all goroutines to complete (errors are intentionally ignored)
	_ = g.Wait()

	return enriched
}

// comparePluginVersions orders semver versions newest first, then versions
// that are not semver in ascending string order. Grouping keeps the order
// transitive.
func comparePluginVersions(left, right string) int {
	leftVer, leftErr := semver.NewVersion(left)
	rightVer, rightErr := semver.NewVersion(right)
	switch {
	case leftErr == nil && rightErr == nil:
		return rightVer.Compare(leftVer)
	case leftErr == nil:
		return -1
	case rightErr == nil:
		return 1
	}
	return strings.Compare(left, right)
}

// fetchSinglePluginMetadata fetches metadata for a single plugin with timeout.
// fetchSinglePluginMetadata fetches runtime metadata for the given plugin and returns an enrichedPluginInfo.
//
// It attempts to contact the plugin via the provided launcher and, when metadata is available,
// populates SpecVersion, RuntimeVersion, SupportedProviders, and Capabilities. SpecVersion and
// RuntimeVersion default to "N/A" when not provided. Capabilities are inferred to include
// "ProjectedCosts" and "ActualCosts" when metadata is present.
//
// Parameters:
//   - ctx: the context used for the operation (may carry cancellation and logging).
//   - launcher: the pluginhost launcher used to start or connect to the plugin.
//   - plugin: the registry.PluginInfo describing the plugin to inspect.
//
// Returns:
//   - a non-nil *enrichedPluginInfo containing the original PluginInfo plus any discovered
//     metadata. If any step fails, the returned struct's Notes field contains a human-readable
//     failure description.
func fetchSinglePluginMetadata(
	ctx context.Context,
	launcher pluginhost.Launcher,
	plugin registry.PluginInfo,
) *enrichedPluginInfo {
	log := logging.FromContext(ctx)
	const launchTimeout = 5 * time.Second
	launchCtx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()

	// Initialize result with N/A values
	result := &enrichedPluginInfo{
		PluginInfo:     plugin,
		SpecVersion:    notAvailable,
		RuntimeVersion: notAvailable,
		probed:         true,
	}

	client, launchErr := pluginhost.NewClient(
		pluginhost.WithInstallName(launchCtx, plugin.Name), launcher, plugin.Path)
	if launchErr != nil {
		log.Debug().
			Ctx(ctx).
			Str("plugin_path", plugin.Path).
			Err(launchErr).
			Msg("failed to launch plugin during list enumeration")
		// Return result with error note instead of nil
		result.Notes = fmt.Sprintf("Failed: %v", launchErr)
		return result
	}
	defer func() { _ = client.Close() }()

	if client.Metadata == nil {
		return result
	}

	if client.Metadata.SpecVersion != "" {
		result.SpecVersion = client.Metadata.SpecVersion
	}
	if client.Metadata.Version != "" {
		result.RuntimeVersion = client.Metadata.Version
	}
	if len(client.Metadata.SupportedProviders) > 0 {
		result.SupportedProviders = client.Metadata.SupportedProviders
	}

	// Read capabilities from plugin metadata
	if len(client.Metadata.Capabilities) > 0 {
		result.Capabilities = client.Metadata.Capabilities
	} else {
		// Fallback for legacy plugins without capability reporting
		result.Capabilities = []string{"ProjectedCosts", "ActualCosts"}
	}

	return result
}

func displayPlugins(cmd *cobra.Command, plugins []enrichedPluginInfo, verbose, showActive bool) error {
	const tabPadding = 2
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)
	return writePluginTable(w, plugins, verbose, showActive)
}

func pluginHasNotes(plugins []enrichedPluginInfo) bool {
	for _, plugin := range plugins {
		if plugin.Notes != "" {
			return true
		}
	}
	return false
}

func activeLabel(active bool) string {
	if active {
		return "yes"
	}
	return "no"
}

func (p enrichedPluginInfo) providerCell() string {
	if !p.probed {
		return "-"
	}
	return formatProviders(p.SupportedProviders)
}

func (p enrichedPluginInfo) capabilityCell() string {
	if !p.probed {
		return "-"
	}
	return formatCapabilities(p.Capabilities)
}

func pluginHeader(verbose, showActive, hasNotes bool) []string {
	header := []string{"Name", "Version"}
	if showActive {
		header = append(header, "Active")
	}
	header = append(header, "Providers")
	if verbose {
		header = append(header, "Capabilities", "Spec", "Path")
	}
	if hasNotes {
		header = append(header, "Notes")
	}
	return header
}

func pluginCells(plugin enrichedPluginInfo, verbose, showActive, hasNotes bool) []string {
	// Version is the install directory name. Only a started plugin has a
	// runtime version, so the column stays comparable across --all rows.
	cells := []string{plugin.Name, plugin.Version}
	if showActive {
		cells = append(cells, activeLabel(plugin.Active))
	}
	cells = append(cells, plugin.providerCell())
	if verbose {
		cells = append(cells, plugin.capabilityCell(), plugin.SpecVersion, plugin.Path)
	}
	if hasNotes {
		cells = append(cells, plugin.Notes)
	}
	return cells
}

func writePluginTable(w *tabwriter.Writer, plugins []enrichedPluginInfo, verbose, showActive bool) error {
	hasNotes := pluginHasNotes(plugins)
	header := pluginHeader(verbose, showActive, hasNotes)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	rule := make([]string, len(header))
	for i, column := range header {
		rule[i] = strings.Repeat("-", len(column))
	}
	fmt.Fprintln(w, strings.Join(rule, "\t"))
	for _, plugin := range plugins {
		fmt.Fprintln(w, strings.Join(pluginCells(plugin, verbose, showActive, hasNotes), "\t"))
	}
	return w.Flush()
}

// formatProviders formats the list of supported providers for display.
// formatProviders returns "*" for global plugins when the providers slice is empty or contains only `"*"`, and returns a comma-separated list of providers otherwise.
// The providers parameter is a slice of provider identifiers; the returned string preserves the order of the slice.
func formatProviders(providers []string) string {
	if len(providers) == 0 {
		return "*"
	}
	if len(providers) == 1 && providers[0] == "*" {
		return "*"
	}
	return strings.Join(providers, ", ")
}

// formatCapabilities formats the list of capabilities for display.
// formatCapabilities returns a comma-separated string of capabilities.
// If the slice is empty, it returns "-".
func formatCapabilities(capabilities []string) string {
	if len(capabilities) == 0 {
		return "-"
	}
	return strings.Join(capabilities, ", ")
}

// PluginJSONEntry is the JSON-serializable representation of a plugin for
// the --output json format. It matches the plugin-list-json contract schema.
type PluginJSONEntry struct {
	Name               string   `json:"name"`
	Version            string   `json:"version"`
	Path               string   `json:"path"`
	SpecVersion        string   `json:"specVersion"`
	RuntimeVersion     string   `json:"runtimeVersion"`
	SupportedProviders []string `json:"supportedProviders"`
	Capabilities       []string `json:"capabilities"`
	Notes              string   `json:"notes,omitempty"`
	// Active reports whether this is the version core opens.
	Active bool `json:"active"`
}

// renderPluginsJSON serializes the enriched plugin list as a JSON array to w.
// An empty or nil input produces "[]".
func renderPluginsJSON(w io.Writer, plugins []enrichedPluginInfo) error {
	entries := make([]PluginJSONEntry, 0, len(plugins))
	for _, p := range plugins {
		providers := p.SupportedProviders
		capabilities := p.Capabilities
		// An unstarted row has no provider list. null is unknown; [] is a
		// started plugin that reported none.
		if p.probed {
			if providers == nil {
				providers = []string{}
			}
			if capabilities == nil {
				capabilities = []string{}
			}
		}
		entries = append(entries, PluginJSONEntry{
			Name:               p.Name,
			Version:            p.Version,
			Path:               p.Path,
			SpecVersion:        p.SpecVersion,
			RuntimeVersion:     p.RuntimeVersion,
			SupportedProviders: providers,
			Capabilities:       capabilities,
			Notes:              p.Notes,
			Active:             p.Active,
		})
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plugin list to JSON: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
