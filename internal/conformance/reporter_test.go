package conformance

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestReport() *SuiteReport {
	startTime := time.Date(2025, 12, 2, 10, 30, 0, 0, time.UTC)
	endTime := startTime.Add(4500 * time.Millisecond)

	return &SuiteReport{
		SuiteName: "conformance",
		Plugin: PluginUnderTest{
			Path:            "./plugins/aws-cost",
			Name:            "aws-cost",
			Version:         "1.2.0",
			ProtocolVersion: "1.0",
			CommMode:        CommModeTCP,
		},
		Results: []TestResult{
			{
				TestName:  "Name_ReturnsPluginIdentifier",
				Category:  CategoryProtocol,
				Status:    StatusPass,
				Duration:  50 * time.Millisecond,
				Timestamp: startTime.Add(50 * time.Millisecond),
			},
			{
				TestName:  "Name_ReturnsProtocolVersion",
				Category:  CategoryProtocol,
				Status:    StatusPass,
				Duration:  45 * time.Millisecond,
				Timestamp: startTime.Add(95 * time.Millisecond),
			},
			{
				TestName:  "GetProjectedCost_InvalidResource",
				Category:  CategoryError,
				Status:    StatusFail,
				Duration:  110 * time.Millisecond,
				Error:     "expected NotFound, got InvalidArgument",
				Timestamp: startTime.Add(205 * time.Millisecond),
			},
			{
				TestName:  "GetActualCost_RequiresCredentials",
				Category:  CategoryProtocol,
				Status:    StatusSkip,
				Error:     "credentials not configured",
				Timestamp: startTime.Add(205 * time.Millisecond),
			},
		},
		Summary: Summary{
			Total:   4,
			Passed:  2,
			Failed:  1,
			Skipped: 1,
			Errors:  0,
		},
		StartTime: startTime,
		EndTime:   endTime,
		TotalTime: 4500 * time.Millisecond,
		Timestamp: endTime,
	}
}

func TestReport_WriteTable(t *testing.T) {
	t.Parallel()

	report := createTestReport()
	var buf bytes.Buffer

	err := report.WriteTable(&buf)

	require.NoError(t, err)
	output := buf.String()

	// Check header
	assert.Contains(t, output, "CONFORMANCE TEST RESULTS")
	assert.Contains(t, output, "Plugin: aws-cost v1.2.0")
	assert.Contains(t, output, "protocol v1.0")
	assert.Contains(t, output, "Mode:   TCP")

	// Check test results
	assert.Contains(t, output, "Name_ReturnsPluginIdentifier")
	assert.Contains(t, output, "Name_ReturnsProtocolVersion")
	assert.Contains(t, output, "GetProjectedCost_InvalidResource")
	assert.Contains(t, output, "GetActualCost_RequiresCredentials")

	// Check status indicators
	assert.Contains(t, output, "✓") // Pass
	assert.Contains(t, output, "✗") // Fail
	assert.Contains(t, output, "⊘") // Skip

	// Check error message is shown for failed test
	assert.Contains(t, output, "expected NotFound, got InvalidArgument")

	// Check summary
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "Total: 4")
	assert.Contains(t, output, "Passed: 2")
	assert.Contains(t, output, "Failed: 1")
	assert.Contains(t, output, "Skipped: 1")
}

func TestReport_WriteJSON(t *testing.T) {
	t.Parallel()

	report := createTestReport()
	var buf bytes.Buffer

	err := report.WriteJSON(&buf)

	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	// Check structure
	assert.Equal(t, "conformance", parsed["suite"])
	assert.NotNil(t, parsed["plugin"])
	assert.NotNil(t, parsed["results"])
	assert.NotNil(t, parsed["summary"])

	// Check plugin info
	plugin, ok := parsed["plugin"].(map[string]interface{})
	require.True(t, ok, "plugin should be a map")
	assert.Equal(t, "aws-cost", plugin["name"])
	assert.Equal(t, "1.2.0", plugin["version"])
	assert.Equal(t, "1.0", plugin["protocol_version"])
	assert.Equal(t, "tcp", plugin["comm_mode"])

	// Check results
	results, ok := parsed["results"].([]interface{})
	require.True(t, ok, "results should be an array")
	assert.Len(t, results, 4)

	// Check first result
	firstResult, ok := results[0].(map[string]interface{})
	require.True(t, ok, "first result should be a map")
	assert.Equal(t, "Name_ReturnsPluginIdentifier", firstResult["name"])
	assert.Equal(t, "protocol", firstResult["category"])
	assert.Equal(t, "pass", firstResult["status"])

	// Check failed result has error
	thirdResult, ok := results[2].(map[string]interface{})
	require.True(t, ok, "third result should be a map")
	assert.Equal(t, "fail", thirdResult["status"])
	assert.Equal(t, "expected NotFound, got InvalidArgument", thirdResult["error"])

	// Check summary
	summary, ok := parsed["summary"].(map[string]interface{})
	require.True(t, ok, "summary should be a map")
	assert.InDelta(t, float64(4), summary["total"], 1e-9)
	assert.InDelta(t, float64(2), summary["passed"], 1e-9)
	assert.InDelta(t, float64(1), summary["failed"], 1e-9)
	assert.InDelta(t, float64(1), summary["skipped"], 1e-9)
}

func TestReport_WriteJUnit(t *testing.T) {
	t.Parallel()

	report := createTestReport()
	var buf bytes.Buffer

	err := report.WriteJUnit(&buf)

	require.NoError(t, err)
	output := buf.String()

	// Check XML declaration
	assert.True(t, strings.HasPrefix(output, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"))

	// Check testsuites element
	assert.Contains(t, output, "<testsuites")
	assert.Contains(t, output, "name=\"finfocus-conformance\"")
	assert.Contains(t, output, "tests=\"4\"")
	assert.Contains(t, output, "failures=\"1\"")
	assert.Contains(t, output, "skipped=\"1\"")

	// Check testsuite element
	assert.Contains(t, output, "<testsuite")
	assert.Contains(t, output, "name=\"conformance\"")

	// Check properties
	assert.Contains(t, output, "<properties>")
	assert.Contains(t, output, "plugin.name")
	assert.Contains(t, output, "aws-cost")
	assert.Contains(t, output, "plugin.version")
	assert.Contains(t, output, "1.2.0")
	assert.Contains(t, output, "protocol.version")
	assert.Contains(t, output, "1.0")

	// Check test cases
	assert.Contains(t, output, "<testcase")
	assert.Contains(t, output, "name=\"Name_ReturnsPluginIdentifier\"")
	assert.Contains(t, output, "classname=\"protocol\"")

	// Check failure element for failed test
	assert.Contains(t, output, "<failure")
	assert.Contains(t, output, "expected NotFound, got InvalidArgument")

	// Check skipped element
	assert.Contains(t, output, "<skipped")
	assert.Contains(t, output, "credentials not configured")

	// Check closing tags
	assert.Contains(t, output, "</testsuite>")
	assert.Contains(t, output, "</testsuites>")
}

func TestReport_WriteTable_EmptyResults(t *testing.T) {
	t.Parallel()

	report := &SuiteReport{
		SuiteName: "conformance",
		Plugin: PluginUnderTest{
			Name:            "test-plugin",
			Version:         "1.0.0",
			ProtocolVersion: "1.0",
			CommMode:        CommModeTCP,
		},
		Results: []TestResult{},
		Summary: Summary{
			Total:   0,
			Passed:  0,
			Failed:  0,
			Skipped: 0,
			Errors:  0,
		},
	}

	var buf bytes.Buffer
	err := report.WriteTable(&buf)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "CONFORMANCE TEST RESULTS")
	assert.Contains(t, output, "Total: 0")
}

func TestReport_WriteJSON_EmptyResults(t *testing.T) {
	t.Parallel()

	report := &SuiteReport{
		SuiteName: "conformance",
		Plugin: PluginUnderTest{
			Name:            "test-plugin",
			Version:         "1.0.0",
			ProtocolVersion: "1.0",
			CommMode:        CommModeTCP,
		},
		Results: []TestResult{},
		Summary: Summary{},
	}

	var buf bytes.Buffer
	err := report.WriteJSON(&buf)

	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	results, ok := parsed["results"].([]interface{})
	require.True(t, ok, "results should be an array")
	assert.Empty(t, results)
}

func TestReport_WriteJUnit_EmptyResults(t *testing.T) {
	t.Parallel()

	report := &SuiteReport{
		SuiteName: "conformance",
		Plugin: PluginUnderTest{
			Name:            "test-plugin",
			Version:         "1.0.0",
			ProtocolVersion: "1.0",
			CommMode:        CommModeTCP,
		},
		Results: []TestResult{},
		Summary: Summary{},
	}

	var buf bytes.Buffer
	err := report.WriteJUnit(&buf)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "tests=\"0\"")
	assert.Contains(t, output, "failures=\"0\"")
}

func TestReport_WriteTable_AllStatusTypes(t *testing.T) {
	t.Parallel()

	report := &SuiteReport{
		SuiteName: "conformance",
		Plugin: PluginUnderTest{
			Name:            "test-plugin",
			Version:         "1.0.0",
			ProtocolVersion: "1.0",
			CommMode:        CommModeTCP,
		},
		Results: []TestResult{
			{TestName: "PassTest", Status: StatusPass, Category: CategoryProtocol},
			{
				TestName: "FailTest",
				Status:   StatusFail,
				Category: CategoryError,
				Error:    "assertion failed",
			},
			{
				TestName: "SkipTest",
				Status:   StatusSkip,
				Category: CategoryProtocol,
				Error:    "skipped",
			},
			{
				TestName: "ErrorTest",
				Status:   StatusError,
				Category: CategoryContext,
				Error:    "plugin crashed",
			},
		},
		Summary: Summary{Total: 4, Passed: 1, Failed: 1, Skipped: 1, Errors: 1},
	}

	var buf bytes.Buffer
	err := report.WriteTable(&buf)

	require.NoError(t, err)
	output := buf.String()

	// Check all status indicators are present
	assert.Contains(t, output, "✓") // Pass
	assert.Contains(t, output, "✗") // Fail
	assert.Contains(t, output, "⊘") // Skip
	assert.Contains(t, output, "!") // Error
}
