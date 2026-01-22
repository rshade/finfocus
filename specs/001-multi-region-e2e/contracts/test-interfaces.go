// Package contracts defines the interfaces and contracts for multi-region E2E testing.
// This is a design-time artifact documenting expected test infrastructure behavior.
package contracts

import (
	"context"
	"time"
)

// RegionTestRunner executes E2E tests for a single AWS region.
type RegionTestRunner interface {
	// RunProjectedCostTest executes projected cost validation for the region.
	// Returns validation results or error if test setup/execution fails.
	RunProjectedCostTest(
		ctx context.Context,
		config RegionTestConfig,
	) (*MultiRegionTestResult, error)

	// RunActualCostTest executes actual cost validation for deployed resources in the region.
	// Requires Pulumi Automation API support (issue #177) and GetActualCost implementation (issue #24).
	// Returns validation results or error if test setup/execution fails.
	RunActualCostTest(ctx context.Context, config RegionTestConfig) (*MultiRegionTestResult, error)

	// RunFallbackTest validates fallback behavior when region-specific plugins are unavailable.
	// Returns validation results or error if fallback logic fails.
	RunFallbackTest(ctx context.Context, config RegionTestConfig) (*MultiRegionTestResult, error)
}

// CostValidator validates actual costs against expected ranges with tolerance.
type CostValidator interface {
	// ValidateCost checks if actual cost is within expected range.
	// Returns CostValidationResult with tolerance check outcome.
	ValidateCost(actual CostResult, expected ExpectedCost, tolerance float64) CostValidationResult

	// ValidateMultiRegionCosts validates costs across multiple regions and aggregates results.
	// Returns MultiRegionTestResult summarizing all validations.
	ValidateMultiRegionCosts(
		results []CostResult,
		expectations []ExpectedCost,
		tolerance float64,
	) (*MultiRegionTestResult, error)
}

// FixtureLoader loads Pulumi programs and expected cost data for test fixtures.
type FixtureLoader interface {
	// LoadExpectedCosts reads and parses expected-costs.json for a region.
	// Returns slice of ExpectedCost or error if file is missing/invalid.
	LoadExpectedCosts(fixturePath string) ([]ExpectedCost, error)

	// GetPulumiProgramPath returns absolute path to Pulumi program for a region.
	// Returns error if fixture directory doesn't exist.
	GetPulumiProgramPath(region string) (string, error)

	// ValidateFixture checks that a fixture directory contains required files.
	// Required files: main.go, Pulumi.yaml, expected-costs.json
	// Returns error if any required file is missing.
	ValidateFixture(fixturePath string) error
}

// RetryPolicy defines retry behavior for transient failures.
type RetryPolicy interface {
	// ShouldRetry determines if an error is transient and should be retried.
	// Returns true for network errors, false for validation errors.
	ShouldRetry(err error) bool

	// GetBackoff returns backoff duration for a given attempt number.
	// Implements exponential backoff: 100ms, 200ms, 400ms for attempts 0, 1, 2.
	GetBackoff(attempt int) time.Duration

	// MaxAttempts returns maximum retry attempts (3 per clarification).
	MaxAttempts() int
}

// RegionTestConfig holds configuration for a single region's E2E test execution.
type RegionTestConfig struct {
	Region            string        // AWS region (us-east-1, eu-west-1, ap-northeast-1)
	FixturePath       string        // Path to Pulumi program fixture directory
	ExpectedCostsPath string        // Path to expected-costs.json file
	Tolerance         float64       // Cost variance tolerance (0.05 for ±5%)
	Timeout           time.Duration // Maximum test execution time (5 minutes default)
	PluginVersion     string        // Expected AWS plugin version (e.g., "0.1.0")
}

// ExpectedCost defines expected cost range for a specific resource type in a region.
type ExpectedCost struct {
	ResourceType string  // Pulumi resource type (e.g., "aws:ec2:Instance")
	ResourceName string  // Logical resource name in Pulumi program
	MinCost      float64 // Minimum acceptable monthly cost (USD)
	MaxCost      float64 // Maximum acceptable monthly cost (USD)
	Region       string  // AWS region this expectation applies to
	CostType     string  // "projected" or "actual"
}

// CostResult represents actual cost calculation result from FinFocus CLI.
type CostResult struct {
	ResourceName string  // Resource identifier
	ResourceType string  // Pulumi resource type
	Region       string  // AWS region
	MonthlyCost  float64 // Calculated monthly cost (USD)
	Currency     string  // Currency code (should always be "USD")
	CostType     string  // "projected" or "actual"
	Timestamp    time.Time
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

// ErrorClassifier classifies errors for retry and failure handling.
type ErrorClassifier interface {
	// IsNetworkError determines if an error is a network-related transient failure.
	IsNetworkError(err error) bool

	// IsMissingPricingData determines if an error indicates missing pricing data.
	IsMissingPricingData(err error) bool

	// IsInvalidRegion determines if an error indicates invalid region configuration.
	IsInvalidRegion(err error) bool
}

// Contract: Test Execution Flow
//
// 1. Test Setup:
//    - Validate RegionTestConfig (region code, fixture path, tolerance)
//    - Load ExpectedCost data from expected-costs.json
//    - Check plugin availability and version
//
// 2. Test Execution (per region, per cost type):
//    - For Projected Costs:
//      * Generate Pulumi plan JSON from fixture program
//      * Execute: finfocus cost projected --pulumi-json <plan> --output json
//      * Parse cost results
//    - For Actual Costs:
//      * Deploy Pulumi resources using Automation API (requires issue #177)
//      * Execute: finfocus cost actual --state-file <state> --output json
//      * Parse cost results
//      * Cleanup deployed resources (defer)
//
// 3. Validation:
//    - For each resource:
//      * Compare actual cost against expected min/max
//      * Calculate variance percentage
//      * Record CostValidationResult (pass/fail)
//
// 4. Aggregation:
//    - Collect all CostValidationResult into MultiRegionTestResult
//    - Calculate pass/fail counts
//    - Determine overall test success
//
// 5. Failure Handling:
//    - Missing pricing data → Fail immediately (no retry)
//    - Network errors → Retry 3x with exponential backoff, then fail
//    - Invalid region config → Fail immediately in setup
//    - Cost out of tolerance → Record failure, continue test
//
// 6. Assertion:
//    - require.True(t, result.Success, "Multi-region validation failed")
//    - Log detailed failure information for debugging

// Contract: Fixture Structure
//
// test/e2e/fixtures/multi-region/<region>/
//   ├── main.go              # Pulumi program (5-10 resources, 3-4 types)
//   ├── Pulumi.yaml          # Stack configuration with region
//   ├── go.mod               # Go module for Pulumi program
//   └── expected-costs.json  # Expected cost ranges per resource
//
// Example expected-costs.json:
// {
//   "region": "us-east-1",
//   "cost_type": "projected",
//   "resources": [
//     {
//       "resource_type": "aws:ec2:Instance",
//       "resource_name": "web-server",
//       "min_cost": 50.0,
//       "max_cost": 55.0,
//       "region": "us-east-1",
//       "cost_type": "projected"
//     }
//   ]
// }

// Contract: Test Naming Convention
//
// Test functions follow naming pattern: TestMultiRegion_<CostType>_<Scenario>_<Region>
//
// Examples:
// - TestMultiRegion_Projected_Baseline_USEast1
// - TestMultiRegion_Actual_PricingDifference_EUWest1
// - TestMultiRegion_Fallback_MissingPlugin_APNortheast1
// - TestMultiRegion_ErrorHandling_MissingPricingData
// - TestMultiRegion_ErrorHandling_NetworkFailure

// Contract: Environment Variables
//
// Tests respect existing E2E environment variables:
// - FINFOCUS_E2E_AWS_REGION: Override default region (default: us-east-1)
// - FINFOCUS_E2E_TOLERANCE: Override cost tolerance (default: 0.05)
// - FINFOCUS_E2E_TIMEOUT: Override test timeout (default: 10m)
//
// Additional variables for multi-region testing:
// - FINFOCUS_E2E_REGIONS: Comma-separated list of regions to test (default: "us-east-1,eu-west-1,ap-northeast-1")
// - FINFOCUS_E2E_SKIP_ACTUAL_COSTS: Skip actual cost tests if "true" (for pre-#177 development)
