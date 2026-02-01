package config

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// RoutingConfig defines the complete routing strategy for plugins.
//
// YAML Location: ~/.finfocus/config.yaml under "routing" key
//
// Example:
//
//	routing:
//	  plugins:
//	    - name: aws-public
//	      priority: 10
type RoutingConfig struct {
	// Plugins contains the ordered list of plugin routing rules.
	// Order matters for tie-breaking when priorities are equal.
	// May be empty (uses automatic routing only).
	Plugins []PluginRouting `yaml:"plugins" json:"plugins"`
}

// PluginRouting defines how a specific plugin should be used.
type PluginRouting struct {
	// Name is the plugin identifier.
	// Must match an installed plugin name from ~/.finfocus/plugins/<name>/.
	// Required.
	Name string `yaml:"name" json:"name"`

	// Features limits which capabilities this plugin handles.
	// If empty, all features the plugin reports are enabled.
	//
	// Valid values:
	//   - ProjectedCosts: Cost estimation from infrastructure specs
	//   - ActualCosts: Historical cost data from cloud APIs
	//   - Recommendations: Cost optimization suggestions
	//   - Carbon: Carbon footprint estimation
	//   - DryRun: Dry run simulation
	//   - Budgets: Budget tracking and alerts
	//
	// Invalid feature names generate a validation warning (non-blocking).
	Features []string `yaml:"features,omitempty" json:"features,omitempty"`

	// Patterns defines resource type patterns this plugin handles.
	// If empty, automatic provider-based routing is used.
	// Patterns take precedence over automatic routing.
	Patterns []ResourcePattern `yaml:"patterns,omitempty" json:"patterns,omitempty"`

	// Priority determines selection order.
	// Higher values = higher priority (preferred).
	// Default is 0.
	//
	// Behavior:
	//   - Different priorities: Highest priority plugin is tried first
	//   - Equal priority (0): All matching plugins queried in parallel
	//   - Fallback: If enabled and plugin fails, next priority is tried
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Fallback enables trying the next plugin if this one fails.
	// Default is true if not specified.
	//
	// Failure conditions that trigger fallback:
	//   - Connection timeout
	//   - Plugin crash (EOF, connection reset)
	//   - Empty result (no cost data)
	//   - gRPC error (Unavailable, Internal)
	//
	// Conditions that do NOT trigger fallback:
	//   - InvalidArgument (plugin explicitly rejected request)
	//   - $0 cost result (valid result, not a failure)
	Fallback *bool `yaml:"fallback,omitempty" json:"fallback,omitempty"`
}

// FallbackEnabled returns whether fallback is enabled for this plugin.
// Returns true if Fallback is nil (default behavior).
func (p PluginRouting) FallbackEnabled() bool {
	if p.Fallback == nil {
		return true
	}
	return *p.Fallback
}

// ResourcePattern defines a pattern for matching resource types.
type ResourcePattern struct {
	// Type is the pattern type.
	// Required. Must be "glob" or "regex".
	//
	// "glob": Uses Go's filepath.Match semantics
	//   - "*" matches any sequence of non-separator characters
	//   - "?" matches any single non-separator character
	//   - "[...]" matches character class
	//   - Example: "aws:ec2:*" matches "aws:ec2:Instance"
	//
	// "regex": Uses Go's regexp package (RE2 syntax)
	//   - Full regular expression support
	//   - Must be valid RE2 syntax (no backreferences)
	//   - Example: "aws:(ec2|rds)/.*" matches "aws:ec2/instance:Instance"
	Type string `yaml:"type" json:"type"`

	// Pattern is the pattern string.
	// Required. Must be non-empty.
	// Validated at config load time.
	Pattern string `yaml:"pattern" json:"pattern"`
}

// PatternTypeGlob is the pattern type for glob matching.
const PatternTypeGlob = "glob"

// PatternTypeRegex is the pattern type for regex matching.
const PatternTypeRegex = "regex"

// IsGlob returns true if this is a glob pattern.
func (p ResourcePattern) IsGlob() bool {
	return p.Type == PatternTypeGlob
}

// IsRegex returns true if this is a regex pattern.
func (p ResourcePattern) IsRegex() bool {
	return p.Type == PatternTypeRegex
}

// Validate performs lightweight structural validation of the routing configuration.
// It checks that plugin names are non-empty, patterns have valid types and non-empty strings,
// and priority values are non-negative.
func (r *RoutingConfig) Validate() error {
	if r == nil {
		return nil
	}

	for i, plugin := range r.Plugins {
		// Validate plugin name is present
		if plugin.Name == "" {
			return fmt.Errorf("plugin at index %d: name is required", i)
		}

		// Validate priority is non-negative
		if plugin.Priority < 0 {
			return fmt.Errorf("plugin %q: priority must be non-negative, got %d", plugin.Name, plugin.Priority)
		}

		// Validate patterns
		for j, pattern := range plugin.Patterns {
			if err := validatePattern(plugin.Name, j, pattern); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePattern validates a single resource pattern.
func validatePattern(pluginName string, index int, pattern ResourcePattern) error {
	if pattern.Pattern == "" {
		return fmt.Errorf("plugin %q: pattern at index %d: pattern string is required", pluginName, index)
	}

	switch pattern.Type {
	case PatternTypeRegex:
		if _, err := regexp.Compile(pattern.Pattern); err != nil {
			return fmt.Errorf("plugin %q: pattern at index %d: invalid regex %q: %w",
				pluginName, index, pattern.Pattern, err)
		}
	case PatternTypeGlob:
		if _, err := filepath.Match(pattern.Pattern, ""); err != nil {
			return fmt.Errorf("plugin %q: pattern at index %d: invalid glob %q: %w",
				pluginName, index, pattern.Pattern, err)
		}
	default:
		return fmt.Errorf("plugin %q: pattern at index %d: invalid type %q (must be %q or %q)",
			pluginName, index, pattern.Type, PatternTypeGlob, PatternTypeRegex)
	}
	return nil
}
