package conformance_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/conformance"
)

func TestStatus_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   conformance.Status
		expected string
	}{
		{"pass status", conformance.StatusPass, "pass"},
		{"fail status", conformance.StatusFail, "fail"},
		{"skip status", conformance.StatusSkip, "skip"},
		{"error status", conformance.StatusError, "error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, string(tc.status))
		})
	}
}

func TestCategory_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category conformance.Category
		expected string
	}{
		{"protocol category", conformance.CategoryProtocol, "protocol"},
		{"performance category", conformance.CategoryPerformance, "performance"},
		{"error category", conformance.CategoryError, "error"},
		{"context category", conformance.CategoryContext, "context"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, string(tc.category))
		})
	}
}

func TestAllCategories(t *testing.T) {
	t.Parallel()

	categories := conformance.AllCategories()

	require.Len(t, categories, 4)
	assert.Contains(t, categories, conformance.CategoryProtocol)
	assert.Contains(t, categories, conformance.CategoryPerformance)
	assert.Contains(t, categories, conformance.CategoryError)
	assert.Contains(t, categories, conformance.CategoryContext)
}

func TestIsValidCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category string
		expected bool
	}{
		{"valid protocol", "protocol", true},
		{"valid performance", "performance", true},
		{"valid error", "error", true},
		{"valid context", "context", true},
		{"invalid empty", "", false},
		{"invalid unknown", "unknown", false},
		{"invalid case", "Protocol", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, conformance.IsValidCategory(tc.category))
		})
	}
}

func TestVerbosity_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		verbosity conformance.Verbosity
		expected  string
	}{
		{"quiet verbosity", conformance.VerbosityQuiet, "quiet"},
		{"normal verbosity", conformance.VerbosityNormal, "normal"},
		{"verbose verbosity", conformance.VerbosityVerbose, "verbose"},
		{"debug verbosity", conformance.VerbosityDebug, "debug"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, string(tc.verbosity))
		})
	}
}

func TestIsValidVerbosity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		verbosity string
		expected  bool
	}{
		{"valid quiet", "quiet", true},
		{"valid normal", "normal", true},
		{"valid verbose", "verbose", true},
		{"valid debug", "debug", true},
		{"invalid empty", "", false},
		{"invalid unknown", "unknown", false},
		{"invalid case", "Quiet", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, conformance.IsValidVerbosity(tc.verbosity))
		})
	}
}

func TestCommMode_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		commMode conformance.CommMode
		expected string
	}{
		{"tcp mode", conformance.CommModeTCP, "tcp"},
		{"stdio mode", conformance.CommModeStdio, "stdio"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, string(tc.commMode))
		})
	}
}

func TestIsValidCommMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     string
		expected bool
	}{
		{"valid tcp", "tcp", true},
		{"valid stdio", "stdio", true},
		{"invalid empty", "", false},
		{"invalid unknown", "unknown", false},
		{"invalid case", "TCP", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, conformance.IsValidCommMode(tc.mode))
		})
	}
}

func TestTestCase(t *testing.T) {
	t.Parallel()

	tc := conformance.TestCase{
		Name:            "Name_ReturnsPluginIdentifier",
		Category:        conformance.CategoryProtocol,
		Description:     "Verifies plugin returns its identifier via Name RPC",
		Timeout:         5 * time.Second,
		RequiredMethods: []string{"Name"},
	}

	assert.Equal(t, "Name_ReturnsPluginIdentifier", tc.Name)
	assert.Equal(t, conformance.CategoryProtocol, tc.Category)
	assert.Equal(t, "Verifies plugin returns its identifier via Name RPC", tc.Description)
	assert.Equal(t, 5*time.Second, tc.Timeout)
	assert.Equal(t, []string{"Name"}, tc.RequiredMethods)
}

func TestTestResult(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := conformance.TestResult{
		TestName:  "Name_ReturnsPluginIdentifier",
		Category:  conformance.CategoryProtocol,
		Status:    conformance.StatusPass,
		Duration:  50 * time.Millisecond,
		Timestamp: now,
	}

	assert.Equal(t, "Name_ReturnsPluginIdentifier", result.TestName)
	assert.Equal(t, conformance.CategoryProtocol, result.Category)
	assert.Equal(t, conformance.StatusPass, result.Status)
	assert.Equal(t, 50*time.Millisecond, result.Duration)
	assert.Equal(t, now, result.Timestamp)
	assert.Empty(t, result.Error)
	assert.Empty(t, result.Details)
}

func TestTestResult_WithError(t *testing.T) {
	t.Parallel()

	result := conformance.TestResult{
		TestName: "GetProjectedCost_InvalidResource",
		Category: conformance.CategoryError,
		Status:   conformance.StatusFail,
		Duration: 110 * time.Millisecond,
		Error:    "expected NotFound, got InvalidArgument",
		Details:  "Request: {...}, Response: {...}",
	}

	assert.Equal(t, conformance.StatusFail, result.Status)
	assert.Equal(t, "expected NotFound, got InvalidArgument", result.Error)
	assert.Equal(t, "Request: {...}, Response: {...}", result.Details)
}

func TestPluginUnderTest(t *testing.T) {
	t.Parallel()

	plugin := conformance.PluginUnderTest{
		Path:            "/path/to/plugin",
		Name:            "aws-cost",
		Version:         "1.2.0",
		ProtocolVersion: "1.0",
		CommMode:        conformance.CommModeTCP,
	}

	assert.Equal(t, "/path/to/plugin", plugin.Path)
	assert.Equal(t, "aws-cost", plugin.Name)
	assert.Equal(t, "1.2.0", plugin.Version)
	assert.Equal(t, "1.0", plugin.ProtocolVersion)
	assert.Equal(t, conformance.CommModeTCP, plugin.CommMode)
}

func TestSuiteConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := conformance.SuiteConfig{
		PluginPath: "/path/to/plugin",
	}

	assert.Equal(t, "/path/to/plugin", cfg.PluginPath)
	assert.Empty(t, cfg.CommMode)   // Will be defaulted by NewSuite
	assert.Empty(t, cfg.Verbosity)  // Will be defaulted by NewSuite
	assert.Zero(t, cfg.Timeout)     // Will be defaulted by NewSuite
	assert.Nil(t, cfg.Categories)   // Empty means all categories
	assert.Empty(t, cfg.TestFilter) // Empty means no filter
}

func TestSuiteConfig_FullConfiguration(t *testing.T) {
	t.Parallel()

	cfg := conformance.SuiteConfig{
		PluginPath:   "/path/to/plugin",
		CommMode:     conformance.CommModeTCP,
		Verbosity:    conformance.VerbosityVerbose,
		OutputFormat: "json",
		OutputPath:   "/path/to/output.json",
		Timeout:      10 * time.Minute,
		Categories:   []conformance.Category{conformance.CategoryProtocol, conformance.CategoryError},
		TestFilter:   "Name_.*",
	}

	assert.Equal(t, "/path/to/plugin", cfg.PluginPath)
	assert.Equal(t, conformance.CommModeTCP, cfg.CommMode)
	assert.Equal(t, conformance.VerbosityVerbose, cfg.Verbosity)
	assert.Equal(t, "json", cfg.OutputFormat)
	assert.Equal(t, "/path/to/output.json", cfg.OutputPath)
	assert.Equal(t, 10*time.Minute, cfg.Timeout)
	assert.Len(t, cfg.Categories, 2)
	assert.Equal(t, "Name_.*", cfg.TestFilter)
}

func TestSummary(t *testing.T) {
	t.Parallel()

	summary := conformance.Summary{
		Total:   20,
		Passed:  18,
		Failed:  1,
		Skipped: 1,
		Errors:  0,
	}

	assert.Equal(t, 20, summary.Total)
	assert.Equal(t, 18, summary.Passed)
	assert.Equal(t, 1, summary.Failed)
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, 0, summary.Errors)

	// Verify counts add up
	assert.Equal(t, summary.Total, summary.Passed+summary.Failed+summary.Skipped+summary.Errors)
}

func TestSuiteReport(t *testing.T) {
	t.Parallel()

	startTime := time.Now()
	endTime := startTime.Add(4500 * time.Millisecond)

	report := conformance.SuiteReport{
		SuiteName: "conformance",
		Plugin: conformance.PluginUnderTest{
			Path:            "/path/to/plugin",
			Name:            "aws-cost",
			Version:         "1.2.0",
			ProtocolVersion: "1.0",
			CommMode:        conformance.CommModeTCP,
		},
		Results: []conformance.TestResult{
			{
				TestName: "Name_ReturnsPluginIdentifier",
				Category: conformance.CategoryProtocol,
				Status:   conformance.StatusPass,
				Duration: 50 * time.Millisecond,
			},
			{
				TestName: "GetProjectedCost_InvalidResource",
				Category: conformance.CategoryError,
				Status:   conformance.StatusFail,
				Duration: 110 * time.Millisecond,
				Error:    "expected NotFound, got InvalidArgument",
			},
		},
		Summary: conformance.Summary{
			Total:   20,
			Passed:  18,
			Failed:  1,
			Skipped: 1,
			Errors:  0,
		},
		StartTime: startTime,
		EndTime:   endTime,
		TotalTime: 4500 * time.Millisecond,
		Timestamp: endTime,
	}

	assert.Equal(t, "conformance", report.SuiteName)
	assert.Equal(t, "aws-cost", report.Plugin.Name)
	assert.Len(t, report.Results, 2)
	assert.Equal(t, 20, report.Summary.Total)
	assert.Equal(t, startTime, report.StartTime)
	assert.Equal(t, endTime, report.EndTime)
	assert.Equal(t, 4500*time.Millisecond, report.TotalTime)
}

func TestConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10*time.Second, conformance.DefaultTimeout)
	assert.Equal(t, 5*time.Minute, conformance.DefaultSuiteTimeout)
	assert.Equal(t, 1000, conformance.MaxBatchSize)
	assert.Equal(t, "1.0", conformance.ProtocolVersion)
}
