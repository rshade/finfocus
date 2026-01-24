package e2e

import (
	"fmt"
	"math"
	"time"
)

// ComparisonReport holds details about a cost comparison.
type ComparisonReport struct {
	Expected    float64
	Actual      float64
	Diff        float64
	PercentDiff float64
	WithinLimit bool
	Message     string
}

// String returns a formatted string representation of the comparison report.
func (r ComparisonReport) String() string {
	status := "PASS"
	if !r.WithinLimit {
		status = "FAIL"
	}
	return fmt.Sprintf("[%s] Expected: $%.4f, Actual: $%.4f, Diff: $%.4f (%.2f%%) - %s",
		status, r.Expected, r.Actual, r.Diff, r.PercentDiff, r.Message)
}

// CostValidator defines the interface for validating cost calculations.
type CostValidator interface {
	ValidateProjected(actual float64, expected float64) error
	ValidateActual(calculated float64, runtime time.Duration, expectedHourly float64) error
	Compare(actual float64, expected float64) ComparisonReport
}

// DefaultCostValidator is a concrete implementation of CostValidator.
type DefaultCostValidator struct {
	TolerancePercent float64
}

// NewDefaultCostValidator creates a new DefaultCostValidator with the given tolerance.
func NewDefaultCostValidator(tolerance float64) *DefaultCostValidator {
	return &DefaultCostValidator{
		TolerancePercent: tolerance,
	}
}

// Compare generates a structured report comparing two cost values.
func (v *DefaultCostValidator) Compare(actual float64, expected float64) ComparisonReport {
	diff := math.Abs(actual - expected)
	var percentDiff float64
	if expected != 0 {
		percentDiff = (diff / expected) * 100
	} else if actual != 0 {
		percentDiff = 100.0 // Infinite difference if expected 0 but actual > 0
	}

	withinLimit := percentDiff <= v.TolerancePercent

	msg := "Within tolerance"
	if !withinLimit {
		msg = fmt.Sprintf("Exceeds tolerance of %.2f%%", v.TolerancePercent)
	}

	return ComparisonReport{
		Expected:    expected,
		Actual:      actual,
		Diff:        diff,
		PercentDiff: percentDiff,
		WithinLimit: withinLimit,
		Message:     msg,
	}
}

// ValidateProjected checks if the actual projected cost is within tolerance of the expected cost.
func (v *DefaultCostValidator) ValidateProjected(actual float64, expected float64) error {
	report := v.Compare(actual, expected)
	if !report.WithinLimit {
		return fmt.Errorf("projected cost mismatch: %s", report.String())
	}
	return nil
}

// ValidateActual checks if the calculated actual cost is proportional to runtime.
// Fallback formula: projected_cost * runtime_hours / 730
func (v *DefaultCostValidator) ValidateActual(calculated float64, runtime time.Duration, expectedHourly float64) error {
	// Note: AWS EC2 has per-second billing with a 1-minute minimum.
	// This validator enforces a 1-minute minimum billing period for testing purposes.
	// For this validator, we'll compare against the expected hourly rate * runtime

	runtimeHours := runtime.Hours()
	// Enforce minimum billing period of 1 minute for testing
	minBillingHours := 1.0 / 60.0 // 1 minute minimum
	if runtimeHours < minBillingHours {
		runtimeHours = minBillingHours
	}
	expectedTotal := expectedHourly * runtimeHours

	// Use a slightly looser tolerance for actual costs due to timing variations
	// or billing granularity if needed. For now, using the same tolerance.
	return v.ValidateProjected(calculated, expectedTotal)
}

// ValidateCost checks if actual cost is within expected range with tolerance.
func (v *DefaultCostValidator) ValidateCost(actual CostResult, expected ExpectedCost, tolerance float64) CostValidationResult {
	// Use midpoint of expected range for tolerance calculation
	expectedMidpoint := (expected.MinCost + expected.MaxCost) / 2
	diff := math.Abs(actual.MonthlyCost - expectedMidpoint)
	var variance float64
	if expectedMidpoint != 0 {
		variance = (diff / expectedMidpoint) * 100
	}

	// Apply tolerance to the range if provided (tolerance > 0)
	var lowerBound, upperBound float64
	if tolerance > 0 {
		// Normalize tolerance: if > 1, treat as percentage (5 -> 0.05)
		t := tolerance
		if tolerance > 1 {
			t = tolerance / 100
		}
		// Apply tolerance as percentage adjustment to midpoint
		lowerBound = expectedMidpoint * (1 - t)
		upperBound = expectedMidpoint * (1 + t)
	} else {
		// Fall back to expected min/max range
		lowerBound = expected.MinCost
		upperBound = expected.MaxCost
	}

	withinTolerance := actual.MonthlyCost >= lowerBound && actual.MonthlyCost <= upperBound

	return CostValidationResult{
		ResourceName:    actual.ResourceName,
		ResourceType:    actual.ResourceType,
		Region:          actual.Region,
		ActualCost:      actual.MonthlyCost,
		ExpectedMin:     lowerBound,
		ExpectedMax:     upperBound,
		WithinTolerance: withinTolerance,
		Variance:        variance,
		Timestamp:       time.Now(),
		CostType:        actual.CostType,
	}
}

// ValidateMultiRegionCosts validates costs across multiple regions and aggregates results.
func (v *DefaultCostValidator) ValidateMultiRegionCosts(results []CostResult, expectations []ExpectedCost, tolerance float64) (*MultiRegionTestResult, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results to validate")
	}

	regions := make(map[string]bool)
	var validationResults []CostValidationResult

	// Create lookup map for expectations by resource and region
	expectationMap := make(map[string]map[string]ExpectedCost)
	for _, exp := range expectations {
		if expectationMap[exp.ResourceName] == nil {
			expectationMap[exp.ResourceName] = make(map[string]ExpectedCost)
		}
		expectationMap[exp.ResourceName][exp.Region] = exp
	}

	for _, result := range results {
		regions[result.Region] = true

		exp, exists := expectationMap[result.ResourceName][result.Region]
		if !exists {
			return nil, fmt.Errorf("no expectation found for resource %s in region %s", result.ResourceName, result.Region)
		}

		validationResult := v.ValidateCost(result, exp, tolerance)
		validationResults = append(validationResults, validationResult)
	}

	// Aggregate results
	totalResources := len(validationResults)
	passed := 0
	failed := 0
	for _, vr := range validationResults {
		if vr.WithinTolerance {
			passed++
		} else {
			failed++
		}
	}

	var regionList []string
	for r := range regions {
		regionList = append(regionList, r)
	}

	testResult := &MultiRegionTestResult{
		Regions:           regionList,
		Results:           validationResults,
		TotalResources:    totalResources,
		PassedValidations: passed,
		FailedValidations: failed,
		ExecutionTime:     0, // Will be set by caller
		TestTimestamp:     time.Now(),
		Success:           failed == 0,
	}

	return testResult, nil
}

// ValidateUnifiedFixtureCosts validates costs for a unified multi-region fixture.
// It validates both per-resource costs and aggregate total.
func (v *DefaultCostValidator) ValidateUnifiedFixtureCosts(results []CostResult, expected UnifiedExpectedCosts) (*UnifiedValidationResult, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results to validate")
	}

	result := &UnifiedValidationResult{
		PerResourceResults: make([]CostValidationResult, 0, len(expected.Resources)),
	}

	// Validate each resource
	for _, exp := range expected.Resources {
		actual := findResultByNameAndRegion(results, exp.ResourceName, exp.Region)
		if actual == nil {
			return nil, fmt.Errorf("missing cost result for resource %s in region %s", exp.ResourceName, exp.Region)
		}

		validation := v.ValidateCost(*actual, exp, expected.AggregateValidation.Tolerance)
		result.PerResourceResults = append(result.PerResourceResults, validation)
	}

	// Calculate and validate aggregate total
	totalCost := sumCosts(results)
	result.TotalCost = totalCost
	result.ExpectedTotalMin = expected.AggregateValidation.TotalMinCost
	result.ExpectedTotalMax = expected.AggregateValidation.TotalMaxCost
	result.TotalWithinTolerance = totalCost >= expected.AggregateValidation.TotalMinCost &&
		totalCost <= expected.AggregateValidation.TotalMaxCost

	return result, nil
}

// findResultByNameAndRegion finds a CostResult by resource name and region.
func findResultByNameAndRegion(results []CostResult, name, region string) *CostResult {
	for i := range results {
		if results[i].ResourceName == name && results[i].Region == region {
			return &results[i]
		}
	}
	return nil
}

// sumCosts calculates the sum of all monthly costs.
func sumCosts(results []CostResult) float64 {
	var total float64
	for _, r := range results {
		total += r.MonthlyCost
	}
	return total
}
