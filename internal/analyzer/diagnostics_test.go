package analyzer

import (
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestCostToDiagnostic(t *testing.T) {
	tests := []struct {
		name         string
		cost         engine.CostResult
		urn          string
		version      string
		wantSeverity pulumirpc.PolicySeverity
		wantContains string
	}{
		{
			name: "successful cost calculation",
			cost: engine.CostResult{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "webserver",
				Adapter:      "local-spec",
				Currency:     "USD",
				Monthly:      8.45,
				Hourly:       0.0116,
			},
			urn:          "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
			version:      "0.1.0",
			wantSeverity: pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW,
			wantContains: "$8.45 USD",
		},
		{
			name: "zero cost with notes (fallback)",
			cost: engine.CostResult{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "webserver",
				Adapter:      "none",
				Currency:     "USD",
				Monthly:      0,
				Notes:        "Unable to estimate: unsupported resource type",
			},
			urn:          "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
			version:      "0.1.0",
			wantSeverity: pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM,
			wantContains: "Unable to estimate",
		},
		{
			name: "zero cost no notes",
			cost: engine.CostResult{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "webserver",
				Adapter:      "none",
				Currency:     "USD",
				Monthly:      0,
			},
			urn:          "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
			version:      "0.1.0",
			wantSeverity: pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW,
			wantContains: "Unable to estimate cost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := CostToDiagnostic(tt.cost, tt.urn, tt.version)

			require.NotNil(t, diag)
			assert.Equal(t, policyNameCost, diag.GetPolicyName())
			assert.Equal(t, policyPackName, diag.GetPolicyPackName())
			assert.Equal(t, tt.version, diag.GetPolicyPackVersion())
			assert.Equal(t, tt.urn, diag.GetUrn())
			assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel())
			assert.Equal(t, tt.wantSeverity, diag.GetSeverity())
			assert.Contains(t, diag.GetMessage(), tt.wantContains)
		})
	}
}

func TestStackSummaryDiagnostic(t *testing.T) {
	tests := []struct {
		name         string
		costs        []engine.CostResult
		version      string
		wantTotal    string
		wantAnalyzed string
	}{
		{
			name: "multiple resources",
			costs: []engine.CostResult{
				{Monthly: 8.45, Currency: "USD"},
				{Monthly: 25.00, Currency: "USD"},
				{Monthly: 0.50, Currency: "USD"},
			},
			version:      "0.1.0",
			wantTotal:    "$33.95 USD",
			wantAnalyzed: "3 resources analyzed",
		},
		{
			name: "mixed costs (some zero)",
			costs: []engine.CostResult{
				{Monthly: 10.00, Currency: "USD"},
				{Monthly: 0, Currency: "USD"},
				{Monthly: 20.00, Currency: "USD"},
			},
			version:      "0.1.0",
			wantTotal:    "$30.00 USD",
			wantAnalyzed: "2 resources analyzed",
		},
		{
			name:         "empty costs",
			costs:        []engine.CostResult{},
			version:      "0.1.0",
			wantTotal:    "$0.00 USD",
			wantAnalyzed: "0 resources analyzed",
		},
		{
			name:         "nil costs",
			costs:        nil,
			version:      "0.1.0",
			wantTotal:    "$0.00 USD",
			wantAnalyzed: "0 resources analyzed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := StackSummaryDiagnostic(tt.costs, tt.version)

			require.NotNil(t, diag)
			assert.Equal(t, policyNameSum, diag.GetPolicyName())
			assert.Equal(t, policyPackName, diag.GetPolicyPackName())
			assert.Equal(t, tt.version, diag.GetPolicyPackVersion())
			assert.Empty(t, diag.GetUrn()) // Stack-level has no URN
			assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel())
			assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW, diag.GetSeverity())
			assert.Contains(t, diag.GetMessage(), tt.wantTotal)
			assert.Contains(t, diag.GetMessage(), tt.wantAnalyzed)
		})
	}
}

func TestFormatCostMessage(t *testing.T) {
	tests := []struct {
		name string
		cost engine.CostResult
		want string
	}{
		{
			name: "has monthly cost",
			cost: engine.CostResult{
				Monthly:  25.50,
				Currency: "USD",
				Adapter:  "vantage",
			},
			want: "Estimated Monthly Cost: $25.50 USD (source: vantage)",
		},
		{
			name: "zero cost with notes",
			cost: engine.CostResult{
				Monthly: 0,
				Notes:   "Plugin returned no pricing data",
			},
			want: "Plugin returned no pricing data",
		},
		{
			name: "zero cost no notes",
			cost: engine.CostResult{
				Monthly: 0,
			},
			want: "Unable to estimate cost",
		},
		{
			name: "small cost",
			cost: engine.CostResult{
				Monthly:  0.01,
				Currency: "USD",
				Adapter:  "local-spec",
			},
			want: "Estimated Monthly Cost: $0.01 USD (source: local-spec)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCostMessage(tt.cost)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCostToDiagnostic_EnforcementLevel(t *testing.T) {
	// Verify all cost diagnostics use ADVISORY (never ERROR in MVP)
	cost := engine.CostResult{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "expensive-server",
		Monthly:      10000.00, // Very expensive
		Currency:     "USD",
	}

	diag := CostToDiagnostic(
		cost,
		"urn:pulumi:dev::myapp::aws:ec2/instance:Instance::expensive-server",
		"0.1.0",
	)

	// Must be ADVISORY per FR-005
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel())
	// Should never be ERROR, even for high costs
	assert.NotEqual(t, pulumirpc.EnforcementLevel_MANDATORY, diag.GetEnforcementLevel())
}

func TestStackSummaryDiagnostic_CurrencyHandling(t *testing.T) {
	// Test that currency is properly extracted from results
	costs := []engine.CostResult{
		{Monthly: 10.00, Currency: ""},
		{Monthly: 20.00, Currency: "EUR"},
		{Monthly: 30.00, Currency: "USD"},
	}

	diag := StackSummaryDiagnostic(costs, "0.1.0")

	// Should use the last non-empty currency
	assert.Contains(t, diag.GetMessage(), "USD")
}

// Phase 5 (US3) - Warning Diagnostic Tests

func TestWarningDiagnostic(t *testing.T) {
	tests := []struct {
		name    string
		message string
		urn     string
		version string
	}{
		{
			name:    "plugin timeout warning",
			message: "Cost estimation timed out, using fallback pricing",
			urn:     "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web",
			version: "0.1.0",
		},
		{
			name:    "network failure warning",
			message: "Network unavailable, using cached pricing specs",
			urn:     "urn:pulumi:dev::myapp::aws:rds/instance:Instance::db",
			version: "0.1.0",
		},
		{
			name:    "unsupported resource warning",
			message: "Unsupported resource type: custom:component:Widget",
			urn:     "urn:pulumi:dev::myapp::custom:component:Widget::w1",
			version: "0.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := WarningDiagnostic(tt.message, tt.urn, tt.version)

			require.NotNil(t, diag)
			assert.Equal(t, policyNameCost, diag.GetPolicyName())
			assert.Equal(t, policyPackName, diag.GetPolicyPackName())
			assert.Equal(t, tt.version, diag.GetPolicyPackVersion())
			assert.Equal(t, tt.urn, diag.GetUrn())
			assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel())
			assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM, diag.GetSeverity())
			assert.Equal(t, tt.message, diag.GetMessage())
		})
	}
}

func TestWarningDiagnostic_NoURN(t *testing.T) {
	// Stack-level warnings have no URN
	diag := WarningDiagnostic("Unable to connect to pricing API", "", "0.1.0")

	require.NotNil(t, diag)
	assert.Empty(t, diag.GetUrn())
	assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM, diag.GetSeverity())
}

func TestCostToDiagnostic_ErrorInNotes(t *testing.T) {
	// When cost calculation fails, the error appears in Notes
	cost := engine.CostResult{
		ResourceType: "aws:lambda/function:Function",
		ResourceID:   "api-handler",
		Adapter:      "none",
		Currency:     "USD",
		Monthly:      0,
		Notes:        "ERROR: Plugin vantage failed: connection refused",
	}

	diag := CostToDiagnostic(
		cost,
		"urn:pulumi:dev::myapp::aws:lambda/function:Function::api-handler",
		"0.1.0",
	)

	// Should report the error in the message
	assert.Contains(t, diag.GetMessage(), "ERROR")
	assert.Contains(t, diag.GetMessage(), "Plugin vantage failed")
	// Should use MEDIUM severity for errors
	assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM, diag.GetSeverity())
}

// Phase 2 (Foundational) - Recommendation Formatting Tests

func TestFormatRecommendation(t *testing.T) {
	tests := []struct {
		name string
		rec  engine.Recommendation
		want string
	}{
		{
			name: "recommendation with savings",
			rec: engine.Recommendation{
				Type:             "Right-sizing",
				Description:      "Switch to t3.small",
				EstimatedSavings: 15.00,
				Currency:         "USD",
			},
			want: "Right-sizing: Switch to t3.small (save $15.00/mo)",
		},
		{
			name: "recommendation without savings",
			rec: engine.Recommendation{
				Type:        "Review",
				Description: "Consider adjusting storage class",
			},
			want: "Review: Consider adjusting storage class",
		},
		{
			name: "recommendation with zero savings",
			rec: engine.Recommendation{
				Type:             "Terminate",
				Description:      "Remove idle instance",
				EstimatedSavings: 0,
				Currency:         "USD",
			},
			want: "Terminate: Remove idle instance",
		},
		{
			name: "recommendation with non-USD currency",
			rec: engine.Recommendation{
				Type:             "Right-sizing",
				Description:      "Downgrade to smaller instance",
				EstimatedSavings: 25.50,
				Currency:         "EUR",
			},
			want: "Right-sizing: Downgrade to smaller instance (save €25.50/mo)",
		},
		{
			name: "recommendation with small savings",
			rec: engine.Recommendation{
				Type:             "Delete Unused",
				Description:      "Remove orphaned EBS volume",
				EstimatedSavings: 0.50,
				Currency:         "USD",
			},
			want: "Delete Unused: Remove orphaned EBS volume (save $0.50/mo)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecommendation(tt.rec)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatRecommendations(t *testing.T) {
	tests := []struct {
		name string
		recs []engine.Recommendation
		want string
	}{
		{
			name: "single recommendation",
			recs: []engine.Recommendation{
				{
					Type:             "Right-sizing",
					Description:      "Switch to t3.small",
					EstimatedSavings: 15.00,
					Currency:         "USD",
				},
			},
			want: "Recommendations: Right-sizing: Switch to t3.small (save $15.00/mo)",
		},
		{
			name: "two recommendations",
			recs: []engine.Recommendation{
				{
					Type:             "Right-sizing",
					Description:      "Switch to t3.small",
					EstimatedSavings: 15.00,
					Currency:         "USD",
				},
				{
					Type:             "Terminate",
					Description:      "Remove idle instance",
					EstimatedSavings: 100.00,
					Currency:         "USD",
				},
			},
			want: "Recommendations: Right-sizing: Switch to t3.small (save $15.00/mo); " +
				"Terminate: Remove idle instance (save $100.00/mo)",
		},
		{
			name: "three recommendations (at limit)",
			recs: []engine.Recommendation{
				{
					Type: "Right-sizing", Description: "Switch to t3.small",
					EstimatedSavings: 15.00, Currency: "USD",
				},
				{
					Type: "Terminate", Description: "Remove idle instance",
					EstimatedSavings: 100.00, Currency: "USD",
				},
				{
					Type: "Delete Unused", Description: "Remove orphaned volume",
					EstimatedSavings: 5.00, Currency: "USD",
				},
			},
			want: "Recommendations: " +
				"Right-sizing: Switch to t3.small (save $15.00/mo); " +
				"Terminate: Remove idle instance (save $100.00/mo); " +
				"Delete Unused: Remove orphaned volume (save $5.00/mo)",
		},
		{
			name: "four recommendations (exceeds limit)",
			recs: []engine.Recommendation{
				{
					Type: "Right-sizing", Description: "Switch to t3.small",
					EstimatedSavings: 15.00, Currency: "USD",
				},
				{
					Type: "Terminate", Description: "Remove idle instance",
					EstimatedSavings: 100.00, Currency: "USD",
				},
				{
					Type: "Delete Unused", Description: "Remove orphaned volume",
					EstimatedSavings: 5.00, Currency: "USD",
				},
				{
					Type: "Purchase Commitment", Description: "Buy reserved",
					EstimatedSavings: 200.00, Currency: "USD",
				},
			},
			want: "Recommendations: " +
				"Right-sizing: Switch to t3.small (save $15.00/mo); " +
				"Terminate: Remove idle instance (save $100.00/mo); " +
				"Delete Unused: Remove orphaned volume (save $5.00/mo); " +
				"and 1 more",
		},
		{
			name: "six recommendations (multiple beyond limit)",
			recs: []engine.Recommendation{
				{
					Type: "Right-sizing", Description: "Switch to t3.small",
					EstimatedSavings: 15.00, Currency: "USD",
				},
				{
					Type: "Terminate", Description: "Remove idle instance",
					EstimatedSavings: 100.00, Currency: "USD",
				},
				{
					Type: "Delete Unused", Description: "Remove orphaned volume",
					EstimatedSavings: 5.00, Currency: "USD",
				},
				{
					Type: "Purchase Commitment", Description: "Buy reserved",
					EstimatedSavings: 200.00, Currency: "USD",
				},
				{
					Type: "Adjust Requests", Description: "Lower CPU requests",
					EstimatedSavings: 10.00, Currency: "USD",
				},
				{Type: "Review", Description: "Consider spot instances"},
			},
			want: "Recommendations: " +
				"Right-sizing: Switch to t3.small (save $15.00/mo); " +
				"Terminate: Remove idle instance (save $100.00/mo); " +
				"Delete Unused: Remove orphaned volume (save $5.00/mo); " +
				"and 3 more",
		},
		{
			name: "empty recommendations",
			recs: []engine.Recommendation{},
			want: "",
		},
		{
			name: "nil recommendations",
			recs: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecommendations(tt.recs)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Phase 3 (US1) - CostToDiagnostic with Recommendations Tests

func TestCostToDiagnostic_SingleRecommendation(t *testing.T) {
	// T008: Test CostToDiagnostic with a single recommendation
	cost := engine.CostResult{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "webserver",
		Adapter:      "aws-plugin",
		Currency:     "USD",
		Monthly:      25.50,
		Hourly:       0.035,
		Recommendations: []engine.Recommendation{
			{
				Type:             "Right-sizing",
				Description:      "Switch to t3.small",
				EstimatedSavings: 15.00,
				Currency:         "USD",
			},
		},
	}

	diag := CostToDiagnostic(
		cost,
		"urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
		"0.1.0",
	)

	require.NotNil(t, diag)
	assert.Contains(t, diag.GetMessage(), "$25.50 USD")
	assert.Contains(t, diag.GetMessage(), "Recommendations:")
	assert.Contains(t, diag.GetMessage(), "Right-sizing: Switch to t3.small")
	assert.Contains(t, diag.GetMessage(), "save $15.00/mo")
}

func TestCostToDiagnostic_MultipleRecommendations(t *testing.T) {
	// T009: Test CostToDiagnostic with multiple recommendations
	cost := engine.CostResult{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "webserver",
		Adapter:      "aws-plugin",
		Currency:     "USD",
		Monthly:      150.00,
		Hourly:       0.205,
		Recommendations: []engine.Recommendation{
			{
				Type:             "Right-sizing",
				Description:      "Switch to t3.medium",
				EstimatedSavings: 50.00,
				Currency:         "USD",
			},
			{
				Type:             "Terminate",
				Description:      "Remove idle instance",
				EstimatedSavings: 100.00,
				Currency:         "USD",
			},
		},
	}

	diag := CostToDiagnostic(
		cost,
		"urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
		"0.1.0",
	)

	require.NotNil(t, diag)
	assert.Contains(t, diag.GetMessage(), "$150.00 USD")
	assert.Contains(t, diag.GetMessage(), "Right-sizing: Switch to t3.medium")
	assert.Contains(t, diag.GetMessage(), "Terminate: Remove idle instance")
	// Recommendations should be separated by semicolon
	assert.Contains(t, diag.GetMessage(), "; ")
}

func TestCostToDiagnostic_NoRecommendations(t *testing.T) {
	// T010: Test CostToDiagnostic with no recommendations (empty slice)
	cost := engine.CostResult{
		ResourceType:    "aws:ec2/instance:Instance",
		ResourceID:      "webserver",
		Adapter:         "aws-plugin",
		Currency:        "USD",
		Monthly:         25.50,
		Hourly:          0.035,
		Recommendations: []engine.Recommendation{},
	}

	diag := CostToDiagnostic(
		cost,
		"urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
		"0.1.0",
	)

	require.NotNil(t, diag)
	assert.Contains(t, diag.GetMessage(), "$25.50 USD")
	// Should NOT contain recommendations section when empty
	assert.NotContains(t, diag.GetMessage(), "Recommendations:")
}

func TestCostToDiagnostic_RecommendationsWithSustainability(t *testing.T) {
	// T011: Test recommendations combined with sustainability metrics
	cost := engine.CostResult{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "webserver",
		Adapter:      "aws-plugin",
		Currency:     "USD",
		Monthly:      25.50,
		Hourly:       0.035,
		Sustainability: map[string]engine.SustainabilityMetric{
			"gCO2e": {Value: 12.5, Unit: "gCO2e/month"},
		},
		Recommendations: []engine.Recommendation{
			{
				Type:             "Right-sizing",
				Description:      "Switch to t3.small",
				EstimatedSavings: 15.00,
				Currency:         "USD",
			},
		},
	}

	diag := CostToDiagnostic(
		cost,
		"urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
		"0.1.0",
	)

	require.NotNil(t, diag)
	// Should contain both sustainability and recommendations
	assert.Contains(t, diag.GetMessage(), "$25.50 USD")
	assert.Contains(t, diag.GetMessage(), "Carbon: 12.50 gCO2e/month")
	assert.Contains(t, diag.GetMessage(), "Recommendations:")
	assert.Contains(t, diag.GetMessage(), "Right-sizing: Switch to t3.small")
}

func TestCostToDiagnostic_RecommendationsADVISORYEnforcement(t *testing.T) {
	// T012: Verify ADVISORY enforcement level for recommendation diagnostics (FR-008)
	testCases := []struct {
		name string
		cost engine.CostResult
	}{
		{
			name: "single recommendation",
			cost: engine.CostResult{
				Monthly:  100.00,
				Currency: "USD",
				Recommendations: []engine.Recommendation{
					{
						Type: "Right-sizing", Description: "Test",
						EstimatedSavings: 50.00, Currency: "USD",
					},
				},
			},
		},
		{
			name: "multiple recommendations",
			cost: engine.CostResult{
				Monthly:  200.00,
				Currency: "USD",
				Recommendations: []engine.Recommendation{
					{
						Type: "Right-sizing", Description: "Test1",
						EstimatedSavings: 50.00, Currency: "USD",
					},
					{
						Type: "Terminate", Description: "Test2",
						EstimatedSavings: 100.00, Currency: "USD",
					},
				},
			},
		},
		{
			name: "high savings recommendation",
			cost: engine.CostResult{
				Monthly:  10000.00,
				Currency: "USD",
				Recommendations: []engine.Recommendation{
					{
						Type: "Right-sizing", Description: "Major savings",
						EstimatedSavings: 5000.00, Currency: "USD",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diag := CostToDiagnostic(
				tc.cost,
				"urn:pulumi:dev::myapp::aws:ec2/instance:Instance::test",
				"0.1.0",
			)

			// FR-008: All diagnostics MUST use ADVISORY enforcement level
			assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel(),
				"recommendation diagnostics must use ADVISORY enforcement level")
			// Must NOT use MANDATORY (which would block deployments)
			assert.NotEqual(t, pulumirpc.EnforcementLevel_MANDATORY, diag.GetEnforcementLevel(),
				"recommendation diagnostics must never use MANDATORY enforcement")
		})
	}
}

// Phase 4 (US2) - Stack Summary with Recommendations Tests

func TestStackSummaryDiagnostic_WithRecommendations(t *testing.T) {
	// T017: Test StackSummaryDiagnostic includes recommendation summary
	costs := []engine.CostResult{
		{
			Monthly:  50.00,
			Currency: "USD",
			Recommendations: []engine.Recommendation{
				{
					Type: "Right-sizing", Description: "Switch to smaller instance",
					EstimatedSavings: 15.00, Currency: "USD",
				},
			},
		},
		{
			Monthly:  100.00,
			Currency: "USD",
			Recommendations: []engine.Recommendation{
				{
					Type: "Terminate", Description: "Remove idle resource",
					EstimatedSavings: 100.00, Currency: "USD",
				},
				{
					Type: "Delete Unused", Description: "Remove orphaned storage",
					EstimatedSavings: 10.00, Currency: "USD",
				},
			},
		},
		{
			Monthly:         25.00,
			Currency:        "USD",
			Recommendations: []engine.Recommendation{}, // No recommendations
		},
	}

	diag := StackSummaryDiagnostic(costs, "0.1.0")

	require.NotNil(t, diag)
	// Should show total cost
	assert.Contains(t, diag.GetMessage(), "$175.00 USD")
	// Should show recommendation count (3 total recommendations across 2 resources)
	assert.Contains(t, diag.GetMessage(), "3 recommendations")
	// Should show potential savings ($15 + $100 + $10 = $125)
	assert.Contains(t, diag.GetMessage(), "$125.00")
	assert.Contains(t, diag.GetMessage(), "potential savings")
}

func TestStackSummaryDiagnostic_AggregateSavingsSameCurrency(t *testing.T) {
	// T018: Test aggregate savings calculation with same currency
	costs := []engine.CostResult{
		{
			Monthly:  100.00,
			Currency: "USD",
			Recommendations: []engine.Recommendation{
				{
					Type: "Right-sizing", Description: "Test1",
					EstimatedSavings: 25.00, Currency: "USD",
				},
			},
		},
		{
			Monthly:  200.00,
			Currency: "USD",
			Recommendations: []engine.Recommendation{
				{
					Type: "Terminate", Description: "Test2",
					EstimatedSavings: 75.00, Currency: "USD",
				},
				{
					Type: "Delete", Description: "Test3",
					EstimatedSavings: 50.00, Currency: "USD",
				},
			},
		},
	}

	diag := StackSummaryDiagnostic(costs, "0.1.0")

	require.NotNil(t, diag)
	// Total savings: $25 + $75 + $50 = $150
	assert.Contains(t, diag.GetMessage(), "$150.00")
	// 3 total recommendations
	assert.Contains(t, diag.GetMessage(), "3 recommendations")
}

func TestStackSummaryDiagnostic_MixedCurrencyHandling(t *testing.T) {
	// T019: Test mixed currency handling in stack summary
	costs := []engine.CostResult{
		{
			Monthly:  100.00,
			Currency: "USD",
			Recommendations: []engine.Recommendation{
				{
					Type: "Right-sizing", Description: "Test1",
					EstimatedSavings: 25.00, Currency: "USD",
				},
			},
		},
		{
			Monthly:  200.00,
			Currency: "EUR",
			Recommendations: []engine.Recommendation{
				{
					Type: "Terminate", Description: "Test2",
					EstimatedSavings: 75.00, Currency: "EUR",
				},
			},
		},
	}

	diag := StackSummaryDiagnostic(costs, "0.1.0")

	require.NotNil(t, diag)
	// Should show recommendation count
	assert.Contains(t, diag.GetMessage(), "2 recommendations")
	// Should indicate mixed currencies (not aggregate a numeric total)
	assert.Contains(t, diag.GetMessage(), "mixed currencies")
}

func TestStackSummaryDiagnostic_NoRecommendations(t *testing.T) {
	// Additional test: Stack summary without any recommendations
	costs := []engine.CostResult{
		{Monthly: 50.00, Currency: "USD", Recommendations: nil},
		{Monthly: 100.00, Currency: "USD", Recommendations: []engine.Recommendation{}},
	}

	diag := StackSummaryDiagnostic(costs, "0.1.0")

	require.NotNil(t, diag)
	assert.Contains(t, diag.GetMessage(), "$150.00 USD")
	// Should NOT contain recommendation info when there are none
	assert.NotContains(t, diag.GetMessage(), "recommendations")
	assert.NotContains(t, diag.GetMessage(), "potential savings")
}

// Phase 5 (US3) - Graceful Handling Tests

func TestFormatRecommendations_NilSlice(t *testing.T) {
	// T024: Test nil Recommendations slice handling
	var recs []engine.Recommendation = nil
	result := formatRecommendations(recs)
	assert.Equal(t, "", result, "nil recommendations should return empty string")
}

func TestFormatRecommendations_EmptySlice(t *testing.T) {
	// T025: Test empty Recommendations slice handling
	recs := []engine.Recommendation{}
	result := formatRecommendations(recs)
	assert.Equal(t, "", result, "empty recommendations should return empty string")
}

func TestFormatRecommendation_ZeroSavings(t *testing.T) {
	// T026: Test recommendation with zero savings
	rec := engine.Recommendation{
		Type:             "Review",
		Description:      "Check resource utilization",
		EstimatedSavings: 0,
		Currency:         "USD",
	}
	result := formatRecommendation(rec)
	assert.Equal(t, "Review: Check resource utilization", result)
	// Should NOT contain savings info when zero
	assert.NotContains(t, result, "save")
	assert.NotContains(t, result, "$")
}

func TestFormatRecommendation_EmptyDescription(t *testing.T) {
	// T027: Test recommendation with empty description
	// Note: formatRecommendation still formats it, but formatRecommendations
	// will filter it out as malformed
	rec := engine.Recommendation{
		Type:             "Right-sizing",
		Description:      "",
		EstimatedSavings: 15.00,
		Currency:         "USD",
	}
	result := formatRecommendation(rec)
	// formatRecommendation formats it as-is (validation happens at list level)
	assert.Equal(t, "Right-sizing:  (save $15.00/mo)", result)
}

func TestFormatRecommendations_FiltersInvalid(t *testing.T) {
	// Test that formatRecommendations filters out invalid recommendations
	recs := []engine.Recommendation{
		{Type: "", Description: "No type"},                    // Invalid: empty type
		{Type: "Valid", Description: "Has both"},              // Valid
		{Type: "NoDesc", Description: ""},                     // Invalid: empty description
		{Type: "", Description: ""},                           // Invalid: both empty
		{Type: "AlsoValid", Description: "Another valid one"}, // Valid
	}

	result := formatRecommendations(recs)

	// Should only include the 2 valid recommendations
	assert.Contains(t, result, "Valid: Has both")
	assert.Contains(t, result, "AlsoValid: Another valid one")
	// Should NOT include invalid ones
	assert.NotContains(t, result, "No type")
	assert.NotContains(t, result, "NoDesc")
}

func TestFormatRecommendations_SkipsMalformed(t *testing.T) {
	// T027 extension: Test that malformed recommendations are skipped
	recs := []engine.Recommendation{
		{Type: "", Description: "", EstimatedSavings: 10.00, Currency: "USD"}, // Both empty
		{
			Type: "Right-sizing", Description: "Valid recommendation",
			EstimatedSavings: 15.00, Currency: "USD",
		},
		{Type: "Delete", Description: ""}, // Empty description only
	}
	result := formatRecommendations(recs)
	// Should include valid recommendation and skip malformed ones
	assert.Contains(t, result, "Right-sizing: Valid recommendation")
	// The count should reflect only valid ones (implementation may vary)
}

func TestCostToDiagnostic_NilRecommendations(t *testing.T) {
	// Test CostToDiagnostic with nil recommendations (graceful handling)
	cost := engine.CostResult{
		ResourceType:    "aws:ec2/instance:Instance",
		ResourceID:      "webserver",
		Adapter:         "aws-plugin",
		Currency:        "USD",
		Monthly:         25.50,
		Recommendations: nil,
	}

	diag := CostToDiagnostic(
		cost,
		"urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
		"0.1.0",
	)

	require.NotNil(t, diag)
	assert.Contains(t, diag.GetMessage(), "$25.50 USD")
	// Should NOT contain recommendations when nil
	assert.NotContains(t, diag.GetMessage(), "Recommendations:")
}

func TestAggregateRecommendations_EmptyCosts(t *testing.T) {
	// Test aggregation with empty costs slice
	costs := []engine.CostResult{}
	agg := AggregateRecommendations(costs)

	assert.Equal(t, 0, agg.Count)
	assert.Equal(t, 0.0, agg.TotalSavings)
	assert.Equal(t, "USD", agg.Currency) // Default
	assert.False(t, agg.MixedCurrencies)
}

func TestAggregateRecommendations_NilCosts(t *testing.T) {
	// Test aggregation with nil costs slice
	var costs []engine.CostResult = nil
	agg := AggregateRecommendations(costs)

	assert.Equal(t, 0, agg.Count)
	assert.Equal(t, 0.0, agg.TotalSavings)
	assert.Equal(t, "USD", agg.Currency) // Default
	assert.False(t, agg.MixedCurrencies)
}

// Phase 5 (US3) - Carbon Equivalency Tests for Analyzer Diagnostics

func TestFormatCostMessage_WithCarbonEquivalencies(t *testing.T) {
	// T034: Test formatCostMessage displays carbon equivalencies in compact format
	cost := engine.CostResult{
		Monthly:  100.0,
		Currency: "USD",
		Adapter:  "aws-public",
		Sustainability: map[string]engine.SustainabilityMetric{
			"carbon_footprint": {Value: 150.0, Unit: "kg"},
		},
	}

	msg := formatCostMessage(cost)

	// Should contain cost info
	assert.Contains(t, msg, "$100.00 USD")
	assert.Contains(t, msg, "aws-public")

	// Should contain carbon equivalencies in compact format
	assert.Contains(t, msg, "≈")
	assert.Contains(t, msg, "mi")
	assert.Contains(t, msg, "phones")
	// Should contain expected values (150/0.192 ≈ 781 miles, 150/0.00822 ≈ 18248 phones)
	assert.Contains(t, msg, "781")
	assert.Contains(t, msg, "18,248")
}

func TestFormatCostMessage_CompactFormatForAnalyzer(t *testing.T) {
	// T035: Test that analyzer uses compact format (≈ X mi, Y phones)
	cost := engine.CostResult{
		Monthly:  50.0,
		Currency: "USD",
		Adapter:  "local-spec",
		Sustainability: map[string]engine.SustainabilityMetric{
			"carbon_footprint": {Value: 100.0, Unit: "kg"},
		},
	}

	msg := formatCostMessage(cost)

	// Compact format should use:
	// - ≈ symbol instead of "Equivalent to"
	// - "mi" instead of "miles driven"
	// - "phones" instead of "smartphones charged"
	assert.Contains(t, msg, "≈")
	assert.Contains(t, msg, "mi")
	assert.Contains(t, msg, "phones")
	// Should NOT contain verbose format elements
	assert.NotContains(t, msg, "Equivalent to")
	assert.NotContains(t, msg, "miles driven")
	assert.NotContains(t, msg, "smartphones charged")
}

func TestFormatCostMessage_OmitsEquivalenciesBelowThreshold(t *testing.T) {
	// Equivalencies should be omitted when carbon is below threshold (1 kg)
	cost := engine.CostResult{
		Monthly:  50.0,
		Currency: "USD",
		Adapter:  "local-spec",
		Sustainability: map[string]engine.SustainabilityMetric{
			"carbon_footprint": {Value: 0.5, Unit: "kg"}, // Below 1kg threshold
		},
	}

	msg := formatCostMessage(cost)

	// Should contain carbon metric
	assert.Contains(t, msg, "carbon_footprint")
	// Should NOT contain equivalency text
	assert.NotContains(t, msg, "≈")
	assert.NotContains(t, msg, "mi")
	assert.NotContains(t, msg, "phones")
}

func TestFormatCostMessage_OmitsEquivalenciesWhenNoCarbon(t *testing.T) {
	// Equivalencies should be omitted when no carbon data
	cost := engine.CostResult{
		Monthly:  50.0,
		Currency: "USD",
		Adapter:  "local-spec",
		Sustainability: map[string]engine.SustainabilityMetric{
			"energy_consumption": {Value: 2000.0, Unit: "kWh"},
		},
	}

	msg := formatCostMessage(cost)

	// Should contain energy metric
	assert.Contains(t, msg, "energy_consumption")
	// Should NOT contain equivalency text
	assert.NotContains(t, msg, "≈")
	assert.NotContains(t, msg, "mi")
	assert.NotContains(t, msg, "phones")
}

func TestFormatCostMessage_LargeCarbon_MillionScaling(t *testing.T) {
	// Large carbon values should use million scaling
	cost := engine.CostResult{
		Monthly:  1000000.0,
		Currency: "USD",
		Adapter:  "enterprise",
		Sustainability: map[string]engine.SustainabilityMetric{
			"carbon_footprint": {Value: 10000000.0, Unit: "kg"}, // 10 million kg
		},
	}

	msg := formatCostMessage(cost)

	// Should use "million" abbreviation for large values
	assert.Contains(t, msg, "million")
}
