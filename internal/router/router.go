package router

import (
	"context"
	"sort"
	"sync"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// Router selects appropriate plugins for resources based on provider matching,
// declarative patterns, and priority rules.
//
// Thread Safety: All methods are safe for concurrent use.
//
// Performance: SelectPlugins completes in <10ms per resource (SC-002).
type Router interface {
	// SelectPlugins returns plugins that match a resource for a given feature.
	//
	// Matching Logic (in order of precedence):
	// 1. Declarative patterns (if configured) - regex/glob matching
	// 2. Automatic provider matching - SupportedProviders metadata
	// 3. Global plugins - empty SupportedProviders or ["*"]
	//
	// Ordering:
	// - Results are ordered by priority (highest first)
	// - If all plugins have equal priority (0), all are returned for parallel query
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - resource: The resource to find plugins for
	//   - feature: The feature being requested (e.g., "ProjectedCosts")
	//
	// Returns:
	//   - []PluginMatch: Matching plugins with metadata, never nil (may be empty)
	SelectPlugins(ctx context.Context, resource engine.ResourceDescriptor, feature string) []PluginMatch

	// ShouldFallback returns true if fallback is enabled for a plugin.
	//
	// Parameters:
	//   - pluginName: Name of the plugin to check
	//
	// Returns:
	//   - bool: True if fallback is enabled (default: true)
	ShouldFallback(pluginName string) bool

	// Validate performs eager validation of the routing configuration.
	//
	// Validation includes:
	// - Plugin existence check (requires registry access)
	// - Pattern syntax validation (regex compilation)
	// - Feature name validation
	// - Duplicate pattern detection
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//
	// Returns:
	//   - ValidationResult: Contains errors (blocking) and warnings (non-blocking)
	Validate(ctx context.Context) ValidationResult
}

// PluginMatch represents a plugin that matches a resource.
type PluginMatch struct {
	// Client is the matched plugin client.
	// Never nil.
	Client *pluginhost.Client

	// Priority is the configured priority (0 if not configured).
	// Higher values = higher priority.
	Priority int

	// Fallback indicates if fallback is enabled for this plugin.
	// Default is true.
	Fallback bool

	// MatchReason describes why this plugin matched.
	MatchReason MatchReason

	// Source describes where the routing decision came from.
	// "automatic" for provider-based, "config" for declarative.
	Source string
}

// MatchReason describes how a plugin was matched to a resource.
type MatchReason int

const (
	// MatchReasonNoMatch indicates the plugin did not match the resource.
	MatchReasonNoMatch MatchReason = iota - 1

	// MatchReasonAutomatic means matched via SupportedProviders metadata.
	// The resource's provider (extracted from type) matched the plugin's
	// declared SupportedProviders list.
	MatchReasonAutomatic

	// MatchReasonPattern means matched via configured pattern.
	// A declarative glob or regex pattern matched the resource type.
	MatchReasonPattern

	// MatchReasonGlobal means plugin is global.
	// The plugin has empty SupportedProviders or ["*"].
	MatchReasonGlobal
)

// String returns the string representation of a MatchReason.
func (r MatchReason) String() string {
	switch r {
	case MatchReasonNoMatch:
		return "no_match"
	case MatchReasonAutomatic:
		return "automatic"
	case MatchReasonPattern:
		return "pattern"
	case MatchReasonGlobal:
		return "global"
	default:
		return "unknown"
	}
}

// DefaultRouter implements Router with automatic + declarative routing.
type DefaultRouter struct {
	// config is the routing configuration (may be nil for automatic-only).
	config *config.RoutingConfig

	// clients are all available plugin clients.
	clients []*pluginhost.Client

	// patterns is the compiled pattern cache.
	patterns map[string]*CompiledPattern

	// pluginConfig caches plugin routing config by name for fast lookup.
	pluginConfig map[string]*config.PluginRouting

	// mu protects patterns map.
	mu sync.RWMutex
}

// Option configures a Router.
type Option func(*DefaultRouter)

// WithConfig sets the routing configuration.
// WithConfig returns an Option that sets the router's routing configuration.
// If cfg is nil, the router will operate in automatic provider-based routing only.
func WithConfig(cfg *config.RoutingConfig) Option {
	return func(r *DefaultRouter) {
		r.config = cfg
	}
}

// WithClients sets the available plugin clients.
// WithClients returns an Option that injects the list of available plugin clients into a DefaultRouter.
// The provided clients slice is required for the router to function and replaces the router's current client list.
func WithClients(clients []*pluginhost.Client) Option {
	return func(r *DefaultRouter) {
		r.clients = clients
	}
}

// NewRouter creates a new Router with the given options.
//
// Example:
//
//	router, err := NewRouter(
//	    WithClients(clients),
//	    WithConfig(config.Routing),
//
// NewRouter creates a DefaultRouter configured by the provided options.
//
// The router is initialized with internal caches, builds a per-plugin routing
// lookup from any supplied routing config, and pre-compiles all declarative
// patterns found in that config for fast matching at runtime.
//
// opts are functional options (e.g., WithConfig, WithClients) used to provide
// the routing configuration and available plugin clients.
//
// The returned error is non-nil if any configured pattern fails to compile.
func NewRouter(opts ...Option) (*DefaultRouter, error) {
	r := &DefaultRouter{
		patterns:     make(map[string]*CompiledPattern),
		pluginConfig: make(map[string]*config.PluginRouting),
	}

	for _, opt := range opts {
		opt(r)
	}

	// Build plugin config lookup map
	if r.config != nil {
		for i := range r.config.Plugins {
			plugin := &r.config.Plugins[i]
			r.pluginConfig[plugin.Name] = plugin
		}
	}

	// Pre-compile all patterns
	if r.config != nil {
		for _, plugin := range r.config.Plugins {
			for _, pattern := range plugin.Patterns {
				compiled, err := CompilePattern(pattern)
				if err != nil {
					return nil, err
				}
				r.patterns[patternKey(plugin.Name, pattern)] = compiled
			}
		}
	}

	return r, nil
}

// compiled pattern caches and lookups.
func patternKey(pluginName string, pattern config.ResourcePattern) string {
	return pluginName + ":" + pattern.Type + ":" + pattern.Pattern
}

// SelectPlugins returns plugins that match a resource for a given feature.
//
//nolint:gocognit,nestif,funlen // Core routing logic requires checking multiple matching strategies.
func (r *DefaultRouter) SelectPlugins(
	ctx context.Context,
	resource engine.ResourceDescriptor,
	feature string,
) []PluginMatch {
	log := logging.FromContext(ctx)

	log.Debug().
		Ctx(ctx).
		Str("component", "router").
		Str("operation", "select_plugins").
		Str("resource_type", resource.Type).
		Str("feature", feature).
		Msg("selecting plugins for resource")

	provider := ExtractProviderFromType(resource.Type)
	resourceRegion := ExtractResourceRegion(resource)
	var matches []PluginMatch

	// First pass: check declarative patterns (highest precedence)
	if r.config != nil {
		for _, pluginCfg := range r.config.Plugins {
			// Check patterns if configured
			if len(pluginCfg.Patterns) > 0 {
				if r.matchesAnyPattern(pluginCfg, resource.Type) {
					client := r.findClient(pluginCfg.Name)
					if client != nil && r.matchesFeature(client, pluginCfg, feature) {
						// Check region compatibility
						pluginReg := PluginRegion(client)
						if !RegionMatches(pluginReg, resourceRegion) {
							log.Warn().
								Ctx(ctx).
								Str("component", "router").
								Str("plugin", pluginCfg.Name).
								Str("resource_type", resource.Type).
								Str("plugin_region", pluginReg).
								Str("resource_region", resourceRegion).
								Msg("plugin skipped: region mismatch")
							continue
						}
						matches = append(matches, PluginMatch{
							Client:      client,
							Priority:    pluginCfg.Priority,
							Fallback:    pluginCfg.FallbackEnabled(),
							MatchReason: MatchReasonPattern,
							Source:      "config",
						})
						log.Debug().
							Ctx(ctx).
							Str("component", "router").
							Str("plugin", pluginCfg.Name).
							Str("resource_type", resource.Type).
							Str("match_reason", "pattern").
							Int("priority", pluginCfg.Priority).
							Msg("plugin matched by pattern")
					}
				}
				continue // Don't fall through to automatic matching if patterns are configured
			}
		}
	}

	// Internal Pulumi types (pulumi:providers:*, pulumi:pulumi:*) should not be
	// routed to cost plugins via automatic matching. If a declarative pattern
	// already matched (first pass), honor it — a future "pulumi cost plugin"
	// can opt-in this way. Otherwise, skip the automatic pass entirely.
	if IsInternalPulumiType(resource.Type) {
		if len(matches) == 0 {
			log.Debug().
				Ctx(ctx).
				Str("component", "router").
				Str("resource_type", resource.Type).
				Msg("skipping internal Pulumi type (no declarative pattern match)")
		}
		sortByPriority(matches)
		return matches
	}

	// Second pass: automatic provider-based matching for plugins without patterns
	for _, client := range r.clients {
		// Skip if already matched by pattern
		if r.hasMatch(matches, client.Name) {
			continue
		}

		// Check if this client matches the provider
		matchReason := r.matchesProvider(client, provider)
		if matchReason == MatchReasonAutomatic || matchReason == MatchReasonGlobal {
			// Check region compatibility
			pluginReg := PluginRegion(client)
			if !RegionMatches(pluginReg, resourceRegion) {
				log.Warn().
					Ctx(ctx).
					Str("component", "router").
					Str("plugin", client.Name).
					Str("resource_type", resource.Type).
					Str("plugin_region", pluginReg).
					Str("resource_region", resourceRegion).
					Msg("plugin skipped: region mismatch (consider installing plugin for this region)")
				continue
			}

			// Check feature filter from config if exists
			if cfg, ok := r.pluginConfig[client.Name]; ok {
				if !r.matchesFeature(client, *cfg, feature) {
					continue
				}
			} else if !r.matchesFeature(client, config.PluginRouting{}, feature) {
				continue
			}

			priority := 0
			fallback := true
			if cfg, ok := r.pluginConfig[client.Name]; ok {
				priority = cfg.Priority
				fallback = cfg.FallbackEnabled()
			}

			matches = append(matches, PluginMatch{
				Client:      client,
				Priority:    priority,
				Fallback:    fallback,
				MatchReason: matchReason,
				Source:      "automatic",
			})
			log.Debug().
				Ctx(ctx).
				Str("component", "router").
				Str("plugin", client.Name).
				Str("resource_type", resource.Type).
				Str("provider", provider).
				Str("match_reason", matchReason.String()).
				Int("priority", priority).
				Msg("plugin matched by provider")
		}
	}

	// Warn if no plugins matched and we had region information
	if len(matches) == 0 && resourceRegion != "" {
		log.Warn().
			Ctx(ctx).
			Str("component", "router").
			Str("resource_type", resource.Type).
			Str("resource_region", resourceRegion).
			Str("feature", feature).
			Msg("no plugin matches resource region; install a region-specific plugin")
	}

	// Sort by priority (highest first)
	sortByPriority(matches)

	log.Debug().
		Ctx(ctx).
		Str("component", "router").
		Str("resource_type", resource.Type).
		Int("matched_count", len(matches)).
		Msg("plugin selection complete")

	return matches
}

// matchesFeature checks if a plugin configuration matches the requested feature.
func (r *DefaultRouter) matchesFeature(client *pluginhost.Client, cfg config.PluginRouting, feature string) bool {
	resolvedFeature := feature
	if parsedFeature, parsedOk := ParseFeature(feature); parsedOk {
		resolvedFeature = string(parsedFeature)
	} else if methodFeature, methodOk := FeatureFromMethod(feature); methodOk {
		resolvedFeature = string(methodFeature)
	}

	// If no features are configured, all features are allowed
	if len(cfg.Features) == 0 {
		return r.matchesCapabilities(client, resolvedFeature)
	}

	for _, f := range cfg.Features {
		if f == resolvedFeature {
			return r.matchesCapabilities(client, resolvedFeature)
		}
	}
	return false
}

func (r *DefaultRouter) matchesCapabilities(client *pluginhost.Client, feature string) bool {
	if client == nil || client.Metadata == nil {
		return true
	}

	capabilities := client.Metadata.Capabilities
	if len(capabilities) == 0 {
		return true
	}

	requestedFeature, ok := ParseFeature(feature)
	if !ok {
		if resolved, found := FeatureFromMethod(feature); found {
			requestedFeature = resolved
		} else {
			return true
		}
	}

	requiredCapability := capabilityNameForFeature(requestedFeature)
	if requiredCapability == "" {
		return true
	}

	capabilitySet := normalizedCapabilitySet(capabilities)
	_, ok = capabilitySet[requiredCapability]
	return ok
}

func capabilityNameForFeature(feature Feature) string {
	capability, ok := capabilityEnumFromFeature(feature)
	if !ok {
		return ""
	}
	converted := pluginhost.ConvertCapabilities([]pbc.PluginCapability{capability})
	if len(converted) == 0 {
		return ""
	}
	return converted[0]
}

func normalizedCapabilitySet(capabilities []string) map[string]struct{} {
	set := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if capability == "" {
			continue
		}
		set[capability] = struct{}{}
		if feature, ok := ParseFeature(capability); ok {
			if normalized := capabilityNameForFeature(feature); normalized != "" {
				set[normalized] = struct{}{}
			}
		}
		if enumValue, ok := capabilityEnumFromString(capability); ok {
			for _, normalized := range pluginhost.ConvertCapabilities([]pbc.PluginCapability{enumValue}) {
				set[normalized] = struct{}{}
			}
		}
	}
	return set
}

func capabilityEnumFromFeature(feature Feature) (pbc.PluginCapability, bool) {
	switch feature {
	case FeatureProjectedCosts:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS, true
	case FeatureActualCosts:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS, true
	case FeatureRecommendations:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS, true
	case FeatureCarbon:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_CARBON, true
	case FeatureDryRun:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN, true
	case FeatureBudgets:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_BUDGETS, true
	default:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED, false
	}
}

func capabilityEnumFromString(capability string) (pbc.PluginCapability, bool) {
	switch capability {
	case "ProjectedCosts", "projected_costs":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS, true
	case "ActualCosts", "actual_costs":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS, true
	case "Recommendations", "recommendations":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS, true
	case "Carbon", "carbon":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_CARBON, true
	case "DryRun", "dry_run":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN, true
	case "Budgets", "budgets":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_BUDGETS, true
	default:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED, false
	}
}

// matchesAnyPattern checks if the resource type matches any of the plugin's patterns.
func (r *DefaultRouter) matchesAnyPattern(cfg config.PluginRouting, resourceType string) bool {
	for _, pattern := range cfg.Patterns {
		key := patternKey(cfg.Name, pattern)
		r.mu.RLock()
		compiled, ok := r.patterns[key]
		r.mu.RUnlock()

		if !ok {
			// Pattern wasn't compiled, try to compile it now
			var err error
			compiled, err = CompilePattern(pattern)
			if err != nil {
				continue
			}
			r.mu.Lock()
			// Double-check: another goroutine may have compiled while we waited
			if existing, exists := r.patterns[key]; exists {
				r.mu.Unlock()
				compiled = existing
			} else {
				r.patterns[key] = compiled
				r.mu.Unlock()
			}
		}

		matched, err := compiled.Match(resourceType)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// matchesProvider checks if a client matches the resource provider.
func (r *DefaultRouter) matchesProvider(client *pluginhost.Client, provider string) MatchReason {
	if client.Metadata == nil {
		return MatchReasonGlobal // No metadata, treat as global
	}

	supportedProviders := client.Metadata.SupportedProviders
	if len(supportedProviders) == 0 {
		return MatchReasonGlobal // Empty providers, treat as global
	}

	for _, supported := range supportedProviders {
		if ProviderMatches(provider, supported) {
			if IsGlobalProvider(supported) {
				return MatchReasonGlobal
			}
			return MatchReasonAutomatic
		}
	}

	return MatchReasonNoMatch
}

// findClient finds a client by name.
func (r *DefaultRouter) findClient(name string) *pluginhost.Client {
	for _, client := range r.clients {
		if client.Name == name {
			return client
		}
	}
	return nil
}

// hasMatch checks if a plugin name is already in the matches list.
func (r *DefaultRouter) hasMatch(matches []PluginMatch, name string) bool {
	for _, m := range matches {
		if m.Client.Name == name {
			return true
		}
	}
	return false
}

// ShouldFallback returns true if fallback is enabled for a plugin.
func (r *DefaultRouter) ShouldFallback(pluginName string) bool {
	if cfg, ok := r.pluginConfig[pluginName]; ok {
		return cfg.FallbackEnabled()
	}
	return true // Default to true
}

// Validate performs eager validation of the routing configuration.
func (r *DefaultRouter) Validate(_ context.Context) ValidationResult {
	return ValidateRoutingConfig(r.config, r.clients)
}

// sortByPriority sorts matches by priority (highest first).
// sortByPriority sorts the provided slice of PluginMatch in-place by Priority in
// descending order. It uses a stable sort, so elements with equal Priority retain
// their relative order.
func sortByPriority(matches []PluginMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Priority > matches[j].Priority
	})
}

// AllEqualPriority returns true if all matches have the same priority.
// AllEqualPriority reports whether all PluginMatch entries in the slice have the same Priority.
// It returns true for slices with zero or one element, or when every element's Priority equals the first element's Priority; otherwise it returns false.
func AllEqualPriority(matches []PluginMatch) bool {
	if len(matches) <= 1 {
		return true
	}
	first := matches[0].Priority
	for _, m := range matches[1:] {
		if m.Priority != first {
			return false
		}
	}
	return true
}
