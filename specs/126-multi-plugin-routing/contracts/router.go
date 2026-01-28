// Package contracts defines the internal API contracts for multi-plugin routing.
// This file serves as a specification for implementation - it is NOT production code.
//
// Location: specs/126-multi-plugin-routing/contracts/router.go
// Implementation: internal/router/router.go
package contracts

import (
	"context"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// Router selects appropriate plugins for resources based on provider matching,
// declarative patterns, and priority rules.
//
// Thread Safety: All methods must be safe for concurrent use.
//
// Performance: SelectPlugins must complete in <10ms per resource (SC-002).
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
}

// MatchReason describes how a plugin was matched to a resource.
type MatchReason int

const (
	// MatchReasonAutomatic means matched via SupportedProviders metadata.
	// The resource's provider (extracted from type) matched the plugin's
	// declared SupportedProviders list.
	MatchReasonAutomatic MatchReason = iota

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

// ValidationResult contains the results of configuration validation.
type ValidationResult struct {
	// Valid is true if no errors were found.
	// Warnings do not affect validity.
	Valid bool

	// Errors are blocking issues that prevent routing.
	// If len(Errors) > 0, Valid must be false.
	Errors []ValidationError

	// Warnings are non-blocking issues that should be reviewed.
	// Routing will still work, but behavior may be unexpected.
	Warnings []ValidationWarning
}

// ValidationError represents a blocking validation error.
type ValidationError struct {
	// Plugin is the plugin name (empty for global errors).
	Plugin string

	// Field is the configuration field with the error.
	// Examples: "name", "patterns[0].pattern", "features"
	Field string

	// Message describes the error.
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.Plugin != "" {
		return e.Plugin + "." + e.Field + ": " + e.Message
	}
	return e.Field + ": " + e.Message
}

// ValidationWarning represents a non-blocking warning.
type ValidationWarning struct {
	// Plugin is the plugin name (empty for global warnings).
	Plugin string

	// Field is the configuration field with the warning.
	Field string

	// Message describes the warning.
	Message string
}

// Option configures a Router.
type Option func(*routerOptions)

type routerOptions struct {
	config  *RoutingConfig
	clients []*pluginhost.Client
}

// WithConfig sets the routing configuration.
// If not provided, automatic routing only is used.
func WithConfig(config *RoutingConfig) Option {
	return func(o *routerOptions) {
		o.config = config
	}
}

// WithClients sets the available plugin clients.
// Required - router cannot function without clients.
func WithClients(clients []*pluginhost.Client) Option {
	return func(o *routerOptions) {
		o.clients = clients
	}
}

// NewRouter creates a new Router with the given options.
//
// Example:
//
//	router := NewRouter(
//	    WithClients(clients),
//	    WithConfig(config.Routing),
//	)
func NewRouter(_ ...Option) (Router, error) {
	// Implementation in internal/router/router.go
	panic("contract only - see internal/router/router.go for implementation")
}
