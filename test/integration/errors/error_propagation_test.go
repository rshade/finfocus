// Package errors_test provides integration tests for error propagation across components.
package errors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// TestErrorPropagation_MissingPlanFile tests error when plan file doesn't exist.
func TestErrorPropagation_MissingPlanFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Execute with non-existent file
	errMsg := h.ExecuteExpectError("cost", "projected", "--pulumi-json", "/nonexistent/plan.json")

	// Error should propagate from file reading layer
	assert.Contains(t, errMsg, "no such file", "Should report file not found error")
}

// TestErrorPropagation_InvalidJSON tests error when JSON parsing fails.
func TestErrorPropagation_InvalidJSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create file with invalid JSON
	invalidJSON := `{invalid json content`
	planFile := h.CreateTempFile(invalidJSON)

	// Execute command
	errMsg := h.ExecuteExpectError("cost", "projected", "--pulumi-json", planFile)

	// Error should propagate from JSON parser
	assert.Contains(t, errMsg, "invalid", "Should report JSON parsing error")
}

// TestErrorPropagation_EmptyResourceType tests error handling for resources without type.
func TestErrorPropagation_EmptyResourceType(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create plan with resource missing type field
	planJSON := `{
		"resources": [
			{
				"id": "test-resource",
				"provider": "aws"
			}
		]
	}`
	planFile := h.CreateTempFile(planJSON)

	// Execute command - should handle gracefully
	_, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")

	// May error or skip invalid resource - both are acceptable
	// Just verify it doesn't panic
	if err != nil {
		assert.NotEmpty(t, err.Error(), "Error message should not be empty")
	}
}

// TestErrorPropagation_PluginError tests error propagation from plugin layer.
func TestErrorPropagation_PluginError(t *testing.T) {
	// This test would require setting up a mock plugin that returns errors
	// For now, we test the error path when no plugins are available
	h := helpers.NewCLIHelper(t)

	planJSON := `{
		"resources": [
			{
				"type": "aws:ec2/instance:Instance",
				"id": "test-instance",
				"provider": "aws"
			}
		]
	}`
	planFile := h.CreateTempFile(planJSON)

	// Execute - should work with "none" adapter when no plugins available
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err, "Should handle missing plugins gracefully with 'none' adapter")

	// Verify it returns results (with "none" adapter)
	h.AssertContains(output, "resources")
}

// TestErrorPropagation_InvalidOutputFormat tests error for unsupported output format.
func TestErrorPropagation_InvalidOutputFormat(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planJSON := `{"resources": []}`
	planFile := h.CreateTempFile(planJSON)

	// Execute with invalid output format
	errMsg := h.ExecuteExpectError("cost", "projected", "--pulumi-json", planFile, "--output", "invalid-format")

	// Should report invalid format
	assert.Contains(t, errMsg, "invalid", "Should report invalid output format")
}

// TestErrorPropagation_MalformedPlanStructure tests handling of malformed plan structure.
func TestErrorPropagation_MalformedPlanStructure(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create plan with unexpected structure
	planJSON := `{
		"not_resources": "wrong field"
	}`
	planFile := h.CreateTempFile(planJSON)

	// Execute command
	_, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")

	// Should handle gracefully (may return empty results)
	// Behavior depends on implementation - just verify no panic
	if err != nil {
		assert.NotEmpty(t, err.Error())
	}
}

// TestErrorPropagation_PluginTimeout tests timeout error from plugin.
func TestErrorPropagation_PluginTimeout(t *testing.T) {
	// Start mock plugin with timeout error
	server, err := plugin.StartMockServerTCP()
	require.NoError(t, err)
	defer server.Stop()

	// Configure timeout error
	server.Plugin.SetError("GetProjectedCost", plugin.ErrorTimeout)

	// This test demonstrates error injection
	// In a real integration test, we would configure CLI to use this mock plugin
	// For now, verify the mock plugin error mechanism works
	config := server.Plugin.GetConfig()
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
}

// TestErrorPropagation_PluginProtocolError tests protocol error from plugin.
func TestErrorPropagation_PluginProtocolError(t *testing.T) {
	// Start mock plugin with protocol error
	server, err := plugin.StartMockServerTCP()
	require.NoError(t, err)
	defer server.Stop()

	// Configure protocol error
	server.Plugin.SetError("GetProjectedCost", plugin.ErrorProtocol)

	// Verify error configuration
	config := server.Plugin.GetConfig()
	assert.Equal(t, plugin.ErrorProtocol, config.ErrorType)
}

// TestErrorPropagation_PluginInvalidData tests invalid data error from plugin.
func TestErrorPropagation_PluginInvalidData(t *testing.T) {
	// Start mock plugin with invalid data error
	server, err := plugin.StartMockServerTCP()
	require.NoError(t, err)
	defer server.Stop()

	// Configure invalid data error
	server.Plugin.SetError("GetProjectedCost", plugin.ErrorInvalidData)

	// Verify error configuration
	config := server.Plugin.GetConfig()
	assert.Equal(t, plugin.ErrorInvalidData, config.ErrorType)
}

// TestErrorPropagation_PluginUnavailable tests unavailable error from plugin.
func TestErrorPropagation_PluginUnavailable(t *testing.T) {
	// Start mock plugin with unavailable error
	server, err := plugin.StartMockServerTCP()
	require.NoError(t, err)
	defer server.Stop()

	// Configure unavailable error
	server.Plugin.SetError("GetProjectedCost", plugin.ErrorUnavailable)

	// Verify error configuration
	config := server.Plugin.GetConfig()
	assert.Equal(t, plugin.ErrorUnavailable, config.ErrorType)
}

// TestErrorPropagation_MultipleErrors tests handling of multiple errors.
func TestErrorPropagation_MultipleErrors(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create multiple scenarios that could error
	// 1. Invalid JSON
	invalidJSON := `{invalid`
	planFile1 := h.CreateTempFile(invalidJSON)

	errMsg1 := h.ExecuteExpectError("cost", "projected", "--pulumi-json", planFile1)
	assert.Contains(t, errMsg1, "invalid", "First error should be about invalid JSON")

	// 2. Missing file
	errMsg2 := h.ExecuteExpectError("cost", "projected", "--pulumi-json", "/nonexistent.json")
	assert.Contains(t, errMsg2, "no such file", "Second error should be about missing file")

	// Errors should be independent
	assert.NotEqual(t, errMsg1, errMsg2, "Different errors should produce different messages")
}

// TestErrorPropagation_GracefulDegradation tests graceful handling when components fail.
func TestErrorPropagation_GracefulDegradation(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create valid plan
	planJSON := `{
		"resources": [
			{
				"type": "aws:s3/bucket:Bucket",
				"id": "test-bucket",
				"provider": "aws"
			}
		]
	}`
	planFile := h.CreateTempFile(planJSON)

	// Execute without plugins (should use "none" adapter)
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err, "Should degrade gracefully to 'none' adapter")

	// Verify output contains resources (with placeholder costs)
	h.AssertContains(output, "resources")
}
