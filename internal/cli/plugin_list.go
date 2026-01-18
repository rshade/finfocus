package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// NewPluginListCmd creates a Cobra "list" command for displaying plugins.
// The command lists installed plugins by default and supports an `--verbose`
// flag for detailed output and an `--available` flag to list plugins from the registry.
// It returns the configured *cobra.Command.
func NewPluginListCmd() *cobra.Command {
	var (
		verbose   bool
		available bool
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
  finfocus plugin list --available`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if available {
				return runPluginListAvailable(cmd)
			}
			return runPluginListCmd(cmd, verbose)
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed plugin information")
	cmd.Flags().BoolVar(&available, "available", false, "List available plugins from registry")

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
	SpecVersion    string `json:"specVersion"`
	RuntimeVersion string `json:"runtimeVersion"`
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
// runPluginListCmd lists installed plugins, enriches them with runtime metadata, and renders the output to the provided command.
// 
// It checks whether the configured plugin directory exists, queries the plugin registry for installed plugins, fetches
// per-plugin metadata in parallel, sorts results by plugin name, and delegates rendering to displayPlugins.
// The function prints user-facing messages to cmd when the plugin directory is missing, no plugins are installed, or when
// installed plugins exist but none responded to metadata requests.
//
// Parameters:
//   - cmd: the Cobra command used for printing output and obtaining the request context.
//   - verbose: when true, instructs the renderer to include extended plugin information.
//
// Returns:
//   An error if listing plugins from the registry fails or if rendering the plugin list fails. Returns nil when the
//   operation completes successfully or when there are no plugins to display.
func runPluginListCmd(cmd *cobra.Command, verbose bool) error {
	cfg := config.New()
	if _, err := os.Stat(cfg.PluginDir); os.IsNotExist(err) {
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
		cmd.Println("No plugins found.")
		return nil
	}

	ctx := cmd.Context()
	enriched := fetchPluginMetadataParallel(ctx, plugins)

	if len(enriched) == 0 && len(plugins) > 0 {
		cmd.Println("No healthy plugins found (all failed to respond).")
		return nil
	}

	// Sort by plugin name for deterministic output
	sort.Slice(enriched, func(i, j int) bool {
		return enriched[i].Name < enriched[j].Name
	})

	return displayPlugins(cmd, enriched, verbose)
}

// fetchPluginMetadataParallel fetches metadata from all plugins concurrently.
// It uses errgroup with a concurrency limit of runtime.NumCPU() to prevent resource exhaustion.
// fetchPluginMetadataParallel fetches metadata for the provided plugins concurrently.
// ctx is the parent context used for per-plugin queries. plugins is the list of
// plugin entries to query.
// Plugin queries are performed in parallel with a bounded level of concurrency.
// Queries that fail or do not respond within their per-call timeout are skipped
// and do not cause the overall operation to fail.
// It returns a slice of enrichedPluginInfo for the plugins that responded successfully.
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
			if info != nil {
				mu.Lock()
				enriched = append(enriched, *info)
				mu.Unlock()
			}
			// Always return nil - we don't want one plugin failure to cancel others
			return nil
		})
	}

	// Wait for all goroutines to complete (errors are intentionally ignored)
	_ = g.Wait()

	return enriched
}

// fetchSinglePluginMetadata fetches metadata for a single plugin with timeout.
// fetchSinglePluginMetadata attempts to launch the given plugin and retrieve its spec and runtime versions.
// It uses a short per-call timeout and returns an enrichedPluginInfo containing the original PluginInfo
// plus SpecVersion and RuntimeVersion populated from the plugin metadata.
//
// Parameters:
//  - ctx: the parent context used for logging and cancellation.
//  - launcher: the plugin host launcher used to start the plugin process.
//  - plugin: the plugin registry information, including the executable path.
//
// Returns a pointer to enrichedPluginInfo with SpecVersion and RuntimeVersion set to the values
// reported by the plugin, or `nil` if the plugin could not be launched, did not respond, or timed out.
func fetchSinglePluginMetadata(
	ctx context.Context,
	launcher pluginhost.Launcher,
	plugin registry.PluginInfo,
) *enrichedPluginInfo {
	log := logging.FromContext(ctx)
	const launchTimeout = 5 * time.Second
	launchCtx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()

	client, launchErr := pluginhost.NewClient(launchCtx, launcher, plugin.Path)
	if launchErr != nil {
		log.Debug().
			Ctx(ctx).
			Str("plugin_path", plugin.Path).
			Err(launchErr).
			Msg("failed to launch plugin during list enumeration")
		return nil
	}
	defer func() { _ = client.Close() }()

	specVer := notAvailable
	runVer := notAvailable

	if client.Metadata != nil {
		specVer = client.Metadata.SpecVersion
		runVer = client.Metadata.Version
	}

	return &enrichedPluginInfo{
		PluginInfo:     plugin,
		SpecVersion:    specVer,
		RuntimeVersion: runVer,
	}
}

// displayPlugins writes the provided plugins list to the command's output using a tabular layout.
// It selects a verbose or simple column set based on the verbose flag.
//
// cmd is the Cobra command whose output writer will receive the table.
// plugins is the slice of enrichedPluginInfo to render.
// verbose controls whether verbose columns (including executable status) are shown.
//
// It returns any error encountered while writing or flushing the table.
func displayPlugins(cmd *cobra.Command, plugins []enrichedPluginInfo, verbose bool) error {
	const tabPadding = 2
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)

	if verbose {
		return displayVerbosePlugins(w, plugins)
	}
	return displaySimplePlugins(w, plugins)
}

func displayVerbosePlugins(w *tabwriter.Writer, plugins []enrichedPluginInfo) error {
	fmt.Fprintln(w, "Name\tVersion\tSpec\tPath\tExecutable")
	fmt.Fprintln(w, "----\t-------\t----\t----\t----------")

	for _, plugin := range plugins {
		execStatus := getExecutableStatus(plugin.Path)
		ver := plugin.displayVersion()

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", plugin.Name, ver, plugin.SpecVersion, plugin.Path, execStatus)
	}
	return w.Flush()
}

func displaySimplePlugins(w *tabwriter.Writer, plugins []enrichedPluginInfo) error {
	fmt.Fprintln(w, "Name\tVersion\tSpec\tPath")
	fmt.Fprintln(w, "----\t-------\t----\t----")

	for _, plugin := range plugins {
		ver := plugin.displayVersion()
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", plugin.Name, ver, plugin.SpecVersion, plugin.Path)
	}
	return w.Flush()
}

func getExecutableStatus(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "No"
	}
	if info.Mode()&0111 != 0 {
		return "Yes"
	}
	return "No"
}