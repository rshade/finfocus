package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// TestSetLatency verifies latency configuration.
func TestSetLatency(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set 100ms latency
	mock.SetLatency(100)

	config := mock.GetConfig()
	assert.Equal(t, 100, config.LatencyMS)

	// Change to 500ms
	mock.SetLatency(500)
	config = mock.GetConfig()
	assert.Equal(t, 500, config.LatencyMS)

	// Clear latency
	mock.SetLatency(0)
	config = mock.GetConfig()
	assert.Equal(t, 0, config.LatencyMS)
}

// TestLatencyDefaultValue verifies default latency is zero.
func TestLatencyDefaultValue(t *testing.T) {
	mock := plugin.NewMockPlugin()

	config := mock.GetConfig()
	assert.Equal(t, 0, config.LatencyMS, "Default latency should be 0")
}

// TestLatencyWithScenarios verifies latency persists across scenario changes.
func TestLatencyWithScenarios(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set latency
	mock.SetLatency(250)
	config := mock.GetConfig()
	assert.Equal(t, 250, config.LatencyMS)

	// Change scenario (which calls Reset internally)
	mock.ConfigureScenario(plugin.ScenarioSuccess)

	// Latency should be cleared because ConfigureScenario calls Reset
	config = mock.GetConfig()
	assert.Equal(t, 0, config.LatencyMS)
}

// TestLatencyResetClearsLatency verifies Reset() clears latency configuration.
func TestLatencyResetClearsLatency(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set latency
	mock.SetLatency(1000)
	config := mock.GetConfig()
	assert.Equal(t, 1000, config.LatencyMS)

	// Reset
	mock.Reset()

	config = mock.GetConfig()
	assert.Equal(t, 0, config.LatencyMS)
}

// TestLatencyWithResponseConfiguration verifies latency and responses can be configured together.
func TestLatencyWithResponseConfiguration(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Configure a response
	mock.SetProjectedCostResponse("test:resource:Type", plugin.QuickResponse("USD", 50.00, 0.068))

	// Set latency
	mock.SetLatency(150)

	config := mock.GetConfig()

	// Both should be set
	assert.Contains(t, config.ProjectedCostResponses, "test:resource:Type")
	assert.Equal(t, 150, config.LatencyMS)
}

// TestDifferentLatencyValues verifies various latency values can be set.
func TestDifferentLatencyValues(t *testing.T) {
	testCases := []struct {
		name      string
		latencyMS int
	}{
		{"No latency", 0},
		{"Fast latency", 10},
		{"Medium latency", 100},
		{"Slow latency", 500},
		{"Very slow latency", 2000},
		{"Timeout simulation", 5000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := plugin.NewMockPlugin()
			mock.SetLatency(tc.latencyMS)

			config := mock.GetConfig()
			assert.Equal(t, tc.latencyMS, config.LatencyMS)
		})
	}
}

// TestLatencyPersistenceAcrossConfigChanges verifies latency persists when adding responses.
func TestLatencyPersistenceAcrossConfigChanges(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set latency
	mock.SetLatency(300)

	// Add responses
	mock.SetProjectedCostResponse("res1", plugin.QuickResponse("USD", 10.0, 0.01))
	mock.SetActualCostResponse("res2", plugin.QuickActualResponse("USD", 20.0))

	// Latency should still be set
	config := mock.GetConfig()
	assert.Equal(t, 300, config.LatencyMS)

	// And responses should be set
	assert.Len(t, config.ProjectedCostResponses, 1)
	assert.Len(t, config.ActualCostResponses, 1)
}

// TestFullConfigureWithLatency verifies Configure() can set latency.
func TestFullConfigureWithLatency(t *testing.T) {
	mock := plugin.NewMockPlugin()

	fullConfig := plugin.MockConfig{
		ProjectedCostResponses: make(map[string]*proto.CostResult),
		ActualCostResponses:    make(map[string]*proto.ActualCostResult),
		ErrorType:              plugin.ErrorNone,
		ErrorMethod:            "",
		LatencyMS:              750,
	}

	mock.Configure(fullConfig)

	config := mock.GetConfig()
	assert.Equal(t, 750, config.LatencyMS)
}

// TestCombinedErrorAndLatency verifies both error and latency can be set together.
func TestCombinedErrorAndLatency(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set both
	mock.SetError("GetProjectedCost", plugin.ErrorProtocol)
	mock.SetLatency(200)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorProtocol, config.ErrorType)
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)
	assert.Equal(t, 200, config.LatencyMS)
}

// TestLatencyChangeMultipleTimes verifies latency can be changed multiple times.
func TestLatencyChangeMultipleTimes(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set different latencies
	latencies := []int{100, 200, 50, 1000, 0, 500}

	for _, latency := range latencies {
		mock.SetLatency(latency)
		config := mock.GetConfig()
		assert.Equal(t, latency, config.LatencyMS)
	}
}

// TestLatencyWithScenarioAndError verifies complete configuration with latency.
func TestLatencyWithScenarioAndError(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Configure everything
	mock.ConfigureScenario(plugin.ScenarioSuccess)
	mock.SetError("GetActualCost", plugin.ErrorTimeout)
	mock.SetLatency(400)

	config := mock.GetConfig()

	// Scenario should have set responses
	assert.NotEmpty(t, config.ProjectedCostResponses)

	// Error should be set
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
	assert.Equal(t, "GetActualCost", config.ErrorMethod)

	// Latency should be set
	assert.Equal(t, 400, config.LatencyMS)
}

// TestNegativeLatency verifies negative latency values can be set (treated as 0 or error by implementation).
func TestNegativeLatency(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set negative latency (implementation may clamp to 0 or allow it)
	mock.SetLatency(-100)

	config := mock.GetConfig()
	// We allow setting negative values, but implementation should handle them
	assert.Equal(t, -100, config.LatencyMS)
}

// TestZeroLatencyAfterPositive verifies clearing latency by setting to 0.
func TestZeroLatencyAfterPositive(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set positive latency
	mock.SetLatency(500)
	config := mock.GetConfig()
	assert.Equal(t, 500, config.LatencyMS)

	// Clear by setting to 0
	mock.SetLatency(0)
	config = mock.GetConfig()
	assert.Equal(t, 0, config.LatencyMS)
}

// TestLatencyIsolationFromErrors verifies latency and errors are independent.
func TestLatencyIsolationFromErrors(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set latency
	mock.SetLatency(100)

	// Set error
	mock.SetError("GetProjectedCost", plugin.ErrorProtocol)

	// Clear error
	mock.SetError("GetProjectedCost", plugin.ErrorNone)

	// Latency should still be set
	config := mock.GetConfig()
	assert.Equal(t, 100, config.LatencyMS)
}

// TestLatencyAfterMultipleResets verifies latency clears after each reset.
func TestLatencyAfterMultipleResets(t *testing.T) {
	mock := plugin.NewMockPlugin()

	for i := 0; i < 5; i++ {
		// Set latency
		mock.SetLatency(100 * (i + 1))
		config := mock.GetConfig()
		assert.Equal(t, 100*(i+1), config.LatencyMS)

		// Reset
		mock.Reset()
		config = mock.GetConfig()
		assert.Equal(t, 0, config.LatencyMS)
	}
}
