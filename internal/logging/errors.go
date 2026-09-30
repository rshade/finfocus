package logging

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

// ErrorCategory represents the type of error for better user guidance.
type ErrorCategory string

const (
	// ErrorCategoryUser indicates user-facing errors (invalid input, configuration).
	ErrorCategoryUser ErrorCategory = "USER"

	// ErrorCategorySystem indicates system errors (network, filesystem, permissions).
	ErrorCategorySystem ErrorCategory = "SYSTEM"

	// ErrorCategoryDeveloper indicates developer errors (bugs, protocol mismatches).
	ErrorCategoryDeveloper ErrorCategory = "DEVELOPER"
)

// CategorizedError represents an error with category, solution, and context.
type CategorizedError struct {
	Category ErrorCategory
	Message  string
	Solution string
	Context  map[string]string
	Cause    error
}

// Error implements the error interface.
func (e *CategorizedError) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Error: [%s] %s\n\n", e.Category, e.Message)

	if e.Solution != "" {
		fmt.Fprintf(&b, "Solution: %s\n\n", e.Solution)
	}

	if len(e.Context) > 0 {
		keys := make([]string, 0, len(e.Context))
		for k := range e.Context {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Fprintf(&b, "Context:\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s: %s\n", k, e.Context[k])
		}
		fmt.Fprintf(&b, "\n")
	}

	if e.Cause != nil {
		fmt.Fprintf(&b, "Details: %s", e.Cause.Error())
	}

	return b.String()
}

// Unwrap returns the cause error for error wrapping.
func (e *CategorizedError) Unwrap() error {
	return e.Cause
}

// UserError creates a user-facing error with helpful guidance.
func UserError(message, solution string, cause error) *CategorizedError {
	return &CategorizedError{
		Category: ErrorCategoryUser,
		Message:  message,
		Solution: solution,
		Context:  make(map[string]string),
		Cause:    cause,
	}
}

// SystemError creates a system error with troubleshooting guidance.
func SystemError(message, solution string, cause error) *CategorizedError {
	return &CategorizedError{
		Category: ErrorCategorySystem,
		Message:  message,
		Solution: solution,
		Context:  make(map[string]string),
		Cause:    cause,
	}
}

// DeveloperError creates a developer error with debugging information.
func DeveloperError(message, solution string, cause error) *CategorizedError {
	return &CategorizedError{
		Category: ErrorCategoryDeveloper,
		Message:  message,
		Solution: solution,
		Context:  make(map[string]string),
		Cause:    cause,
	}
}

// WithContext adds contextual information to the error.
// It returns a new error with the additional context to maintain immutability.
func (e *CategorizedError) WithContext(key, value string) *CategorizedError {
	// Create a copy of the context map
	newContext := make(map[string]string, len(e.Context)+1)
	maps.Copy(newContext, e.Context)
	newContext[key] = value

	// Return a new error with the updated context
	return &CategorizedError{
		Category: e.Category,
		Message:  e.Message,
		Solution: e.Solution,
		Context:  newContext,
		Cause:    e.Cause,
	}
}

// Common error constructors for specific scenarios.

// InvalidArgumentError creates an error for invalid CLI arguments.
func InvalidArgumentError(arg string, cause error) *CategorizedError {
	return UserError(
		fmt.Sprintf("Invalid argument: %s", arg),
		"Check the command usage with --help flag for valid options",
		cause,
	)
}

// MissingConfigError creates an error for missing configuration.
func MissingConfigError(configKey string, cause error) *CategorizedError {
	return UserError(
		fmt.Sprintf("Missing configuration: %s", configKey),
		"Run 'finfocus config init' to create default configuration, then set the required value with 'finfocus config set'",
		cause,
	).WithContext("config_key", configKey)
}

// InvalidPulumiJSONError creates an error for invalid Pulumi JSON.
func InvalidPulumiJSONError(path string, cause error) *CategorizedError {
	return UserError(
		"Invalid Pulumi JSON file",
		"Ensure the file was generated with 'pulumi preview --json > plan.json' and is valid JSON",
		cause,
	).WithContext("file_path", path)
}

// PluginNotFoundError creates an error for missing plugins.
func PluginNotFoundError(pluginName string, cause error) *CategorizedError {
	return UserError(
		fmt.Sprintf("Plugin not found: %s", pluginName),
		fmt.Sprintf(
			"Install the plugin to ~/.finfocus/plugins/%s/<version>/ or check available plugins with 'finfocus plugin list'",
			pluginName,
		),
		cause,
	).WithContext("plugin_name", pluginName)
}

// NetworkError creates an error for network connectivity issues.
func NetworkError(operation string, cause error) *CategorizedError {
	return SystemError(
		fmt.Sprintf("Network error during %s", operation),
		"Check your internet connection and firewall settings. If the problem persists, try again later",
		cause,
	).WithContext("operation", operation)
}

// FileSystemError creates an error for filesystem operations.
func FileSystemError(operation, path string, cause error) *CategorizedError {
	return SystemError(
		fmt.Sprintf("Filesystem error: %s", operation),
		"Check file permissions and ensure the directory exists. You may need to run with appropriate permissions",
		cause,
	).WithContext("operation", operation).WithContext("path", path)
}

// PluginCommunicationError creates an error for plugin communication failures.
func PluginCommunicationError(pluginName string, cause error) *CategorizedError {
	return SystemError(
		fmt.Sprintf("Failed to communicate with plugin: %s", pluginName),
		"Try restarting the plugin or reinstalling it. Check plugin logs for more details",
		cause,
	).WithContext("plugin", pluginName)
}

// ProtocolMismatchError creates an error for protocol version mismatches.
func ProtocolMismatchError(pluginName, expectedVersion, actualVersion string) *CategorizedError {
	return DeveloperError(
		fmt.Sprintf("Protocol version mismatch with plugin: %s", pluginName),
		"Update the plugin to a compatible version or report this issue to the plugin developer",
		nil,
	).WithContext("plugin", pluginName).
		WithContext("expected_version", expectedVersion).
		WithContext("actual_version", actualVersion)
}

// PluginBugError creates an error for plugin implementation bugs.
func PluginBugError(pluginName, operation string, cause error) *CategorizedError {
	return DeveloperError(
		fmt.Sprintf("Plugin bug detected in %s: %s", pluginName, operation),
		"Report this issue to the plugin developer with the error details below",
		cause,
	).WithContext("plugin", pluginName).WithContext("operation", operation)
}
