package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

// RegionTestConfig holds configuration for a single region's E2E test execution.
type RegionTestConfig struct {
	Region            string        // AWS region (us-east-1, eu-west-1, ap-northeast-1)
	FixturePath       string        // Path to Pulumi program fixture directory
	ExpectedCostsPath string        // Path to expected-costs.json file
	Tolerance         float64       // Cost variance tolerance (0.05 for ±5%)
	Timeout           time.Duration // Maximum test execution time (5 minutes default)
	PluginVersion     string        // Expected AWS plugin version (e.g., "0.1.0")
}

// Validate validates the RegionTestConfig.
func (r *RegionTestConfig) Validate() error {
	validRegions := []string{"us-east-1", "eu-west-1", "ap-northeast-1"}
	if !contains(validRegions, r.Region) {
		return fmt.Errorf("invalid region: %s (must be one of %v)", r.Region, validRegions)
	}
	if r.Tolerance <= 0 {
		return fmt.Errorf("tolerance must be > 0, got: %f", r.Tolerance)
	}
	if r.Tolerance > 100 {
		return fmt.Errorf("tolerance must be ≤ 100 (percent) or ≤ 1.0 (fraction), got: %f", r.Tolerance)
	}
	// Convert percent-style tolerance (e.g., 5 for 5%) to fractional form (0.05)
	if r.Tolerance > 1.0 {
		r.Tolerance = r.Tolerance / 100.0
	}
	if r.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", r.Timeout)
	}
	return nil
}

// ExpectedCost defines expected cost range for a specific resource type in a region.
type ExpectedCost struct {
	ResourceType string  `json:"resource_type"` // Pulumi resource type (e.g., "aws:ec2:Instance")
	ResourceName string  `json:"resource_name"` // Logical resource name in Pulumi program
	MinCost      float64 `json:"min_cost"`      // Minimum acceptable monthly cost (USD)
	MaxCost      float64 `json:"max_cost"`      // Maximum acceptable monthly cost (USD)
	Region       string  `json:"region"`        // AWS region this expectation applies to
	CostType     string  `json:"cost_type"`     // "projected" or "actual"
}

// Validate validates the ExpectedCost.
func (e *ExpectedCost) Validate() error {
	if e.ResourceType == "" {
		return fmt.Errorf("resource type is required")
	}
	if e.ResourceName == "" {
		return fmt.Errorf("resource name is required")
	}
	if e.MinCost < 0 {
		return fmt.Errorf("min cost must be non-negative, got: %f", e.MinCost)
	}
	if e.MaxCost <= e.MinCost {
		return fmt.Errorf("max cost (%f) must be greater than min cost (%f)", e.MaxCost, e.MinCost)
	}
	if e.CostType != "projected" && e.CostType != "actual" {
		return fmt.Errorf("invalid cost type: %s", e.CostType)
	}
	return nil
}

// CostResult represents the output from a cost calculation command.
type CostResult struct {
	ResourceName string    // Resource identifier
	ResourceType string    // Pulumi resource type
	Region       string    // AWS region
	MonthlyCost  float64   // Calculated monthly cost (USD)
	Currency     string    // Currency code (should always be "USD")
	CostType     string    // "projected" or "actual"
	Timestamp    time.Time // When the cost was calculated
}

// CostValidationResult captures the result of cost validation for a single resource.
type CostValidationResult struct {
	ResourceName    string    // Resource identifier
	ResourceType    string    // Pulumi resource type
	Region          string    // AWS region
	ActualCost      float64   // Actual calculated cost (USD)
	ExpectedMin     float64   // Minimum expected cost (USD)
	ExpectedMax     float64   // Maximum expected cost (USD)
	WithinTolerance bool      // Whether actual cost is within expected range
	Variance        float64   // Percentage variance from expected midpoint
	Timestamp       time.Time // When validation was performed
	CostType        string    // "projected" or "actual"
}

// MultiRegionTestResult aggregates validation results across all regions and cost types.
type MultiRegionTestResult struct {
	Regions           []string               // List of tested regions
	Results           []CostValidationResult // Individual validation results
	TotalResources    int                    // Total number of resources validated
	PassedValidations int                    // Number of resources within tolerance
	FailedValidations int                    // Number of resources outside tolerance
	ExecutionTime     time.Duration          // Total test execution time
	TestTimestamp     time.Time              // When test suite started
	Success           bool                   // True if all validations passed
}

// Validate validates the MultiRegionTestResult.
func (m *MultiRegionTestResult) Validate() error {
	if len(m.Results) == 0 {
		return fmt.Errorf("results cannot be empty")
	}
	if m.TotalResources != len(m.Results) {
		return fmt.Errorf("total resources (%d) must match results length (%d)", m.TotalResources, len(m.Results))
	}
	if m.PassedValidations+m.FailedValidations != m.TotalResources {
		return fmt.Errorf("passed (%d) + failed (%d) must equal total (%d)", m.PassedValidations, m.FailedValidations, m.TotalResources)
	}
	return nil
}

// IsSuccess returns true if all validations passed.
func (m *MultiRegionTestResult) IsSuccess() bool {
	return m.FailedValidations == 0
}

// CostType enumeration.
type CostType string

const (
	CostTypeProjected CostType = "projected"
	CostTypeActual    CostType = "actual"
)

// IsValid checks if the CostType is valid.
func (c CostType) IsValid() bool {
	return c == CostTypeProjected || c == CostTypeActual
}

// String returns the string representation.
func (c CostType) String() string {
	return string(c)
}

// RetryWithBackoff retries an operation with exponential backoff on transient errors.
func RetryWithBackoff(ctx context.Context, attempts int, operation func() error) error {
	var err error
	for i := range attempts {
		err = operation()
		if err == nil {
			return nil
		}
		if !isTransientError(err) {
			return fmt.Errorf("non-transient error: %w", err)
		}
		backoff := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", attempts, err)
}

// isTransientError determines if an error is transient (network-related).
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	// Simple check for network errors - can be expanded
	errStr := err.Error()
	return containsAnySubstring(errStr, []string{"connection refused", "timeout", "network", "unreachable"})
}

// contains checks if a slice contains a string (exact match).
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

// containsAnySubstring checks if a string contains any of the given substrings.
func containsAnySubstring(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// ErrorClassifier interface for classifying errors.
type ErrorClassifier interface {
	IsNetworkError(err error) bool
	IsMissingPricingData(err error) bool
	IsInvalidRegion(err error) bool
}

// DefaultErrorClassifier implements ErrorClassifier.
type DefaultErrorClassifier struct{}

// IsNetworkError checks if error is network-related.
func (d *DefaultErrorClassifier) IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	return isTransientError(err)
}

// IsMissingPricingData checks if error indicates missing pricing data.
func (d *DefaultErrorClassifier) IsMissingPricingData(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsAnySubstring(errStr, []string{"no pricing data", "pricing unavailable", "cost not found"})
}

// IsInvalidRegion checks if error indicates invalid region.
func (d *DefaultErrorClassifier) IsInvalidRegion(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsAnySubstring(errStr, []string{"invalid region", "region not supported"})
}

// PluginInfo holds information about a loaded plugin.
type PluginInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// GetLoadedPlugins retrieves information about currently loaded plugins.
// This function executes 'finfocus plugin list' to get plugin information.
func GetLoadedPlugins(t *testing.T) ([]PluginInfo, error) {
	t.Helper()

	binaryPath := findFinFocusBinary()
	cmd := exec.Command(binaryPath, "plugin", "list")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w (output: %s)", err, string(output))
	}

	var plugins []PluginInfo
	if err := json.Unmarshal(output, &plugins); err != nil {
		// Try parsing as a different format if JSON unmarshaling fails
		// The plugin list might not be in JSON format yet
		return parsePluginListText(string(output)), nil
	}

	return plugins, nil
}

// parsePluginListText parses the text output of 'plugin list' command.
// This is a fallback for when JSON output is not available.
func parsePluginListText(output string) []PluginInfo {
	var plugins []PluginInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		// Skip header lines and empty lines
		if strings.HasPrefix(line, "NAME") || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse plugin information from text output
		// Expected format: "NAME    VERSION    PATH"
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			plugins = append(plugins, PluginInfo{
				Name:    fields[0],
				Version: fields[1],
				Path:    strings.Join(fields[2:], " "),
			})
		}
	}

	return plugins
}

// LogPluginVersions logs the versions of all loaded plugins for debugging.
func LogPluginVersions(t *testing.T) {
	t.Helper()

	plugins, err := GetLoadedPlugins(t)
	if err != nil {
		t.Logf("Warning: failed to get plugin versions: %v", err)
		return
	}

	if len(plugins) == 0 {
		t.Log("No plugins currently loaded")
		return
	}

	t.Log("Loaded plugins:")
	for _, plugin := range plugins {
		t.Logf("  - %s (version: %s, path: %s)", plugin.Name, plugin.Version, plugin.Path)
	}
}

// ValidatePluginVersion verifies that a specific plugin version is loaded.
func ValidatePluginVersion(t *testing.T, expectedPluginName, expectedVersion string) error {
	t.Helper()

	plugins, err := GetLoadedPlugins(t)
	if err != nil {
		return fmt.Errorf("failed to get loaded plugins: %w", err)
	}

	for _, plugin := range plugins {
		if plugin.Name == expectedPluginName {
			if plugin.Version == expectedVersion {
				t.Logf("Plugin %s version %s is loaded correctly", expectedPluginName, expectedVersion)
				return nil
			}
			return fmt.Errorf("plugin %s version mismatch: expected %s, got %s",
				expectedPluginName, expectedVersion, plugin.Version)
		}
	}

	return fmt.Errorf("plugin %s not found in loaded plugins", expectedPluginName)
}

// UnifiedExpectedCosts represents expected costs for a unified multi-region fixture.
type UnifiedExpectedCosts struct {
	FixtureType         string               `json:"fixture_type"`
	TotalRegions        int                  `json:"total_regions"`
	Resources           []ExpectedCost       `json:"resources"`
	AggregateValidation AggregateExpectation `json:"aggregate_validation"`
}

// AggregateExpectation defines aggregate cost validation bounds.
type AggregateExpectation struct {
	TotalMinCost float64 `json:"total_min_cost"`
	TotalMaxCost float64 `json:"total_max_cost"`
	Tolerance    float64 `json:"tolerance"`
}

// UnifiedValidationResult holds the results of unified fixture validation.
type UnifiedValidationResult struct {
	PerResourceResults   []CostValidationResult
	TotalCost            float64
	TotalWithinTolerance bool
	ExpectedTotalMin     float64
	ExpectedTotalMax     float64
}
