package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/sync/errgroup"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// maxConcurrentPluginOpens bounds how many plugin processes Open starts at once.
const maxConcurrentPluginOpens = 8

// Registry manages plugin discovery and lifecycle operations.
// It scans plugin directories and provides client connections to active plugins.
type Registry struct {
	root     string
	launcher pluginhost.Launcher
}

// New creates a new Registry rooted at the given plugin directory,
// using ProcessLauncher for plugin execution.
func New(root string) *Registry {
	return &Registry{
		root:     root,
		launcher: pluginhost.NewProcessLauncher(),
	}
}

// NewDefault creates a new Registry with default configuration from config.PluginDir
// and using ProcessLauncher for plugin execution.
func NewDefault() *Registry {
	return New(config.New().PluginDir)
}

// ListPlugins scans the plugin directory and returns metadata for all discovered plugins.
// It returns an empty list if the plugin directory doesn't exist.
//
//nolint:gocognit // Filesystem traversal with platform-specific binary detection requires multiple conditions.
func (r *Registry) ListPlugins() ([]PluginInfo, error) {
	var plugins []PluginInfo

	if _, err := os.Stat(r.root); os.IsNotExist(err) {
		return plugins, nil
	}

	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil, fmt.Errorf("reading plugin directory: %w", err)
	}

	for _, entry := range entries {
		pluginPath := filepath.Join(r.root, entry.Name())
		// Use os.Stat (not entry.IsDir) so symlinks-to-directories are followed (#750).
		entryInfo, entryStatErr := os.Stat(pluginPath)
		if entryStatErr != nil || !entryInfo.IsDir() {
			continue
		}

		versions, versionErr := os.ReadDir(pluginPath)
		if versionErr != nil {
			continue
		}

		for _, version := range versions {
			versionPath := filepath.Join(pluginPath, version.Name())
			// Use os.Stat (not version.IsDir) so symlinks-to-directories are followed (#750).
			versionInfo, versionStatErr := os.Stat(versionPath)
			if versionStatErr != nil || !versionInfo.IsDir() {
				continue
			}

			// Read metadata once and pass to findBinary to avoid duplicate reads.
			meta, _ := ReadPluginMetadata(versionPath)
			binPath := r.findBinary(versionPath, meta)
			if binPath != "" {
				// Enrich metadata with region from binary name if not already set.
				if region, ok := ParseRegionFromBinaryName(binPath); ok {
					if meta == nil {
						meta = map[string]string{"region": region}
					} else if _, hasRegion := meta["region"]; !hasRegion {
						meta["region"] = region
					}
				}
				plugins = append(plugins, PluginInfo{
					Name:     entry.Name(),
					Version:  version.Name(),
					Path:     binPath,
					Metadata: meta,
				})
			}
		}
	}

	return plugins, nil
}

// ListLatestPlugins scans the plugin directory and returns only the latest version
// of each plugin. Plugins with same name in different locations are treated as
// duplicates and the latest version across all locations is selected.
// Results are sorted by plugin name. Returns warnings for invalid or corrupted plugins.
func (r *Registry) ListLatestPlugins() ([]PluginInfo, []string, error) {
	allPlugins, err := r.ListPlugins()
	if err != nil {
		return nil, nil, err
	}

	latest := make(map[string]PluginInfo)
	var warnings []string

	for _, plugin := range allPlugins {
		v, verErr := semver.NewVersion(plugin.Version)
		if verErr != nil {
			warnings = append(warnings,
				fmt.Sprintf("Plugin %s version %s has invalid semver format: %v",
					plugin.Name, plugin.Version, verErr))
			continue
		}

		existing, ok := latest[plugin.Name]
		if !ok {
			latest[plugin.Name] = plugin
			continue
		}

		vExisting, _ := semver.NewVersion(existing.Version)
		if v.GreaterThan(vExisting) {
			latest[plugin.Name] = plugin
		}
	}

	result := make([]PluginInfo, 0, len(latest))
	for _, plugin := range latest {
		result = append(result, plugin)
	}
	slices.SortFunc(result, func(a, b PluginInfo) int {
		return strings.Compare(a.Name, b.Name)
	})

	return result, warnings, nil
}

// GetLatestPlugin returns the latest version of a specific plugin.
// Returns (PluginInfo{}, false, warnings) if plugin not found or all versions are invalid.
func (r *Registry) GetLatestPlugin(name string) (PluginInfo, bool, []string, error) {
	plugins, warnings, err := r.ListLatestPlugins()
	if err != nil {
		return PluginInfo{}, false, warnings, err
	}

	for _, plugin := range plugins {
		if plugin.Name == name {
			return plugin, true, warnings, nil
		}
	}

	return PluginInfo{}, false, warnings, nil
}

// isExecutable checks if the given file info represents an executable file.
func isExecutable(path string, info os.FileInfo) bool {
	if runtime.GOOS == osWindows {
		return filepath.Ext(path) == extExe
	}
	return info.Mode()&0111 != 0
}

// findByPattern attempts to find a binary matching the given pattern in dir.
func findByPattern(dir, pattern string) string {
	path := filepath.Join(dir, pattern)
	if runtime.GOOS == osWindows {
		path += extExe
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}

	if isExecutable(path, info) {
		return path
	}
	return ""
}

// buildPluginPatterns returns the list of patterns to search for a plugin binary.
func buildPluginPatterns(pluginName string) []string {
	patterns := []string{
		"finfocus-plugin-" + pluginName,
		pluginName,
	}
	if os.Getenv("FINFOCUS_LOG_LEGACY") == "1" {
		patterns = append(patterns, "pulumicost-plugin-"+pluginName)
	}
	return patterns
}

func (r *Registry) findBinary(dir string, meta map[string]string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	pluginName := filepath.Base(filepath.Dir(dir))

	// If metadata specifies a region, try region-specific binary first.
	// This ensures that "plugin install aws-public --metadata=region=us-west-2"
	// selects the us-west-2 binary instead of the first one alphabetically.
	if region := meta["region"]; region != "" {
		regionBinPattern := "finfocus-plugin-" + pluginName + "-" + region
		if path := findByPattern(dir, regionBinPattern); path != "" {
			return path
		}
	}

	// Try to find by name patterns
	for _, pattern := range buildPluginPatterns(pluginName) {
		if path := findByPattern(dir, pattern); path != "" {
			return path
		}
	}

	// Fallback: search for ANY executable in the directory
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}

		if isExecutable(path, info) {
			return path
		}
	}

	return ""
}

// Open launches plugin processes and returns active gRPC clients with a cleanup function.
// If onlyName is non-empty and is an installed plugin's install name or catalog
// alias, only that plugin is opened. Any other non-empty onlyName may be a name
// a plugin reports, which is unknown until it runs, so every plugin is opened
// and the caller filters with pluginhost.Client.MatchesName.
func (r *Registry) Open(
	ctx context.Context,
	onlyName string,
) ([]*pluginhost.Client, func(), error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "registry").
		Str("operation", "open_plugins").
		Str("plugin_filter", onlyName).
		Str("plugin_root", r.root).
		Msg("opening plugins")

	plugins, warnings, err := r.ListLatestPlugins()
	if err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "registry").
			Err(err).
			Msg("failed to list plugins")
		return nil, nil, err
	}

	for _, warning := range warnings {
		log.Warn().
			Ctx(ctx).
			Str("component", "registry").
			Str("warning", warning).
			Msg("plugin warning")
	}

	filteredPlugins := filterByInstallName(plugins, onlyName)
	matchReported := onlyName != "" && len(filteredPlugins) == 0
	if matchReported {
		log.Debug().
			Ctx(ctx).
			Str("component", "registry").
			Str("plugin_filter", onlyName).
			Msg("plugin filter names no install name or alias; opening every plugin to match reported names")
		filteredPlugins = plugins
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "registry").
		Int("discovered_plugins", len(filteredPlugins)).
		Msg("latest plugins discovered after filtering")

	clients := r.openPlugins(ctx, filteredPlugins)
	if matchReported {
		clients = keepReportedMatches(clients, onlyName)
	}
	cleanup := func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}

	log.Info().
		Ctx(ctx).
		Str("component", "registry").
		Int("connected_plugins", len(clients)).
		Msg("plugin discovery complete")

	return clients, cleanup, nil
}

// openPlugins launches plugins concurrently, at most maxConcurrentPluginOpens at a
// time, and returns the clients that connected in the order of plugins.
func (r *Registry) openPlugins(ctx context.Context, plugins []PluginInfo) []*pluginhost.Client {
	opened := make([]*pluginhost.Client, len(plugins))
	var g errgroup.Group
	g.SetLimit(maxConcurrentPluginOpens)
	for i, plugin := range plugins {
		g.Go(func() error {
			opened[i] = r.openPlugin(ctx, plugin)
			return nil
		})
	}
	_ = g.Wait()

	var clients []*pluginhost.Client
	for _, client := range opened {
		if client != nil {
			clients = append(clients, client)
		}
	}
	return clients
}

// openPlugin connects to a single plugin, returning nil if it fails to start.
// Under a context that carries a pluginhost.Pool, the process is shared with
// every other open that resolves the same binary and environment.
func (r *Registry) openPlugin(ctx context.Context, plugin PluginInfo) *pluginhost.Client {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "registry").
		Str("plugin_name", plugin.Name).
		Str("plugin_version", plugin.Version).
		Str("plugin_path", plugin.Path).
		Msg("attempting to connect to plugin")

	open := pluginhost.NewClient
	if pool, ok := pluginhost.PoolFromContext(ctx); ok {
		open = pool.Acquire
	}
	client, err := open(pluginhost.WithInstallName(ctx, plugin.Name), r.launcher, plugin.Path)
	if err != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "registry").
			Str("plugin_name", plugin.Name).
			Str("plugin_path", plugin.Path).
			Err(err).
			Msg("failed to connect to plugin")
		return nil
	}

	client.Aliases = CatalogAliases(plugin.Name)
	warnNameMismatch(ctx, client)

	// Merge registry-level metadata (e.g., region) into the client's plugin metadata.
	// This ensures routing decisions can use registry metadata even if the plugin
	// didn't report it via GetPluginInfo.
	mergeRegistryMetadata(client, plugin)

	log.Debug().
		Ctx(ctx).
		Str("component", "registry").
		Str("operation", "plugin_connected").
		Str("plugin_name", plugin.Name).
		Str("plugin_version", plugin.Version).
		Str("region", plugin.Region()).
		Msg("plugin connected successfully")

	return client
}

// nameMismatchWarned records the install|reported pairs already warned about,
// so a dashboard that reopens plugins, or several stacks sharing them, warns
// once per process.
var nameMismatchWarned sync.Map //nolint:gochecknoglobals // once-per-process warning set

// warnNameMismatch warns once per process when a plugin reports a name other
// than its install name. Both names are accepted; the plugin should report
// its install name.
func warnNameMismatch(ctx context.Context, client *pluginhost.Client) {
	if client.InstallName == "" || client.Name == client.InstallName {
		return
	}
	if _, seen := nameMismatchWarned.LoadOrStore(client.InstallName+"|"+client.Name, struct{}{}); seen {
		return
	}
	logging.FromContext(ctx).Warn().
		Ctx(ctx).
		Str("component", "registry").
		Str("install_name", client.InstallName).
		Str("reported_name", client.Name).
		Msg("plugin reports a name that differs from its install name; both are accepted")
}

// ResetNameMismatchWarningsForTest clears the once-per-process warning set.
// It exists for tests only; tests that call it must not run in parallel.
func ResetNameMismatchWarningsForTest() {
	nameMismatchWarned.Clear()
}

// keepReportedMatches keeps the clients that report name and closes the rest,
// so a filter that only a running plugin can answer still returns just that
// plugin.
func keepReportedMatches(clients []*pluginhost.Client, name string) []*pluginhost.Client {
	kept := clients[:0]
	for _, c := range clients {
		if c.MatchesName(name) {
			kept = append(kept, c)
			continue
		}
		_ = c.Close()
	}
	return kept
}

// filterByInstallName keeps the plugins whose install name is name, or else
// those whose catalog aliases include it. An empty name keeps every plugin.
func filterByInstallName(plugins []PluginInfo, name string) []PluginInfo {
	if name == "" {
		return plugins
	}
	var byInstall, byAlias []PluginInfo
	for _, plugin := range plugins {
		switch {
		case plugin.Name == name:
			byInstall = append(byInstall, plugin)
		case slices.Contains(CatalogAliases(plugin.Name), name):
			byAlias = append(byAlias, plugin)
		}
	}
	if len(byInstall) > 0 {
		return byInstall
	}
	return byAlias
}

// PluginInfo contains metadata about a discovered plugin.
type PluginInfo struct {
	Name     string            `json:"name"`
	Version  string            `json:"version"`
	Path     string            `json:"path"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Region returns the plugin's region from metadata, or empty string if universal.
func (p PluginInfo) Region() string {
	if p.Metadata == nil {
		return ""
	}
	return p.Metadata["region"]
}

// mergeRegistryMetadata injects metadata discovered by the registry into a plugin
// client's metadata without overwriting keys the plugin already provides.
// If the registry has no metadata for the plugin this is a no-op.
// The function ensures the client's PluginMetadata and its Metadata map are
// initialized before copying keys from the registry's PluginInfo.Metadata.
func mergeRegistryMetadata(client *pluginhost.Client, plugin PluginInfo) {
	if len(plugin.Metadata) == 0 {
		return
	}

	if client.Metadata == nil {
		client.Metadata = &proto.PluginMetadata{
			Name: client.Name,
		}
	}

	if client.Metadata.Metadata == nil {
		client.Metadata.Metadata = make(map[string]string, len(plugin.Metadata))
	}

	// Only set keys that aren't already reported by the plugin itself
	for k, v := range plugin.Metadata {
		if _, exists := client.Metadata.Metadata[k]; !exists {
			client.Metadata.Metadata[k] = v
		}
	}
}
