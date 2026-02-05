package router

import (
	"fmt"
	"regexp"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// ValidationResult contains the results of configuration validation.
type ValidationResult struct {
	// Valid is true if no errors were found.
	// Warnings do not affect validity.
	Valid bool `json:"valid"`

	// Errors are blocking issues that prevent routing.
	// If len(Errors) > 0, Valid must be false.
	Errors []ValidationError `json:"errors"`

	// Warnings are non-blocking issues that should be reviewed.
	// Routing will still work, but behavior may be unexpected.
	Warnings []ValidationWarning `json:"warnings"`
}

// ValidationError represents a blocking validation error.
type ValidationError struct {
	// Plugin is the plugin name (empty for global errors).
	Plugin string `json:"plugin"`

	// Field is the configuration field with the error.
	// Examples: "name", "patterns[0].pattern", "features"
	Field string `json:"field"`

	// Message describes the error.
	Message string `json:"message"`
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
	Plugin string `json:"plugin"`

	// Field is the configuration field with the warning.
	Field string `json:"field"`

	// Message describes the warning.
	Message string `json:"message"`
}

// ValidateRoutingConfig validates the routing configuration against available plugins.
//
// ValidateRoutingConfig validates routing configuration against the provided plugin clients.
// It verifies plugin presence, detects duplicate plugin entries, ensures priorities are >= 0,
// warns about unknown features, and validates pattern definitions (type must be "glob" or "regex",
// pattern must be non-empty, and regex patterns must compile).
// cfg is the routing configuration to validate; if nil, validation passes (automatic routing only).
// clients is the list of available plugin clients used to confirm plugin existence.
// It returns a ValidationResult whose Valid field is true when no blocking errors were found,
// Errors contains blocking validation issues, and Warnings contains non-blocking advisories.
//
//nolint:gocognit,funlen // Validation logic requires nested checks for comprehensive coverage
func ValidateRoutingConfig(cfg *config.RoutingConfig, clients []*pluginhost.Client) ValidationResult {
	result := ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
	}

	// If no config, validation passes (automatic routing only)
	if cfg == nil {
		return result
	}

	// Build plugin name lookup
	pluginNames := make(map[string]bool)
	for _, client := range clients {
		pluginNames[client.Name] = true
	}

	// Track seen plugin names for duplicate detection
	seenPlugins := make(map[string]int) // maps plugin name to config index

	for i, plugin := range cfg.Plugins {
		// Validate plugin name is not empty
		if plugin.Name == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Plugin:  fmt.Sprintf("plugins[%d]", i),
				Field:   "name",
				Message: "plugin name is required",
			})
			continue
		}

		// Check for duplicate plugin names (warning, not error)
		if prevIndex, seen := seenPlugins[plugin.Name]; seen {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Plugin:  plugin.Name,
				Field:   "name",
				Message: fmt.Sprintf("duplicate plugin configuration (also at index %d)", prevIndex),
			})
		}
		seenPlugins[plugin.Name] = i

		// Validate plugin exists
		if !pluginNames[plugin.Name] {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Plugin:  plugin.Name,
				Field:   "name",
				Message: "plugin not found",
			})
		}

		// Validate priority is non-negative
		if plugin.Priority < 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Plugin:  plugin.Name,
				Field:   "priority",
				Message: fmt.Sprintf("priority must be >= 0, got %d", plugin.Priority),
			})
		}

		// Validate features
		for j, feature := range plugin.Features {
			if !IsValidFeature(feature) {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Plugin:  plugin.Name,
					Field:   fmt.Sprintf("features[%d]", j),
					Message: fmt.Sprintf("unknown feature %q; valid features are: %v", feature, ValidFeatureNames()),
				})
			}
		}

		// Validate patterns
		for j, pattern := range plugin.Patterns {
			fieldName := fmt.Sprintf("patterns[%d]", j)

			// Validate pattern type
			if pattern.Type != config.PatternTypeGlob && pattern.Type != config.PatternTypeRegex {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Plugin:  plugin.Name,
					Field:   fieldName + ".type",
					Message: fmt.Sprintf("invalid pattern type %q; must be \"glob\" or \"regex\"", pattern.Type),
				})
				continue
			}

			// Validate pattern is not empty
			if pattern.Pattern == "" {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Plugin:  plugin.Name,
					Field:   fieldName + ".pattern",
					Message: "pattern cannot be empty",
				})
				continue
			}

			// Validate regex patterns compile
			if pattern.Type == config.PatternTypeRegex {
				if _, err := regexp.Compile(pattern.Pattern); err != nil {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Plugin:  plugin.Name,
						Field:   fieldName + ".pattern",
						Message: fmt.Sprintf("invalid regex: %v", err),
					})
				}
			}
		}
	}

	return result
}

// HasErrors returns true if the validation result contains any errors.
func (r ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if the validation result contains any warnings.
func (r ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// ErrorMessages returns all error messages as a slice of strings.
func (r ValidationResult) ErrorMessages() []string {
	messages := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		messages[i] = err.Error()
	}
	return messages
}

// WarningMessages returns all warning messages as a slice of strings.
func (r ValidationResult) WarningMessages() []string {
	messages := make([]string, len(r.Warnings))
	for i, warn := range r.Warnings {
		if warn.Plugin != "" {
			messages[i] = warn.Plugin + "." + warn.Field + ": " + warn.Message
		} else {
			messages[i] = warn.Field + ": " + warn.Message
		}
	}
	return messages
}
