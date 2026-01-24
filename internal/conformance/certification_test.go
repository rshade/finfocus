package conformance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/conformance"
)

func TestCertification_Pass(t *testing.T) {
	// Create a report with all tests passing
	suiteReport := &conformance.SuiteReport{
		Summary: conformance.Summary{
			Total:   10,
			Passed:  10,
			Failed:  0,
			Skipped: 0,
		},
		Results: []conformance.TestResult{
			{TestName: "Protocol.Name", Status: conformance.StatusPass},
			{TestName: "Protocol.Version", Status: conformance.StatusPass},
		},
	}

	certReport := conformance.Certify(suiteReport, "test-plugin", "v1.0.0")

	assert.True(t, certReport.Certified)
	assert.Equal(t, "test-plugin", certReport.PluginName)
	assert.Equal(t, "v1.0.0", certReport.PluginVersion)
	assert.Empty(t, certReport.Issues)
}

func TestCertification_Fail(t *testing.T) {
	// Create a report with failures
	suiteReport := &conformance.SuiteReport{
		Summary: conformance.Summary{
			Total:   10,
			Passed:  9,
			Failed:  1,
			Skipped: 0,
		},
		Results: []conformance.TestResult{
			{TestName: "Protocol.Name", Status: conformance.StatusPass},
			{TestName: "Protocol.Version", Status: conformance.StatusFail, Error: "version mismatch"},
		},
	}

	certReport := conformance.Certify(suiteReport, "test-plugin", "v1.0.0")

	assert.False(t, certReport.Certified)
	assert.NotEmpty(t, certReport.Issues)
	assert.Contains(t, certReport.Issues[0], "Protocol.Version")
}
