package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Masterminds/semver/v3"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// Registry manages plugin discovery and lifecycle operations.
// It scans plugin directories and provides client connections to active plugins.
type Registry struct {
	root     string
	launcher pluginhost.Launcher
}

// NewDefault creates a new Registry with default configuration from config.PluginDir
// and using ProcessLauncher for plugin execution.
func NewDefault() *Registry {
	cfg := config.New()
	return &Registry{
		root:     cfg.PluginDir,
		launcher: pluginhost.NewProcessLauncher(),
	}
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
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(r.root, entry.Name())
		versions, versionErr := os.ReadDir(pluginPath)
		if versionErr != nil {
			continue
		}

		for _, version := range versions {
			if !version.IsDir() {
				continue
			}

			versionPath := filepath.Join(pluginPath, version.Name())
			// Read metadata once and pass to findBinary to avoid duplicate reads.
			meta, _ := ReadPluginMetadata(versionPath)
			binPath := r.findBinary(versionPath, meta)
			if binPath != "" {
				// Enrich metadata with region from binary name if not already set.
				if meta == nil {
					if region, ok := ParseRegionFromBinaryName(binPath); ok {
						meta = map[string]string{"region": region}
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
// Returns warnings for invalid or corrupted plugins.
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
// If onlyName is non-empty, only that specific plugin is opened.
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

	var filteredPlugins []PluginInfo
	for _, plugin := range plugins {
		if onlyName == "" || plugin.Name == onlyName {
			filteredPlugins = append(filteredPlugins, plugin)
		}
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "registry").
		Int("discovered_plugins", len(filteredPlugins)).
		Msg("latest plugins discovered after filtering")

	var clients []*pluginhost.Client
	cleanup := func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}

	for _, plugin := range filteredPlugins {
		log.Debug().
			Ctx(ctx).
			Str("component", "registry").
			Str("plugin_name", plugin.Name).
			Str("plugin_version", plugin.Version).
			Str("plugin_path", plugin.Path).
			Msg("attempting to connect to plugin")

		client, clientErr := pluginhost.NewClient(ctx, r.launcher, plugin.Path)
		if clientErr != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "registry").
				Str("plugin_name", plugin.Name).
				Str("plugin_path", plugin.Path).
				Err(clientErr).
				Msg("failed to connect to plugin")
			continue
		}

		// Merge registry-level metadata (e.g., region) into the client's plugin metadata.
		// This ensures routing decisions can use registry metadata even if the plugin
		// didn't report it via GetPluginInfo.
		mergeRegistryMetadata(client, plugin)

		log.Debug().
			Ctx(ctx).
			Str("component", "registry").
			Str("plugin_name", plugin.Name).
			Str("plugin_version", plugin.Version).
			Str("region", plugin.Region()).
			Msg("plugin connected successfully")

		clients = append(clients, client)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "registry").
		Int("connected_plugins", len(clients)).
		Msg("plugin discovery complete")

	return clients, cleanup, nil
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
