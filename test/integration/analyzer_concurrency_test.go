//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/test/integration/helpers"
)

// largeStackCalculator implements analyzer.CostCalculator for concurrency tests.
// It returns predictable cost results for each resource.
type largeStackCalculator struct {
	costPerResource float64
}

func (m *largeStackCalculator) GetProjectedCost(
	_ context.Context,
	resources []engine.ResourceDescriptor,
) ([]engine.CostResult, error) {
	results := make([]engine.CostResult, 0, len(resources))
	for _, r := range resources {
		results = append(results, engine.CostResult{
			ResourceType: r.Type,
			ResourceID:   r.ID,
			Adapter:      "mock-calculator",
			Currency:     "USD",
			Monthly:      m.costPerResource,
			Hourly:       m.costPerResource / 730,
		})
	}
	return results, nil
}

func (m *largeStackCalculator) GetRecommendationsForResources(
	_ context.Context,
	_ []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	return &engine.RecommendationsResult{}, nil
}

// TestAnalyzer_LargeStack100Resources verifies that the analyzer handles a
// 100-resource stack within 10 seconds, returns diagnostics for all resources,
// and does not leak goroutines.
func TestAnalyzer_LargeStack100Resources(t *testing.T) {
	const resourceCount = 100
	const maxDuration = 10 * time.Second

	calc := &largeStackCalculator{costPerResource: 10.0}
	srv := analyzer.NewServer(calc, "1.0.0-test")

	// Configure stack context
	_, err := srv.ConfigureStack(context.Background(), &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:   "dev",
		Project: "large-stack-test",
	})
	require.NoError(t, err, "ConfigureStack should succeed")

	// Generate 100-resource synthetic stack
	resources := helpers.GenerateSyntheticStack(resourceCount)
	require.Len(t, resources, resourceCount, "should generate exactly %d resources", resourceCount)

	// Capture goroutine count before analysis
	runtime.GC()
	goroutinesBefore := runtime.NumGoroutine()

	// Set up timeout context
	ctx, cancel := context.WithTimeout(context.Background(), maxDuration)
	defer cancel()

	// Call Analyze() for each resource (simulating Pulumi workflow)
	start := time.Now()
	totalDiagnostics := 0

	for _, resource := range resources {
		analyzeReq := &pulumirpc.AnalyzeRequest{
			Type:       resource.GetType(),
			Urn:        resource.GetUrn(),
			Properties: resource.GetProperties(),
		}

		resp, analyzeErr := srv.Analyze(ctx, analyzeReq)
		require.NoError(t, analyzeErr, "Analyze should not error for resource %s", resource.GetUrn())
		require.NotNil(t, resp, "Analyze response should not be nil")
		totalDiagnostics += len(resp.GetDiagnostics())
	}

	// Call AnalyzeStack() for the summary
	stackResp, stackErr := srv.AnalyzeStack(ctx, &pulumirpc.AnalyzeStackRequest{
		Resources: resources,
	})
	require.NoError(t, stackErr, "AnalyzeStack should not error")
	require.NotNil(t, stackResp, "AnalyzeStack response should not be nil")

	elapsed := time.Since(start)

	// Verify completion within time limit
	assert.Less(t, elapsed, maxDuration,
		"100-resource analysis should complete within %v, took %v", maxDuration, elapsed)

	// Verify diagnostic count: at least one per resource from Analyze() calls
	assert.GreaterOrEqual(t, totalDiagnostics, resourceCount,
		"should have at least %d diagnostics from Analyze() calls, got %d",
		resourceCount, totalDiagnostics)

	// Verify AnalyzeStack returned at least the summary diagnostic
	assert.GreaterOrEqual(t, len(stackResp.GetDiagnostics()), 1,
		"AnalyzeStack should return at least the summary diagnostic")

	// Verify summary contains the total cost
	summaryMsg := stackResp.GetDiagnostics()[0].GetMessage()
	assert.Contains(t, summaryMsg, "resources analyzed",
		"summary should mention analyzed resources")

	// Check goroutine delta for leaks
	runtime.GC()
	time.Sleep(100 * time.Millisecond) // Allow goroutines to settle
	goroutinesAfter := runtime.NumGoroutine()
	goroutineDelta := goroutinesAfter - goroutinesBefore

	assert.LessOrEqual(t, goroutineDelta, 2,
		"goroutine count should not increase by more than 2 (before=%d, after=%d, delta=%d)",
		goroutinesBefore, goroutinesAfter, goroutineDelta)

	t.Logf("100-resource analysis completed in %v (%d diagnostics, goroutine delta=%d)",
		elapsed, totalDiagnostics, goroutineDelta)
}

// partialFailureCalculator implements analyzer.CostCalculator that errors
// for specific resource types and succeeds for others.
type partialFailureCalculator struct {
	failTypes       map[string]bool
	costPerResource float64
}

func (m *partialFailureCalculator) GetProjectedCost(
	_ context.Context,
	resources []engine.ResourceDescriptor,
) ([]engine.CostResult, error) {
	for _, r := range resources {
		if m.failTypes[r.Type] {
			return nil, fmt.Errorf("simulated plugin failure for type %s", r.Type)
		}
	}
	results := make([]engine.CostResult, 0, len(resources))
	for _, r := range resources {
		results = append(results, engine.CostResult{
			ResourceType: r.Type,
			ResourceID:   r.ID,
			Adapter:      "mock-calculator",
			Currency:     "USD",
			Monthly:      m.costPerResource,
			Hourly:       m.costPerResource / 730,
		})
	}
	return results, nil
}

func (m *partialFailureCalculator) GetRecommendationsForResources(
	_ context.Context,
	_ []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	return &engine.RecommendationsResult{}, nil
}

// contextAwareCalculator implements CostCalculator with context-cancellation
// awareness and a signal channel for coordinated cancellation testing.
type contextAwareCalculator struct {
	costPerResource float64
	processed       chan struct{}
}

func (m *contextAwareCalculator) GetProjectedCost(
	ctx context.Context,
	resources []engine.ResourceDescriptor,
) ([]engine.CostResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case m.processed <- struct{}{}:
	default:
	}
	results := make([]engine.CostResult, 0, len(resources))
	for _, r := range resources {
		results = append(results, engine.CostResult{
			ResourceType: r.Type,
			ResourceID:   r.ID,
			Adapter:      "mock-calculator",
			Currency:     "USD",
			Monthly:      m.costPerResource,
			Hourly:       m.costPerResource / 730,
		})
	}
	return results, nil
}

func (m *contextAwareCalculator) GetRecommendationsForResources(
	_ context.Context,
	_ []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	return &engine.RecommendationsResult{}, nil
}

// TestAnalyzer_ConcurrentAnalyzeCalls verifies that 5 goroutines can call
// Analyze() concurrently on a single Server instance without data races,
// and that the cost cache remains consistent afterward.
func TestAnalyzer_ConcurrentAnalyzeCalls(t *testing.T) {
	const (
		goroutineCount      = 5
		resourcesPerRoutine = 3
		costPerResource     = 25.0
	)

	calc := &largeStackCalculator{costPerResource: costPerResource}
	srv := analyzer.NewServer(calc, "1.0.0-test")

	_, err := srv.ConfigureStack(context.Background(), &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:   "dev",
		Project: "concurrent-test",
	})
	require.NoError(t, err)

	type analyzeResult struct {
		responses []*pulumirpc.AnalyzeResponse
		err       error
	}

	var wg sync.WaitGroup
	results := make([]analyzeResult, goroutineCount)

	for i := range goroutineCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var resps []*pulumirpc.AnalyzeResponse
			for j := range resourcesPerRoutine {
				req := &pulumirpc.AnalyzeRequest{
					Type: "aws:ec2/instance:Instance",
					Urn:  fmt.Sprintf("urn:pulumi:dev::concurrent-test::aws:ec2/instance:Instance::res-g%d-r%d", idx, j),
				}
				resp, analyzeErr := srv.Analyze(context.Background(), req)
				if analyzeErr != nil {
					results[idx] = analyzeResult{err: analyzeErr}
					return
				}
				resps = append(resps, resp)
			}
			results[idx] = analyzeResult{responses: resps}
		}(i)
	}

	wg.Wait()

	// Verify all goroutines succeeded with valid diagnostics
	for i, r := range results {
		require.NoError(t, r.err, "goroutine %d should not error", i)
		require.Len(t, r.responses, resourcesPerRoutine,
			"goroutine %d should return %d responses", i, resourcesPerRoutine)
		for j, resp := range r.responses {
			require.NotNil(t, resp, "response %d from goroutine %d should not be nil", j, i)
			assert.NotEmpty(t, resp.GetDiagnostics(),
				"response %d from goroutine %d should have diagnostics", j, i)
			for _, diag := range resp.GetDiagnostics() {
				assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel())
			}
		}
	}

	// Verify cost cache consistency via AnalyzeStack summary
	totalResources := goroutineCount * resourcesPerRoutine
	expectedTotal := float64(totalResources) * costPerResource

	stackResp, stackErr := srv.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
	require.NoError(t, stackErr)
	require.NotNil(t, stackResp)
	require.NotEmpty(t, stackResp.GetDiagnostics())

	summaryMsg := stackResp.GetDiagnostics()[0].GetMessage()
	assert.Contains(t, summaryMsg, fmt.Sprintf("%d resources analyzed", totalResources),
		"summary should reflect all %d resources from %d goroutines", totalResources, goroutineCount)
	assert.Contains(t, summaryMsg, fmt.Sprintf("$%.2f USD", expectedTotal),
		"summary should reflect total cost of $%.2f", expectedTotal)

	t.Logf("Concurrent analysis: %d goroutines x %d resources = %d total, cost=$%.2f",
		goroutineCount, resourcesPerRoutine, totalResources, expectedTotal)
}

// TestAnalyzer_PartialPluginFailures verifies that when the cost calculator
// errors for one resource type but succeeds for another, the analyzer returns
// a WARNING diagnostic for the failed type and a cost estimate for the
// successful type, without panicking.
func TestAnalyzer_PartialPluginFailures(t *testing.T) {
	failType := "aws:ec2/instance:Instance"
	successType := "aws:s3/bucket:Bucket"

	calc := &partialFailureCalculator{
		failTypes:       map[string]bool{failType: true},
		costPerResource: 15.0,
	}
	srv := analyzer.NewServer(calc, "1.0.0-test")

	_, err := srv.ConfigureStack(context.Background(), &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:   "dev",
		Project: "partial-failure-test",
	})
	require.NoError(t, err)

	// Analyze failing resource type — should get WARNING diagnostic
	failResp, failErr := srv.Analyze(context.Background(), &pulumirpc.AnalyzeRequest{
		Type: failType,
		Urn:  "urn:pulumi:dev::partial-failure-test::" + failType + "::web-server",
	})
	require.NoError(t, failErr, "Analyze should not return error even for failed calculator")
	require.NotNil(t, failResp)
	require.NotEmpty(t, failResp.GetDiagnostics())

	failDiag := failResp.GetDiagnostics()[0]
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, failDiag.GetEnforcementLevel(),
		"failed resource should have ADVISORY enforcement")
	assert.Contains(t, failDiag.GetMessage(), "Cost calculation failed",
		"failed resource diagnostic should indicate calculation failure")
	assert.Contains(t, failDiag.GetMessage(), failType,
		"failed resource diagnostic should mention the resource type")
	assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM, failDiag.GetSeverity(),
		"warning diagnostic should have MEDIUM severity")

	// Analyze succeeding resource type — should get cost estimate diagnostic
	successResp, successErr := srv.Analyze(context.Background(), &pulumirpc.AnalyzeRequest{
		Type: successType,
		Urn:  "urn:pulumi:dev::partial-failure-test::" + successType + "::data-bucket",
	})
	require.NoError(t, successErr, "Analyze should not return error for successful calculator")
	require.NotNil(t, successResp)
	require.NotEmpty(t, successResp.GetDiagnostics())

	successDiag := successResp.GetDiagnostics()[0]
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, successDiag.GetEnforcementLevel())
	assert.Contains(t, successDiag.GetMessage(), "$15.00 USD",
		"successful resource should have cost estimate")
	assert.NotContains(t, successDiag.GetMessage(), "Cost calculation failed",
		"successful resource should not indicate failure")
	assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW, successDiag.GetSeverity(),
		"cost estimate diagnostic should have LOW severity")

	// Verify mixed results in AnalyzeStack summary — should not panic
	stackResp, stackErr := srv.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
	require.NoError(t, stackErr)
	require.NotNil(t, stackResp)
	require.NotEmpty(t, stackResp.GetDiagnostics())

	// Summary should count only the successful resource (Monthly > 0)
	summaryMsg := stackResp.GetDiagnostics()[0].GetMessage()
	assert.Contains(t, summaryMsg, "1 resources analyzed",
		"summary should count only the successful resource")

	t.Logf("Partial failure: fail=%s, success=%s, summary=%s",
		failType, successType, summaryMsg)
}

// TestAnalyzer_ContextCancellationMidAnalysis verifies that cancelling a
// context mid-analysis causes the analyzer to return gracefully without
// panic and without leaking goroutines.
func TestAnalyzer_ContextCancellationMidAnalysis(t *testing.T) {
	const (
		totalResources  = 50
		cancelAfter     = 10
		costPerResource = 5.0
	)

	calc := &contextAwareCalculator{
		costPerResource: costPerResource,
		processed:       make(chan struct{}, totalResources),
	}
	srv := analyzer.NewServer(calc, "1.0.0-test")

	_, err := srv.ConfigureStack(context.Background(), &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:   "dev",
		Project: "cancellation-test",
	})
	require.NoError(t, err)

	resources := helpers.GenerateSyntheticStack(totalResources)

	// Capture goroutine count before analysis
	runtime.GC()
	goroutinesBefore := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel context after cancelAfter resources are processed
	go func() {
		for i := 0; i < cancelAfter; i++ {
			<-calc.processed
		}
		cancel()
	}()

	// Process all resources — some will see canceled context
	var successCount, canceledCount int
	for _, resource := range resources {
		req := &pulumirpc.AnalyzeRequest{
			Type:       resource.GetType(),
			Urn:        resource.GetUrn(),
			Properties: resource.GetProperties(),
		}

		resp, analyzeErr := srv.Analyze(ctx, req)

		// Analyze should never return an error — it handles errors
		// internally via warning diagnostics
		require.NoError(t, analyzeErr,
			"Analyze should not return error even with canceled context")
		require.NotNil(t, resp)
		require.NotEmpty(t, resp.GetDiagnostics())

		// Distinguish successful cost estimates (LOW severity) from
		// warning diagnostics caused by cancellation (MEDIUM severity)
		diag := resp.GetDiagnostics()[0]
		if diag.GetSeverity() == pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM {
			canceledCount++
		} else {
			successCount++
		}
	}

	// At least cancelAfter resources should have succeeded before cancellation
	assert.GreaterOrEqual(t, successCount, cancelAfter,
		"at least %d resources should succeed before cancellation", cancelAfter)

	// Total responses must equal total resources (all processed, no panics)
	assert.Equal(t, totalResources, successCount+canceledCount,
		"total responses should equal total resources")

	// Check for goroutine leaks
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	goroutineDelta := goroutinesAfter - goroutinesBefore

	assert.LessOrEqual(t, goroutineDelta, 2,
		"goroutine count should not increase significantly (before=%d, after=%d, delta=%d)",
		goroutinesBefore, goroutinesAfter, goroutineDelta)

	t.Logf("Context cancellation: %d succeeded, %d canceled, goroutine delta=%d",
		successCount, canceledCount, goroutineDelta)
}

// TestAnalyzer_UnknownResourceTypes verifies that sending resources with unknown
// types (not handled by the mock calculator) produces advisory warning diagnostics
// rather than hard errors, while known types produce valid cost diagnostics.
func TestAnalyzer_UnknownResourceTypes(t *testing.T) {
	unknownType := "custom:unknown/widget:Widget"
	knownTypes := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
		"aws:rds/instance:Instance",
		"aws:lambda/function:Function",
		"aws:dynamodb/table:Table",
	}

	calc := &partialFailureCalculator{
		failTypes:       map[string]bool{unknownType: true},
		costPerResource: 20.0,
	}
	srv := analyzer.NewServer(calc, "1.0.0-test")

	_, err := srv.ConfigureStack(context.Background(), &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:   "dev",
		Project: "unknown-types-test",
	})
	require.NoError(t, err)

	// Send 10 resources: 5 unknown (indices 0-4), 5 known (indices 5-9)
	var unknownWarnings, knownCosts int
	for i := range 10 {
		var resType string
		if i < 5 {
			resType = unknownType
		} else {
			resType = knownTypes[i-5]
		}

		req := &pulumirpc.AnalyzeRequest{
			Type: resType,
			Urn:  fmt.Sprintf("urn:pulumi:dev::unknown-types-test::%s::resource-%d", resType, i),
		}

		resp, analyzeErr := srv.Analyze(context.Background(), req)
		require.NoError(t, analyzeErr, "Analyze should not return gRPC error for %s", resType)
		require.NotNil(t, resp)
		require.NotEmpty(t, resp.GetDiagnostics())

		diag := resp.GetDiagnostics()[0]
		assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel(),
			"all diagnostics should be ADVISORY for resource %s", resType)

		if i < 5 {
			// Unknown types should get warning diagnostics (MEDIUM severity)
			assert.Contains(t, diag.GetMessage(), "Cost calculation failed",
				"unknown type %s should produce failure diagnostic", resType)
			assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM, diag.GetSeverity(),
				"warning diagnostic should have MEDIUM severity")
			unknownWarnings++
		} else {
			// Known types should get cost estimates (LOW severity)
			assert.Contains(t, diag.GetMessage(), "$20.00 USD",
				"known type %s should produce cost estimate", resType)
			assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW, diag.GetSeverity(),
				"cost estimate diagnostic should have LOW severity")
			knownCosts++
		}
	}

	assert.Equal(t, 5, unknownWarnings, "should have 5 warning diagnostics for unknown types")
	assert.Equal(t, 5, knownCosts, "should have 5 cost diagnostics for known types")

	// Verify mixed results in AnalyzeStack summary
	stackResp, stackErr := srv.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
	require.NoError(t, stackErr)
	require.NotNil(t, stackResp)
	require.NotEmpty(t, stackResp.GetDiagnostics())

	// Summary should count only successful resources (with Monthly > 0)
	summaryMsg := stackResp.GetDiagnostics()[0].GetMessage()
	assert.Contains(t, summaryMsg, "5 resources analyzed",
		"summary should count only the 5 known resources")

	t.Logf("Unknown types: %d warnings, %d costs, summary=%s",
		unknownWarnings, knownCosts, summaryMsg)

	// Sub-test: zero priceable resources (empty AnalyzeStack)
	t.Run("ZeroPriceableResources", func(t *testing.T) {
		// Create fresh server with empty cost cache (no prior Analyze calls)
		freshCalc := &largeStackCalculator{costPerResource: 10.0}
		freshSrv := analyzer.NewServer(freshCalc, "1.0.0-test")

		_, cfgErr := freshSrv.ConfigureStack(context.Background(), &pulumirpc.AnalyzerStackConfigureRequest{
			Stack:   "dev",
			Project: "empty-stack-test",
		})
		require.NoError(t, cfgErr)

		// Call AnalyzeStack without any prior Analyze() calls — empty cost cache
		stackResp, stackErr := freshSrv.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{
			Resources: []*pulumirpc.AnalyzerResource{},
		})
		require.NoError(t, stackErr, "AnalyzeStack should not panic or error on empty resources")
		require.NotNil(t, stackResp, "response should not be nil")
		require.NotEmpty(t, stackResp.GetDiagnostics(), "should return at least summary diagnostic")

		// Summary should indicate 0 resources analyzed
		summaryMsg := stackResp.GetDiagnostics()[0].GetMessage()
		assert.Contains(t, summaryMsg, "0 resources analyzed",
			"empty stack summary should indicate 0 resources")
	})
}
