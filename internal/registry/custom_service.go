package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rshade/finfocus/internal/config"
)

// CustomPlugin identifies a catalog entry and its global registry source.
type CustomPlugin struct {
	Source config.PluginRegistrySource `json:"source"`
	Entry  RegistryEntry               `json:"entry"`
}

// Revision binds an install plan to the source and resolved entry.
func (p CustomPlugin) Revision() string {
	raw, _ := json.Marshal(p) // CustomPlugin contains only JSON-compatible data.
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// ErrCustomRegistryConfirmation indicates that the registry name must be typed.
var ErrCustomRegistryConfirmation = errors.New("type the registry name to confirm this source")

// ErrCustomPluginNotFound identifies a missing, shadowed, or ambiguous entry.
var ErrCustomPluginNotFound = errors.New("plugin unavailable from the selected registry")

// CustomRegistryService coordinates source management, browsing, and installs.
type CustomRegistryService struct {
	index *CustomIndexClient
}

// NewCustomRegistryService uses the given index client or a default HTTPS client.
func NewCustomRegistryService(index *CustomIndexClient) *CustomRegistryService {
	if index == nil {
		index = NewCustomIndexClient(nil)
	}
	return &CustomRegistryService{index: index}
}

// List returns the registry sources from global configuration.
func (s *CustomRegistryService) List(ctx context.Context) ([]config.PluginRegistrySource, error) {
	return config.GlobalPluginRegistries(ctx)
}

// Add checks an index before recording the source in global configuration.
func (s *CustomRegistryService) Add(ctx context.Context, source config.PluginRegistrySource) error {
	if _, err := s.index.Fetch(ctx, source); err != nil {
		return err
	}
	return config.AddPluginRegistry(ctx, source)
}

// Remove deletes a source and its confirmation.
func (s *CustomRegistryService) Remove(ctx context.Context, name string) error {
	return config.RemovePluginRegistry(ctx, name)
}

// Catalog serves unambiguous custom entries without shadowing built-in names.
// Valid entries remain available when another source cannot be loaded.
func (s *CustomRegistryService) Catalog(ctx context.Context) ([]CustomPlugin, error) {
	sources, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var available []CustomPlugin
	var sourceErrors []error
	for _, source := range sources {
		entries, fetchErr := s.index.Fetch(ctx, source)
		if fetchErr != nil {
			sourceErrors = append(sourceErrors, fmt.Errorf("registry %s: %w", source.Name, fetchErr))
			continue
		}
		for _, entry := range entries {
			available = append(available, CustomPlugin{Source: source, Entry: entry})
		}
	}
	plugins, catalogErr := filterCustomPlugins(available)
	return plugins, errors.Join(errors.Join(sourceErrors...), catalogErr)
}

func filterCustomPlugins(available []CustomPlugin) ([]CustomPlugin, error) {
	reserved, err := builtinNames()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(available))
	for _, plugin := range available {
		counts[plugin.Entry.Name]++
	}
	filtered := make([]CustomPlugin, 0, len(available))
	for _, plugin := range available {
		if _, exists := reserved[plugin.Entry.Name]; !exists && counts[plugin.Entry.Name] == 1 {
			filtered = append(filtered, plugin)
		}
	}
	slices.SortFunc(filtered, func(a, b CustomPlugin) int { return strings.Compare(a.Entry.Name, b.Entry.Name) })
	return filtered, nil
}

// builtinNames returns every name the built-in catalog claims: each entry's
// name and its aliases (former names). A custom entry may use none of them,
// or routing and --adapter could read the name as two plugins.
func builtinNames() (map[string]struct{}, error) {
	entries, err := GetAllPluginEntries()
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		names[entry.Name] = struct{}{}
		for _, alias := range entry.Aliases {
			names[alias] = struct{}{}
		}
	}
	return names, nil
}

// Resolve fetches a source-qualified entry using the current configured source.
func (s *CustomRegistryService) Resolve(ctx context.Context, source, name string) (CustomPlugin, error) {
	reserved, err := builtinNames()
	if err != nil {
		return CustomPlugin{}, err
	}
	if _, exists := reserved[name]; exists {
		return CustomPlugin{}, ErrCustomPluginNotFound
	}
	sources, err := s.List(ctx)
	if err != nil {
		return CustomPlugin{}, err
	}
	for _, candidate := range sources {
		if candidate.Name != source {
			continue
		}
		entries, fetchErr := s.index.Fetch(ctx, candidate)
		if fetchErr != nil {
			return CustomPlugin{}, fetchErr
		}
		for _, entry := range entries {
			if entry.Name == name {
				return CustomPlugin{Source: candidate, Entry: entry}, nil
			}
		}
		break
	}
	return CustomPlugin{}, ErrCustomPluginNotFound
}

// Install authorizes the resolved source and invokes the strict installer.
func (s *CustomRegistryService) Install(
	ctx context.Context, installer *Installer, plugin CustomPlugin, version, confirmation string,
	opts InstallOptions, progress func(string),
) (*InstallResult, error) {
	confirmed, err := config.PluginRegistryConfirmed(ctx, plugin.Source)
	if err != nil {
		return nil, err
	}
	if !confirmed {
		if confirmation != plugin.Source.Name {
			return nil, ErrCustomRegistryConfirmation
		}
		if confirmErr := config.ConfirmPluginRegistry(ctx, plugin.Source); confirmErr != nil {
			return nil, confirmErr
		}
	}
	return installer.installCustom(ctx, plugin, version, opts, progress)
}
