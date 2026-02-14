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

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/registry"
)

// NewPluginListCmd creates a Cobra "list" command for displaying plugins.
// The command lists installed plugins by default and supports an `--verbose`
// flag for detailed output and an `--available` flag to list plugins from the registry.
// It returns the configured *cobra.Command.
func NewPluginListCmd() *cobra.Command {
	var (
		verbose   bool
		available bool
		output    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Long:  "List all installed plugins with their versions and paths",
		Example: `  # List all installed plugins
  finfocus plugin list

  # List plugins with detailed information
  finfocus plugin list --verbose

  # List available plugins from registry
  finfocus plugin list --available

  # List plugins as JSON for machine consumption
  finfocus plugin list --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if output != outputFormatTable && output != outputFormatJSON {
				return fmt.Errorf("unsupported output format: %s (supported: table, json)", output)
			}
			if available {
				return runPluginListAvailable(cmd)
			}
			return runPluginListCmd(cmd, verbose, output)
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed plugin information")
	cmd.Flags().BoolVar(&available, "available", false, "List available plugins from registry")
	cmd.Flags().StringVar(&output, "output", outputFormatTable, "Output format (table, json)")

	return cmd
}

// runPluginListAvailable lists plugins available in the registry and writes a tabulated
// table (Name, Description, Repository, Security) to the command's output.
//
// If the registry cannot be loaded the function returns an error wrapping the underlying
// cause. If no entries exist the function prints "No plugins available in registry."
// to the command output and returns nil. For entries with an empty security level the
// security column defaults to "community".
func runPluginListAvailable(cmd *cobra.Command) error {
	entries, err := registry.GetAllPluginEntries()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
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
}

// displayVersion returns RuntimeVersion when it's not notAvailable, otherwise Version.
func (p enrichedPluginInfo) displayVersion() string {
	if p.RuntimeVersion != notAvailable {
		return p.RuntimeVersion
	}
	return p.Version
}

const notAvailable = "N/A"

// runPluginListCmd lists installed plugins and writes a tabulated listing to the provided Cobra command output.
// It checks whether the configured plugin directory exists and prints a message and returns nil if it does not.
// If no plugins are installed it prints 'No plugins found.' and returns nil.
//
// Plugin metadata is fetched concurrently using errgroup with a concurrency limit of runtime.NumCPU()
// to minimize total execution time. Results are sorted deterministically by plugin name.
//
// cmd is the Cobra command used for printing. verbose controls whether plugin details are shown.
// Returns an error if querying the registry for installed plugins fails; otherwise nil.
func runPluginListCmd(cmd *cobra.Command, verbose bool, output string) error {
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
	plugins, err := reg.ListPlugins()
	if err != nil {
		return fmt.Errorf("listing plugins: %w", err)
	}

	if len(plugins) == 0 {
		if output == outputFormatJSON {
			return renderPluginsJSON(cmd.OutOrStdout(), nil)
		}
		cmd.Println("No plugins found.")
		return nil
	}

	ctx := cmd.Context()
	enriched := fetchPluginMetadataParallel(ctx, plugins)

	// Sort by plugin name for deterministic output
	sort.Slice(enriched, func(i, j int) bool {
		return enriched[i].Name < enriched[j].Name
	})

	if output == outputFormatJSON {
		return renderPluginsJSON(cmd.OutOrStdout(), enriched)
	}
	return displayPlugins(cmd, enriched, verbose)
}

// fetchPluginMetadataParallel fetches metadata from all plugins concurrently.
// It uses errgroup with a concurrency limit of runtime.NumCPU() to prevent resource exhaustion.
// Plugins that fail to respond within the timeout are skipped (not included in results).
func fetchPluginMetadataParallel(ctx context.Context, plugins []registry.PluginInfo) []enrichedPluginInfo {
	launcher := pluginhost.NewProcessLauncher()

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
	}

	client, launchErr := pluginhost.NewClient(launchCtx, launcher, plugin.Path)
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

func displayPlugins(cmd *cobra.Command, plugins []enrichedPluginInfo, verbose bool) error {
	const tabPadding = 2
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)

	if verbose {
		return displayVerbosePlugins(w, plugins)
	}
	return displaySimplePlugins(w, plugins)
}

// displayVerbosePlugins writes a detailed tab-separated table of plugin information to w.
// The table includes columns for Name, Version, Providers, Capabilities, Spec, and Path.
// If any plugin in the provided slice has a non-empty Notes field, a Notes column is included.
// w is the tabwriter used for output and plugins is the list of enrichedPluginInfo to display.
// It returns any error encountered while flushing the writer.
func displayVerbosePlugins(w *tabwriter.Writer, plugins []enrichedPluginInfo) error {
	// Check if any plugins have notes to show
	hasNotes := false
	for _, plugin := range plugins {
		if plugin.Notes != "" {
			hasNotes = true
			break
		}
	}

	if hasNotes {
		fmt.Fprintln(w, "Name\tVersion\tProviders\tCapabilities\tSpec\tPath\tNotes")
		fmt.Fprintln(w, "----\t-------\t---------\t------------\t----\t----\t-----")
	} else {
		fmt.Fprintln(w, "Name\tVersion\tProviders\tCapabilities\tSpec\tPath")
		fmt.Fprintln(w, "----\t-------\t---------\t------------\t----\t----")
	}

	for _, plugin := range plugins {
		ver := plugin.displayVersion()
		providers := formatProviders(plugin.SupportedProviders)
		capabilities := formatCapabilities(plugin.Capabilities)

		if hasNotes {
			fmt.Fprintf(
				w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				plugin.Name, ver, providers, capabilities, plugin.SpecVersion, plugin.Path, plugin.Notes,
			)
		} else {
			fmt.Fprintf(
				w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				plugin.Name, ver, providers, capabilities, plugin.SpecVersion, plugin.Path,
			)
		}
	}
	return w.Flush()
}

// displaySimplePlugins writes a compact tab-separated table of plugins to w.
// If any plugin has a non-empty Notes field, a Notes column is included.
// Each row contains the plugin name, a display version (RuntimeVersion when available,
// otherwise the plugin version), and the formatted providers; Notes are appended when present.
// Parameters:
//   - w: destination tab writer for formatted output.
//   - plugins: list of enriched plugin information to display.
//
// Returns any error encountered while flushing the writer.
func displaySimplePlugins(w *tabwriter.Writer, plugins []enrichedPluginInfo) error {
	// Check if any plugins have notes to show
	hasNotes := false
	for _, plugin := range plugins {
		if plugin.Notes != "" {
			hasNotes = true
			break
		}
	}

	if hasNotes {
		fmt.Fprintln(w, "Name\tVersion\tProviders\tNotes")
		fmt.Fprintln(w, "----\t-------\t---------\t-----")
	} else {
		fmt.Fprintln(w, "Name\tVersion\tProviders")
		fmt.Fprintln(w, "----\t-------\t---------")
	}

	for _, plugin := range plugins {
		ver := plugin.displayVersion()
		providers := formatProviders(plugin.SupportedProviders)
		if hasNotes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", plugin.Name, ver, providers, plugin.Notes)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", plugin.Name, ver, providers)
		}
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
}

// renderPluginsJSON serializes the enriched plugin list as a JSON array to w.
// An empty or nil input produces "[]".
func renderPluginsJSON(w io.Writer, plugins []enrichedPluginInfo) error {
	entries := make([]PluginJSONEntry, 0, len(plugins))
	for _, p := range plugins {
		entries = append(entries, PluginJSONEntry{
			Name:               p.Name,
			Version:            p.displayVersion(),
			Path:               p.Path,
			SpecVersion:        p.SpecVersion,
			RuntimeVersion:     p.RuntimeVersion,
			SupportedProviders: p.SupportedProviders,
			Capabilities:       p.Capabilities,
			Notes:              p.Notes,
		})
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plugin list to JSON: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
