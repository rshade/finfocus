package conformance

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reportWith(results ...TestResult) *SuiteReport {
	return &SuiteReport{
		SuiteName: "conformance",
		Plugin:    PluginUnderTest{Name: "finfocus-plugin-aws-public", CommMode: CommModeTCP},
		Results:   results,
		Summary:   calculateSummary(results),
		TotalTime: time.Second,
		Timestamp: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
	}
}

func warnResult() TestResult {
	return TestResult{
		TestName: "Name_MatchesInstallName", Category: CategoryProtocol, Status: StatusWarn,
		Error: `plugin reports "finfocus-plugin-aws-public"; its install name is "aws-public" (from install-layout)`,
	}
}

func passResult() TestResult {
	return TestResult{TestName: "Name_ReturnsPluginIdentifier", Category: CategoryProtocol, Status: StatusPass}
}

func TestCalculateSummary_CountsWarnings(t *testing.T) {
	t.Parallel()

	summary := calculateSummary([]TestResult{passResult(), warnResult(), warnResult()})

	assert.Equal(t, Summary{Total: 3, Passed: 1, Warnings: 2}, summary)
}

func TestCertify_WarningsDoNotBlock(t *testing.T) {
	t.Parallel()

	report := Certify(reportWith(passResult(), warnResult()), "aws-public", "v0.2.5")

	assert.True(t, report.Certified)
	assert.Empty(t, report.Issues)
	assert.Contains(t, report.GenerateMarkdown(), "- Warnings: 1\n")
}

func TestReporter_Warnings(t *testing.T) {
	t.Parallel()

	t.Run("table shows the warning and its count", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, reportWith(passResult(), warnResult()).WriteTable(&buf))
		out := buf.String()
		assert.Contains(t, out, "⚠ Name_MatchesInstallName")
		assert.Contains(t, out, `  Warning: plugin reports "finfocus-plugin-aws-public"`)
		assert.Contains(t, out, "Warnings: 1\n")
	})

	t.Run("table without warnings is unchanged", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, reportWith(passResult()).WriteTable(&buf))
		assert.NotContains(t, buf.String(), "Warning")
	})

	t.Run("json carries the status and count", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, reportWith(passResult(), warnResult()).WriteJSON(&buf))
		var doc struct {
			Results []struct {
				Status string `json:"status"`
			} `json:"results"`
			Summary struct {
				Warnings int `json:"warnings"`
			} `json:"summary"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
		assert.Equal(t, "warn", doc.Results[1].Status)
		assert.Equal(t, 1, doc.Summary.Warnings)
	})

	t.Run("junit passes the case and notes the warning", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, reportWith(passResult(), warnResult()).WriteJUnit(&buf))
		out := buf.String()
		assert.Contains(t, out, `<system-out>WARNING: plugin reports &#34;finfocus-plugin-aws-public&#34;`)
		assert.Contains(t, out, `failures="0"`)
		assert.NotContains(t, out, "<failure")
	})
}
