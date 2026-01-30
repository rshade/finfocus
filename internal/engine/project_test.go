package engine

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderResults tests the main rendering dispatcher.
func TestRenderResults(t *testing.T) {
	tests := []struct {
		name    string
		format  OutputFormat
		results []CostResult
		wantErr bool
	}{
		{
			name:    "table format with empty results",
			format:  OutputTable,
			results: []CostResult{},
			wantErr: false,
		},
		{
			name:   "table format with single resource",
			format: OutputTable,
			results: []CostResult{
				{
					ResourceType: "aws:ec2:Instance",
					ResourceID:   "i-123",
					Adapter:      "kubecost",
					Currency:     "USD",
					Monthly:      73.00,
					Hourly:       0.10,
				},
			},
			wantErr: false,
		},
		{
			name:   "json format with results",
			format: OutputJSON,
			results: []CostResult{
				{ResourceType: "aws:ec2:Instance", Monthly: 50.00, Currency: "USD"},
			},
			wantErr: false,
		},
		{
			name:   "ndjson format with results",
			format: OutputNDJSON,
			results: []CostResult{
				{ResourceType: "aws:ec2:Instance", Monthly: 50.00, Currency: "USD"},
				{ResourceType: "aws:rds:Instance", Monthly: 100.00, Currency: "USD"},
			},
			wantErr: false,
		},
		{
			name:    "unsupported format",
			format:  OutputFormat("invalid"),
			results: []CostResult{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RenderResults(io.Discard, tt.format, tt.results)
			if (err != nil) != tt.wantErr {
				t.Errorf("RenderResults() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestRenderActualCostResults tests actual cost rendering.
func TestRenderActualCostResults(t *testing.T) {
	tests := []struct {
		name    string
		format  OutputFormat
		results []CostResult
		wantErr bool
	}{
		{
			name:    "table format with actual costs",
			format:  OutputTable,
			results: []CostResult{{TotalCost: 100.0, CostPeriod: "monthly", Currency: "USD"}},
			wantErr: false,
		},
		{
			name:    "json format with actual costs",
			format:  OutputJSON,
			results: []CostResult{{TotalCost: 100.0, Currency: "USD"}},
			wantErr: false,
		},
		{
			name:    "ndjson format with actual costs",
			format:  OutputNDJSON,
			results: []CostResult{{TotalCost: 100.0, Currency: "USD"}},
			wantErr: false,
		},
		{
			name:    "unsupported format",
			format:  OutputFormat("invalid"),
			results: []CostResult{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RenderActualCostResults(io.Discard, tt.format, tt.results, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("RenderActualCostResults() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestRenderCrossProviderAggregation tests cross-provider rendering.
func TestRenderCrossProviderAggregation(t *testing.T) {
	aggregations := []CrossProviderAggregation{
		{
			Period:    "2025-01-01",
			Total:     100.0,
			Currency:  "USD",
			Providers: map[string]float64{"aws": 100.0},
		},
	}

	tests := []struct {
		name         string
		format       OutputFormat
		aggregations []CrossProviderAggregation
		groupBy      GroupBy
		wantErr      bool
	}{
		{
			name:         "table format daily",
			format:       OutputTable,
			aggregations: aggregations,
			groupBy:      GroupByDaily,
			wantErr:      false,
		},
		{
			name:         "table format monthly",
			format:       OutputTable,
			aggregations: aggregations,
			groupBy:      GroupByMonthly,
			wantErr:      false,
		},
		{
			name:         "json format",
			format:       OutputJSON,
			aggregations: aggregations,
			groupBy:      GroupByDaily,
			wantErr:      false,
		},
		{
			name:         "ndjson format",
			format:       OutputNDJSON,
			aggregations: aggregations,
			groupBy:      GroupByDaily,
			wantErr:      false,
		},
		{
			name:         "unsupported format",
			format:       OutputFormat("invalid"),
			aggregations: aggregations,
			groupBy:      GroupByDaily,
			wantErr:      true,
		},
		{
			name:         "empty aggregations",
			format:       OutputTable,
			aggregations: []CrossProviderAggregation{},
			groupBy:      GroupByDaily,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RenderCrossProviderAggregation(
				io.Discard,
				tt.format,
				tt.aggregations,
				tt.groupBy,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("RenderCrossProviderAggregation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetCurrencySymbol tests currency symbol mapping.
func TestGetCurrencySymbol(t *testing.T) {
	tests := []struct {
		currency string
		expected string
	}{
		{"USD", "$"},
		{"EUR", "€"},
		{"GBP", "£"},
		{"JPY", "¥"},
		{"CAD", "C$"},
		{"AUD", "A$"},
		{"UNKNOWN", "UNKNOWN"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			got := getCurrencySymbol(tt.currency)
			if got != tt.expected {
				t.Errorf("getCurrencySymbol(%q) = %q, want %q", tt.currency, got, tt.expected)
			}
		})
	}
}

// TestOutputFormatConstants tests that output format constants are defined.
func TestOutputFormatConstants(t *testing.T) {
	tests := []struct {
		name   string
		format OutputFormat
		value  string
	}{
		{"table format", OutputTable, "table"},
		{"json format", OutputJSON, "json"},
		{"ndjson format", OutputNDJSON, "ndjson"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.format) != tt.value {
				t.Errorf("OutputFormat = %q, want %q", tt.format, tt.value)
			}
		})
	}
}

// TestJSONMarshaling tests that CostResult and CrossProviderAggregation marshal correctly.
func TestJSONMarshaling(t *testing.T) {
	t.Run("CostResult marshals to JSON", func(t *testing.T) {
		result := CostResult{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-123",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      100.0,
			Hourly:       0.137,
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Failed to marshal CostResult: %v", err)
		}

		var unmarshaled CostResult
		if err = json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal CostResult: %v", err)
		}

		if unmarshaled.ResourceType != result.ResourceType {
			t.Errorf("ResourceType = %q, want %q", unmarshaled.ResourceType, result.ResourceType)
		}
		if unmarshaled.Monthly != result.Monthly {
			t.Errorf("Monthly = %f, want %f", unmarshaled.Monthly, result.Monthly)
		}
	})

	t.Run("CrossProviderAggregation marshals to JSON", func(t *testing.T) {
		agg := CrossProviderAggregation{
			Period:    "2025-01-01",
			Total:     150.0,
			Currency:  "USD",
			Providers: map[string]float64{"aws": 100.0, "azure": 50.0},
		}

		data, err := json.Marshal(agg)
		if err != nil {
			t.Fatalf("Failed to marshal CrossProviderAggregation: %v", err)
		}

		var unmarshaled CrossProviderAggregation
		if err = json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal CrossProviderAggregation: %v", err)
		}

		if unmarshaled.Period != agg.Period {
			t.Errorf("Period = %q, want %q", unmarshaled.Period, agg.Period)
		}
		if unmarshaled.Total != agg.Total {
			t.Errorf("Total = %f, want %f", unmarshaled.Total, agg.Total)
		}
		if len(unmarshaled.Providers) != len(agg.Providers) {
			t.Errorf(
				"Providers length = %d, want %d",
				len(unmarshaled.Providers),
				len(agg.Providers),
			)
		}
	})

	t.Run("AggregatedResults marshals to JSON", func(t *testing.T) {
		aggregated := &AggregatedResults{
			Summary: CostSummary{
				TotalMonthly: 100.0,
				TotalHourly:  0.137,
				Currency:     "USD",
				ByProvider:   map[string]float64{"aws": 100.0},
				ByService:    map[string]float64{"ec2": 100.0},
				ByAdapter:    map[string]float64{"kubecost": 100.0},
			},
			Resources: []CostResult{
				{ResourceType: "aws:ec2:Instance", Monthly: 100.0},
			},
		}

		data, err := json.Marshal(aggregated)
		if err != nil {
			t.Fatalf("Failed to marshal AggregatedResults: %v", err)
		}

		var unmarshaled AggregatedResults
		if err = json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal AggregatedResults: %v", err)
		}

		if unmarshaled.Summary.TotalMonthly != aggregated.Summary.TotalMonthly {
			t.Errorf(
				"TotalMonthly = %f, want %f",
				unmarshaled.Summary.TotalMonthly,
				aggregated.Summary.TotalMonthly,
			)
		}
	})
}

// TestRenderingWithNilResults tests edge cases with nil results.
func TestRenderingWithNilResults(t *testing.T) {
	t.Run("RenderResults with nil", func(t *testing.T) {
		err := RenderResults(io.Discard, OutputJSON, nil)
		if err != nil {
			t.Errorf("RenderResults with nil should not error: %v", err)
		}
	})

	t.Run("RenderActualCostResults with nil", func(t *testing.T) {
		err := RenderActualCostResults(io.Discard, OutputJSON, nil, false)
		if err != nil {
			t.Errorf("RenderActualCostResults with nil should not error: %v", err)
		}
	})

	t.Run("RenderCrossProviderAggregation with nil", func(t *testing.T) {
		err := RenderCrossProviderAggregation(io.Discard, OutputJSON, nil, GroupByDaily)
		if err != nil {
			t.Errorf("RenderCrossProviderAggregation with nil should not error: %v", err)
		}
	})
}

// TestRenderingWithEmptyResults tests edge cases with empty results.
func TestRenderingWithEmptyResults(t *testing.T) {
	t.Run("RenderResults with empty slice", func(t *testing.T) {
		err := RenderResults(io.Discard, OutputTable, []CostResult{})
		if err != nil {
			t.Errorf("RenderResults with empty should not error: %v", err)
		}
	})

	t.Run("RenderActualCostResults with empty slice", func(t *testing.T) {
		err := RenderActualCostResults(io.Discard, OutputTable, []CostResult{}, false)
		if err != nil {
			t.Errorf("RenderActualCostResults with empty should not error: %v", err)
		}
	})

	t.Run("RenderCrossProviderAggregation with empty slice", func(t *testing.T) {
		err := RenderCrossProviderAggregation(
			io.Discard,
			OutputTable,
			[]CrossProviderAggregation{},
			GroupByDaily,
		)
		if err != nil {
			t.Errorf("RenderCrossProviderAggregation with empty should not error: %v", err)
		}
	})
}

// TestMultiProviderRendering tests rendering with multiple providers.
func TestMultiProviderRendering(t *testing.T) {
	results := []CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-123",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      50.0,
		},
		{
			ResourceType: "azure:compute:VirtualMachine",
			ResourceID:   "vm-456",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      75.0,
		},
		{
			ResourceType: "gcp:compute:Instance",
			ResourceID:   "instance-789",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      100.0,
		},
	}

	t.Run("multi-provider table rendering", func(t *testing.T) {
		err := RenderResults(io.Discard, OutputTable, results)
		if err != nil {
			t.Errorf("Multi-provider table rendering failed: %v", err)
		}
	})

	t.Run("multi-provider json rendering", func(t *testing.T) {
		err := RenderResults(io.Discard, OutputJSON, results)
		if err != nil {
			t.Errorf("Multi-provider JSON rendering failed: %v", err)
		}
	})

	t.Run("multi-provider ndjson rendering", func(t *testing.T) {
		err := RenderResults(io.Discard, OutputNDJSON, results)
		if err != nil {
			t.Errorf("Multi-provider NDJSON rendering failed: %v", err)
		}
	})
}

// TestRenderBreakdowns_DeterministicOrder tests that renderBreakdowns produces deterministic output.
// This is critical for SC-003 (deterministic output) to ensure CI tests pass reliably.
func TestRenderBreakdowns_DeterministicOrder(t *testing.T) {
	// Create aggregated results with multiple entries in each breakdown map.
	// Map iteration order in Go is non-deterministic, so we need to verify
	// that the output is sorted alphabetically regardless of insertion order.
	aggregated := &AggregatedResults{
		Summary: CostSummary{
			TotalMonthly: 300.0,
			TotalHourly:  0.41,
			Currency:     "USD",
			// Insert in non-alphabetical order to test sorting
			ByProvider: map[string]float64{
				"gcp":   100.0,
				"aws":   150.0,
				"azure": 50.0,
			},
			ByService: map[string]float64{
				"rds":     50.0,
				"ec2":     100.0,
				"compute": 75.0,
				"storage": 75.0,
			},
			ByAdapter: map[string]float64{
				"kubecost":   200.0,
				"local-spec": 100.0,
			},
		},
		Resources: []CostResult{},
	}

	// Run the render function multiple times and verify output is identical
	var outputs []string
	for i := 0; i < 10; i++ {
		var buf strings.Builder
		renderBreakdowns(&buf, aggregated)
		outputs = append(outputs, buf.String())
	}

	// All outputs should be identical
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("renderBreakdowns produced non-deterministic output on iteration %d", i)
			t.Logf("Expected:\n%s", outputs[0])
			t.Logf("Got:\n%s", outputs[i])
		}
	}

	// Verify the output contains keys in alphabetical order
	output := outputs[0]

	// Check provider order: aws < azure < gcp
	awsIdx := strings.Index(output, "aws:")
	azureIdx := strings.Index(output, "azure:")
	gcpIdx := strings.Index(output, "gcp:")
	if awsIdx == -1 || azureIdx == -1 || gcpIdx == -1 {
		t.Error("Expected all providers to be present in output")
	} else if !(awsIdx < azureIdx && azureIdx < gcpIdx) {
		t.Error("Providers not in alphabetical order (aws, azure, gcp)")
	}

	// Check service order: compute < ec2 < rds < storage
	computeIdx := strings.Index(output, "compute:")
	ec2Idx := strings.Index(output, "ec2:")
	rdsIdx := strings.Index(output, "rds:")
	storageIdx := strings.Index(output, "storage:")
	if computeIdx == -1 || ec2Idx == -1 || rdsIdx == -1 || storageIdx == -1 {
		t.Error("Expected all services to be present in output")
	} else if !(computeIdx < ec2Idx && ec2Idx < rdsIdx && rdsIdx < storageIdx) {
		t.Error("Services not in alphabetical order (compute, ec2, rds, storage)")
	}

	// Check adapter order: kubecost < local-spec
	kubecostIdx := strings.Index(output, "kubecost:")
	localSpecIdx := strings.Index(output, "local-spec:")
	if kubecostIdx == -1 || localSpecIdx == -1 {
		t.Error("Expected all adapters to be present in output")
	} else if !(kubecostIdx < localSpecIdx) {
		t.Error("Adapters not in alphabetical order (kubecost, local-spec)")
	}
}

// TestLongResourceNameHandling tests truncation behavior.
func TestLongResourceNameHandling(t *testing.T) {
	longID := "i-" + string(make([]byte, 100))
	for i := range longID[2:] {
		longID = longID[:2+i] + "a" + longID[2+i+1:]
	}

	results := []CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   longID,
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      10.0,
		},
	}

	err := RenderResults(io.Discard, OutputTable, results)
	if err != nil {
		t.Errorf("Long resource name rendering failed: %v", err)
	}
}

// TestRenderActualCostResultsWithConfidence tests confidence column in table output.
func TestRenderActualCostResultsWithConfidence(t *testing.T) {
	results := []CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-12345",
			Adapter:      "kubecost",
			Currency:     "USD",
			TotalCost:    150.00,
			CostPeriod:   "30 days",
			Confidence:   ConfidenceHigh,
		},
		{
			ResourceType: "aws:rds:Instance",
			ResourceID:   "db-001",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      75.00,
			Confidence:   ConfidenceMedium,
		},
		{
			ResourceType: "aws:s3:Bucket",
			ResourceID:   "my-bucket",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      5.00,
			Confidence:   ConfidenceLow,
		},
	}

	var buf strings.Builder
	err := RenderActualCostResults(&buf, OutputTable, results, true)
	if err != nil {
		t.Fatalf("RenderActualCostResults with confidence failed: %v", err)
	}

	output := buf.String()

	// Check header contains CONFIDENCE column
	if !strings.Contains(output, "Confidence") {
		t.Error("Expected 'Confidence' header in table output")
	}

	// Check confidence values appear in output
	if !strings.Contains(output, "HIGH") {
		t.Error("Expected 'HIGH' confidence in output")
	}
	if !strings.Contains(output, "MEDIUM") {
		t.Error("Expected 'MEDIUM' confidence in output")
	}
	if !strings.Contains(output, "LOW") {
		t.Error("Expected 'LOW' confidence in output")
	}
}

// TestRenderActualCostResultsWithConfidenceJSON tests confidence field in JSON output.
func TestRenderActualCostResultsWithConfidenceJSON(t *testing.T) {
	results := []CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-12345",
			Adapter:      "kubecost",
			Currency:     "USD",
			TotalCost:    150.00,
			Confidence:   ConfidenceHigh,
		},
		{
			ResourceType: "aws:rds:Instance",
			ResourceID:   "db-001",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      75.00,
			Confidence:   ConfidenceLow,
		},
	}

	var buf strings.Builder
	err := RenderActualCostResults(&buf, OutputJSON, results, true)
	if err != nil {
		t.Fatalf("RenderActualCostResults with confidence (JSON) failed: %v", err)
	}

	output := buf.String()

	// Parse JSON to verify confidence field
	var parsed []CostResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(parsed))
	}

	// Verify confidence is preserved in JSON
	if parsed[0].Confidence != ConfidenceHigh {
		t.Errorf("Expected first result confidence to be HIGH, got %v", parsed[0].Confidence)
	}
	if parsed[1].Confidence != ConfidenceLow {
		t.Errorf("Expected second result confidence to be LOW, got %v", parsed[1].Confidence)
	}
}

// TestConfidenceInCostResultJSON tests that Confidence is properly serialized in CostResult JSON.
func TestConfidenceInCostResultJSON(t *testing.T) {
	result := CostResult{
		ResourceType: "aws:ec2:Instance",
		ResourceID:   "i-12345",
		Monthly:      50.00,
		Currency:     "USD",
		Confidence:   ConfidenceMedium,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal CostResult: %v", err)
	}

	jsonStr := string(data)

	// Verify confidence field is present
	if !strings.Contains(jsonStr, `"confidence":"medium"`) {
		t.Errorf("Expected confidence field in JSON, got: %s", jsonStr)
	}
}

// TestConfidenceOmittedWhenUnknown tests that empty confidence is omitted in JSON.
func TestConfidenceOmittedWhenUnknown(t *testing.T) {
	result := CostResult{
		ResourceType: "aws:ec2:Instance",
		ResourceID:   "i-12345",
		Monthly:      50.00,
		Currency:     "USD",
		Confidence:   ConfidenceUnknown, // empty string
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal CostResult: %v", err)
	}

	jsonStr := string(data)

	// Verify confidence field is omitted (omitempty behavior)
	if strings.Contains(jsonStr, `"confidence"`) {
		t.Errorf("Expected confidence to be omitted when unknown, got: %s", jsonStr)
	}
}

// TestRenderSustainabilitySummary_WithEquivalencies tests carbon equivalency display.
// This test verifies User Story 1: View Carbon Equivalencies in CLI Table Output.
func TestRenderSustainabilitySummary_WithEquivalencies(t *testing.T) {
	// Test case from spec: 150 kg CO2e should produce equivalency text
	aggregated := &AggregatedResults{
		Summary: CostSummary{
			TotalMonthly: 100.0,
			Currency:     "USD",
		},
		Resources: []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-123",
				Monthly:      100.0,
				Currency:     "USD",
				Sustainability: map[string]SustainabilityMetric{
					"carbon_footprint": {Value: 150.0, Unit: "kg"},
				},
			},
		},
	}

	var buf strings.Builder
	renderSustainabilitySummary(context.Background(), &buf, aggregated)
	output := buf.String()

	// Verify SUSTAINABILITY SUMMARY header is present
	assert.Contains(t, output, "SUSTAINABILITY SUMMARY")

	// Verify carbon_footprint metric is displayed
	assert.Contains(t, output, "carbon_footprint:")
	assert.Contains(t, output, "150.00 kg")

	// FR-001: Verify equivalency display text is present
	assert.Contains(t, output, "Equivalent to")

	// FR-005: Verify number formatting with separators
	assert.Contains(t, output, "18,248") // smartphones with comma separator

	// Verify miles driven is mentioned
	assert.Contains(t, output, "miles")

	// Verify smartphones charged is mentioned
	assert.Contains(t, output, "smartphones")
}

// TestRenderSustainabilitySummary_BelowThreshold tests that equivalencies are omitted for small values.
func TestRenderSustainabilitySummary_BelowThreshold(t *testing.T) {
	// Carbon value below 1kg threshold should not show equivalencies
	aggregated := &AggregatedResults{
		Summary: CostSummary{
			TotalMonthly: 100.0,
			Currency:     "USD",
		},
		Resources: []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-123",
				Monthly:      100.0,
				Currency:     "USD",
				Sustainability: map[string]SustainabilityMetric{
					"carbon_footprint": {Value: 0.5, Unit: "kg"}, // Below threshold
				},
			},
		},
	}

	var buf strings.Builder
	renderSustainabilitySummary(context.Background(), &buf, aggregated)
	output := buf.String()

	// Verify SUSTAINABILITY SUMMARY is present with the metric
	assert.Contains(t, output, "SUSTAINABILITY SUMMARY")
	assert.Contains(t, output, "carbon_footprint:")

	// Equivalencies should NOT be displayed for values below threshold
	assert.NotContains(t, output, "Equivalent to")
	assert.NotContains(t, output, "miles")
}

// TestRenderSustainabilitySummary_NoCarbon tests behavior when no carbon metric exists.
func TestRenderSustainabilitySummary_NoCarbon(t *testing.T) {
	// Only energy consumption, no carbon_footprint
	aggregated := &AggregatedResults{
		Summary: CostSummary{
			TotalMonthly: 100.0,
			Currency:     "USD",
		},
		Resources: []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-123",
				Monthly:      100.0,
				Currency:     "USD",
				Sustainability: map[string]SustainabilityMetric{
					"energy_consumption": {Value: 2000.0, Unit: "kWh"},
				},
			},
		},
	}

	var buf strings.Builder
	renderSustainabilitySummary(context.Background(), &buf, aggregated)
	output := buf.String()

	// Verify sustainability summary is present with energy metric
	assert.Contains(t, output, "SUSTAINABILITY SUMMARY")
	assert.Contains(t, output, "energy_consumption:")

	// No equivalencies should be shown without carbon metric
	assert.NotContains(t, output, "Equivalent to")
}

// TestRenderSustainabilitySummary_AggregatesMultipleResources tests aggregation.
func TestRenderSustainabilitySummary_AggregatesMultipleResources(t *testing.T) {
	// Multiple resources with carbon emissions should be aggregated
	aggregated := &AggregatedResults{
		Summary: CostSummary{
			TotalMonthly: 200.0,
			Currency:     "USD",
		},
		Resources: []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-123",
				Monthly:      100.0,
				Currency:     "USD",
				Sustainability: map[string]SustainabilityMetric{
					"carbon_footprint": {Value: 75.0, Unit: "kg"},
				},
			},
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-456",
				Monthly:      100.0,
				Currency:     "USD",
				Sustainability: map[string]SustainabilityMetric{
					"carbon_footprint": {Value: 75.0, Unit: "kg"},
				},
			},
		},
	}

	var buf strings.Builder
	renderSustainabilitySummary(context.Background(), &buf, aggregated)
	output := buf.String()

	// Total should be 150kg (75 + 75)
	assert.Contains(t, output, "150.00 kg")

	// Equivalencies should reflect the aggregated total
	assert.Contains(t, output, "Equivalent to")
}

// TestRenderSustainabilitySummary_LargeValues tests large number formatting.
func TestRenderSustainabilitySummary_LargeValues(t *testing.T) {
	// Large carbon value should use million/billion scaling
	aggregated := &AggregatedResults{
		Summary: CostSummary{
			TotalMonthly: 100000.0,
			Currency:     "USD",
		},
		Resources: []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-123",
				Monthly:      100000.0,
				Currency:     "USD",
				Sustainability: map[string]SustainabilityMetric{
					"carbon_footprint": {Value: 10000000.0, Unit: "kg"}, // 10 million kg
				},
			},
		},
	}

	var buf strings.Builder
	renderSustainabilitySummary(context.Background(), &buf, aggregated)
	output := buf.String()

	// Large values should use "million" scaling
	assert.Contains(t, output, "million")
}

// TestRenderSustainabilitySummary_GracefulDegradation tests error handling.
func TestRenderSustainabilitySummary_GracefulDegradation(t *testing.T) {
	// Empty sustainability data should not crash
	aggregated := &AggregatedResults{
		Summary: CostSummary{
			TotalMonthly: 100.0,
			Currency:     "USD",
		},
		Resources: []CostResult{
			{
				ResourceType:   "aws:ec2:Instance",
				ResourceID:     "i-123",
				Monthly:        100.0,
				Currency:       "USD",
				Sustainability: nil, // No sustainability data
			},
		},
	}

	var buf strings.Builder
	// Should not panic
	require.NotPanics(t, func() {
		renderSustainabilitySummary(context.Background(), &buf, aggregated)
	})
}
