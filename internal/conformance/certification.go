package conformance

import (
	"fmt"
	"strings"
	"time"
)

// CertificationReport represents the result of a plugin certification process.
type CertificationReport struct {
	PluginName    string    `json:"plugin_name"`
	PluginVersion string    `json:"plugin_version"`
	Certified     bool      `json:"certified"`
	CertifiedAt   time.Time `json:"certified_at"`
	Issues        []string  `json:"issues,omitempty"`
	SuiteSummary  Summary   `json:"suite_summary"`
}

// Certify evaluates a SuiteReport and returns a CertificationReport for the specified plugin name and version.
// It sets CertifiedAt to the current time and copies the suite summary from suiteReport.
// If the suite summary indicates any failures, Certified is set to false and Issues is populated with entries
// formatted as "<TestName>: <Error>" for each result whose status is StatusFail or StatusError.
// If there are no failures, Certified is set to true.
// The returned *CertificationReport contains the plugin identification, certification status, timestamp,
// any collected issues, and the suite summary.
func Certify(suiteReport *SuiteReport, name, version string) *CertificationReport {
	report := &CertificationReport{
		PluginName:    name,
		PluginVersion: version,
		CertifiedAt:   time.Now(),
		SuiteSummary:  suiteReport.Summary,
	}

	// Include both failures and errors in certification check
	if suiteReport.Summary.Failed > 0 || suiteReport.Summary.Errors > 0 {
		report.Certified = false
		for _, res := range suiteReport.Results {
			if res.Status == StatusFail || res.Status == StatusError {
				report.Issues = append(report.Issues, fmt.Sprintf("%s: %s", res.TestName, res.Error))
			}
		}
	} else {
		report.Certified = true
	}

	return report
}

// GenerateMarkdown generates a markdown report for certification.
func (r *CertificationReport) GenerateMarkdown() string {
	status := "❌ FAILED"
	if r.Certified {
		status = "✅ CERTIFIED"
	}

	var sb strings.Builder
	sb.WriteString("# FinFocus Plugin Certification\n\n")
	fmt.Fprintf(&sb, "**Plugin**: %s\n", r.PluginName)
	fmt.Fprintf(&sb, "**Version**: %s\n", r.PluginVersion)
	fmt.Fprintf(&sb, "**Status**: %s\n", status)
	fmt.Fprintf(&sb, "**Date**: %s\n\n", r.CertifiedAt.Format(time.RFC1123))

	sb.WriteString("## Summary\n\n")
	fmt.Fprintf(&sb, "- Total Tests: %d\n", r.SuiteSummary.Total)
	fmt.Fprintf(&sb, "- Passed: %d\n", r.SuiteSummary.Passed)
	fmt.Fprintf(&sb, "- Failed: %d\n", r.SuiteSummary.Failed)
	fmt.Fprintf(&sb, "- Errors: %d\n", r.SuiteSummary.Errors)
	fmt.Fprintf(&sb, "- Skipped: %d\n\n", r.SuiteSummary.Skipped)

	if len(r.Issues) > 0 {
		sb.WriteString("## Issues\n\n")
		for _, issue := range r.Issues {
			sb.WriteString("- " + issue + "\n")
		}
	}

	return sb.String()
}
