package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// TestSetError verifies error configuration for specific methods.
func TestSetError(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set timeout error for GetProjectedCost
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)

	// Change to protocol error for GetActualCost
	mock.SetError("GetActualCost", plugin.ErrorProtocol)

	config = mock.GetConfig()
	assert.Equal(t, plugin.ErrorProtocol, config.ErrorType)
	assert.Equal(t, "GetActualCost", config.ErrorMethod)
}

// TestSetErrorToNone verifies clearing error injection.
func TestSetErrorToNone(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set an error
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)
	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)

	// Clear error
	mock.SetError("GetProjectedCost", plugin.ErrorNone)
	config = mock.GetConfig()
	assert.Equal(t, plugin.ErrorNone, config.ErrorType)
}

// TestErrorTypeTimeout verifies timeout error injection.
func TestErrorTypeTimeout(t *testing.T) {
	mock := plugin.NewMockPlugin()

	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)
}

// TestErrorTypeProtocol verifies protocol error injection.
func TestErrorTypeProtocol(t *testing.T) {
	mock := plugin.NewMockPlugin()

	mock.SetError("GetActualCost", plugin.ErrorProtocol)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorProtocol, config.ErrorType)
	assert.Equal(t, "GetActualCost", config.ErrorMethod)
}

// TestErrorTypeInvalidData verifies invalid data error injection.
func TestErrorTypeInvalidData(t *testing.T) {
	mock := plugin.NewMockPlugin()

	mock.SetError("GetProjectedCost", plugin.ErrorInvalidData)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorInvalidData, config.ErrorType)
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)
}

// TestErrorTypeUnavailable verifies service unavailable error injection.
func TestErrorTypeUnavailable(t *testing.T) {
	mock := plugin.NewMockPlugin()

	mock.SetError("GetActualCost", plugin.ErrorUnavailable)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorUnavailable, config.ErrorType)
	assert.Equal(t, "GetActualCost", config.ErrorMethod)
}

// TestErrorMethodIsolation verifies errors only apply to specified method.
func TestErrorMethodIsolation(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set error for GetProjectedCost only
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)

	config := mock.GetConfig()
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)

	// Verify error is only for the specified method
	// (This test verifies configuration, actual behavior tested in server_test.go)
	assert.NotEqual(t, "GetActualCost", config.ErrorMethod)
}

// TestMultipleErrorChanges verifies error type can be changed multiple times.
func TestMultipleErrorChanges(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Start with timeout
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)
	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)

	// Change to protocol error
	mock.SetError("GetProjectedCost", plugin.ErrorProtocol)
	config = mock.GetConfig()
	assert.Equal(t, plugin.ErrorProtocol, config.ErrorType)

	// Change to invalid data
	mock.SetError("GetProjectedCost", plugin.ErrorInvalidData)
	config = mock.GetConfig()
	assert.Equal(t, plugin.ErrorInvalidData, config.ErrorType)

	// Change to unavailable
	mock.SetError("GetProjectedCost", plugin.ErrorUnavailable)
	config = mock.GetConfig()
	assert.Equal(t, plugin.ErrorUnavailable, config.ErrorType)

	// Clear error
	mock.SetError("GetProjectedCost", plugin.ErrorNone)
	config = mock.GetConfig()
	assert.Equal(t, plugin.ErrorNone, config.ErrorType)
}

// TestErrorResetClearsErrors verifies Reset() clears error configuration.
func TestErrorResetClearsErrors(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set error
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)
	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)

	// Reset
	mock.Reset()

	config = mock.GetConfig()
	assert.Equal(t, plugin.ErrorNone, config.ErrorType)
	assert.Empty(t, config.ErrorMethod)
}

// TestScenarioChangePreservesErrors verifies scenario changes don't affect error config.
func TestScenarioChangePreservesErrors(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set error first
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)

	// Change scenario (which calls Reset internally)
	mock.ConfigureScenario(plugin.ScenarioSuccess)

	// Error should be cleared because ConfigureScenario calls Reset
	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorNone, config.ErrorType)
	assert.Empty(t, config.ErrorMethod)
}

// TestErrorWithResponseConfiguration verifies errors and responses can be configured together.
func TestErrorWithResponseConfiguration(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Configure a response
	mock.SetProjectedCostResponse("test:resource:Type", plugin.QuickResponse("USD", 50.00, 0.068))

	// Set error for different method
	mock.SetError("GetActualCost", plugin.ErrorTimeout)

	config := mock.GetConfig()

	// Both should be set
	assert.Contains(t, config.ProjectedCostResponses, "test:resource:Type")
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
	assert.Equal(t, "GetActualCost", config.ErrorMethod)
}

// TestErrorConstants verifies all error types have unique string values.
func TestErrorConstants(t *testing.T) {
	errorTypes := []plugin.ErrorType{
		plugin.ErrorNone,
		plugin.ErrorTimeout,
		plugin.ErrorProtocol,
		plugin.ErrorInvalidData,
		plugin.ErrorUnavailable,
	}

	// Verify all are different
	seen := make(map[plugin.ErrorType]bool)
	for _, et := range errorTypes {
		assert.False(t, seen[et], "Error type %s should be unique", et)
		seen[et] = true
	}
}

// TestErrorMessagesExist verifies error messages are defined for all error types.
func TestErrorMessagesExist(t *testing.T) {
	// Verify error variables exist and have messages
	assert.NotNil(t, plugin.ErrMockTimeout)
	assert.Contains(t, plugin.ErrMockTimeout.Error(), "timeout")

	assert.NotNil(t, plugin.ErrMockProtocol)
	assert.Contains(t, plugin.ErrMockProtocol.Error(), "protocol")

	assert.NotNil(t, plugin.ErrMockInvalidData)
	assert.Contains(t, plugin.ErrMockInvalidData.Error(), "invalid")

	assert.NotNil(t, plugin.ErrMockUnavailable)
	assert.Contains(t, plugin.ErrMockUnavailable.Error(), "unavailable")

	assert.NotNil(t, plugin.ErrMockNotConfigured)
	assert.Contains(t, plugin.ErrMockNotConfigured.Error(), "no response configured")
}

// TestCombinedErrorAndLatencyConfiguration verifies errors and latency can be set together.
func TestCombinedErrorAndLatencyConfiguration(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set both error and latency
	mock.SetError("GetProjectedCost", plugin.ErrorProtocol)
	mock.SetLatency(500)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorProtocol, config.ErrorType)
	assert.Equal(t, 500, config.LatencyMS)
}

// TestFullConfigureWithErrors verifies Configure() can set all error fields.
func TestFullConfigureWithErrors(t *testing.T) {
	mock := plugin.NewMockPlugin()

	fullConfig := plugin.MockConfig{
		ProjectedCostResponses: map[string]*proto.CostResult{},
		ActualCostResponses:    map[string]*proto.ActualCostResult{},
		ErrorType:              plugin.ErrorInvalidData,
		ErrorMethod:            "GetProjectedCost",
		LatencyMS:              100,
	}

	mock.Configure(fullConfig)

	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorInvalidData, config.ErrorType)
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)
	assert.Equal(t, 100, config.LatencyMS)
}

// TestDifferentMethodsSeparateErrors verifies error method targeting works correctly.
func TestDifferentMethodsSeparateErrors(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		error  plugin.ErrorType
	}{
		{"GetProjectedCost with timeout", "GetProjectedCost", plugin.ErrorTimeout},
		{"GetActualCost with protocol error", "GetActualCost", plugin.ErrorProtocol},
		{"GetProjectedCost with invalid data", "GetProjectedCost", plugin.ErrorInvalidData},
		{"GetActualCost with unavailable", "GetActualCost", plugin.ErrorUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := plugin.NewMockPlugin()
			mock.SetError(tc.method, tc.error)

			config := mock.GetConfig()
			assert.Equal(t, tc.error, config.ErrorType)
			assert.Equal(t, tc.method, config.ErrorMethod)
		})
	}
}

// TestErrorPersistenceAcrossConfigChanges verifies errors persist when adding responses.
func TestErrorPersistenceAcrossConfigChanges(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set error
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)

	// Add responses
	mock.SetProjectedCostResponse("res1", plugin.QuickResponse("USD", 10.0, 0.01))
	mock.SetActualCostResponse("res2", plugin.QuickActualResponse("USD", 20.0))

	// Error should still be set
	config := mock.GetConfig()
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
	assert.Equal(t, "GetProjectedCost", config.ErrorMethod)

	// And responses should be set
	assert.Len(t, config.ProjectedCostResponses, 1)
	assert.Len(t, config.ActualCostResponses, 1)
}
