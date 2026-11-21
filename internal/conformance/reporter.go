package conformance

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// WriteTable writes the report as human-readable table to the given writer.
// This implements FR-007: Clear, actionable error messages for conformance failures.
// This implements FR-017: Human-readable test reports for developers.
func (r *SuiteReport) WriteTable(w io.Writer) error {
	// Helper to propagate write errors
	var writeErr error
	fprintln := func(a ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintln(w, a...)
	}
	fprintf := func(format string, a ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(w, format, a...)
	}

	// Header
	fprintln("CONFORMANCE TEST RESULTS")
	fprintln("========================")
	fprintf(
		"Plugin: %s v%s (protocol v%s)\n",
		r.Plugin.Name,
		r.Plugin.Version,
		r.Plugin.ProtocolVersion,
	)
	fprintf("Mode:   %s\n", strings.ToUpper(string(r.Plugin.CommMode)))
	fprintln()

	// Tests
	fprintln("TESTS")
	fprintln("-----")

	for _, result := range r.Results {
		if writeErr != nil {
			break
		}
		statusIcon := getStatusIcon(result.Status)
		duration := formatDuration(result.Duration)

		fprintf("%s %-45s [%7s]\n", statusIcon, result.TestName, duration)

		// Show error message for failed/error tests
		if result.Error != "" && (result.Status == StatusFail || result.Status == StatusError) {
			fprintf("  Error: %s\n", result.Error)
		}

		// Show skip reason
		if result.Status == StatusSkip && result.Error != "" {
			fprintf("  (%s)\n", result.Error)
		}

		// Show warning message
		if result.Status == StatusWarn && result.Error != "" {
			fprintf("  Warning: %s\n", result.Error)
		}
	}

	fprintln()

	// Summary
	fprintln("SUMMARY")
	fprintln("-------")
	fprintf("Total: %d | Passed: %d | Failed: %d | Skipped: %d | Duration: %s\n",
		r.Summary.Total, r.Summary.Passed, r.Summary.Failed, r.Summary.Skipped,
		formatTotalDuration(r.TotalTime))

	if r.Summary.Errors > 0 {
		fprintf("Errors: %d\n", r.Summary.Errors)
	}

	if r.Summary.Warnings > 0 {
		fprintf("Warnings: %d\n", r.Summary.Warnings)
	}

	return writeErr
}

// WriteJSON writes the report as JSON to the given writer.
// This implements FR-016: Machine-readable JSON format for programmatic access.
func (r *SuiteReport) WriteJSON(w io.Writer) error {
	// Create a custom struct for JSON output with proper duration formatting
	type jsonResult struct {
		Name       string   `json:"name"`
		Category   Category `json:"category"`
		Status     Status   `json:"status"`
		DurationMS int64    `json:"duration_ms"`
		Error      string   `json:"error,omitempty"`
		Details    string   `json:"details,omitempty"`
	}

	type jsonReport struct {
		Suite      string          `json:"suite"`
		Plugin     PluginUnderTest `json:"plugin"`
		Results    []jsonResult    `json:"results"`
		Summary    Summary         `json:"summary"`
		DurationMS int64           `json:"duration_ms"`
		Timestamp  string          `json:"timestamp"`
	}

	results := make([]jsonResult, len(r.Results))
	for i, res := range r.Results {
		results[i] = jsonResult{
			Name:       res.TestName,
			Category:   res.Category,
			Status:     res.Status,
			DurationMS: res.Duration.Milliseconds(),
			Error:      res.Error,
			Details:    res.Details,
		}
	}

	report := jsonReport{
		Suite:      r.SuiteName,
		Plugin:     r.Plugin,
		Results:    results,
		Summary:    r.Summary,
		DurationMS: r.TotalTime.Milliseconds(),
		Timestamp:  r.Timestamp.Format(time.RFC3339),
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// JUnit XML type definitions for WriteJUnit output.
type (
	junitProperty struct {
		XMLName xml.Name `xml:"property"`
		Name    string   `xml:"name,attr"`
		Value   string   `xml:"value,attr"`
	}

	junitProperties struct {
		XMLName    xml.Name        `xml:"properties"`
		Properties []junitProperty `xml:"property"`
	}

	junitFailure struct {
		XMLName xml.Name `xml:"failure"`
		Message string   `xml:"message,attr"`
		Type    string   `xml:"type,attr"`
		Content string   `xml:",chardata"`
	}

	junitSkipped struct {
		XMLName xml.Name `xml:"skipped"`
		Message string   `xml:"message,attr,omitempty"`
	}

	junitTestcase struct {
		XMLName   xml.Name      `xml:"testcase"`
		Name      string        `xml:"name,attr"`
		Classname string        `xml:"classname,attr"`
		Time      string        `xml:"time,attr"`
		Failure   *junitFailure `xml:"failure,omitempty"`
		Skipped   *junitSkipped `xml:"skipped,omitempty"`
		// SystemOut carries a warning on a passing testcase; JUnit has no
		// warning state, and a failure would break CI that reads the file.
		SystemOut string `xml:"system-out,omitempty"`
	}

	junitTestsuite struct {
		XMLName    xml.Name        `xml:"testsuite"`
		Name       string          `xml:"name,attr"`
		Tests      int             `xml:"tests,attr"`
		Failures   int             `xml:"failures,attr"`
		Skipped    int             `xml:"skipped,attr"`
		Time       string          `xml:"time,attr"`
		Timestamp  string          `xml:"timestamp,attr"`
		Properties junitProperties `xml:"properties"`
		Testcases  []junitTestcase `xml:"testcase"`
	}

	junitTestsuites struct {
		XMLName   xml.Name         `xml:"testsuites"`
		Name      string           `xml:"name,attr"`
		Tests     int              `xml:"tests,attr"`
		Failures  int              `xml:"failures,attr"`
		Skipped   int              `xml:"skipped,attr"`
		Time      string           `xml:"time,attr"`
		Testsuite []junitTestsuite `xml:"testsuite"`
	}
)

// WriteJUnit writes the report as JUnit XML to the given writer.
// This implements FR-016: Machine-readable JUnit XML format for CI integration.
func (r *SuiteReport) WriteJUnit(w io.Writer) error {
	testcases := r.buildJUnitTestcases()
	output := r.buildJUnitOutput(testcases)

	// Write XML header explicitly so we can surface any write error.
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	return encoder.Encode(output)
}

// buildJUnitTestcases converts test results to JUnit testcase elements.
func (r *SuiteReport) buildJUnitTestcases() []junitTestcase {
	testcases := make([]junitTestcase, len(r.Results))
	for i, res := range r.Results {
		tc := junitTestcase{
			Name:      res.TestName,
			Classname: string(res.Category),
			Time:      fmt.Sprintf("%.2f", res.Duration.Seconds()),
		}

		switch res.Status {
		case StatusFail, StatusError:
			tc.Failure = &junitFailure{
				Message: res.Error,
				Type:    "AssertionError",
				Content: res.Error,
			}
		case StatusSkip:
			tc.Skipped = &junitSkipped{
				Message: res.Error,
			}
		case StatusWarn:
			tc.SystemOut = "WARNING: " + res.Error
		case StatusPass:
			// No additional elements needed for passing tests
		}

		testcases[i] = tc
	}
	return testcases
}

// buildJUnitOutput creates the complete JUnit XML structure.
func (r *SuiteReport) buildJUnitOutput(testcases []junitTestcase) junitTestsuites {
	return junitTestsuites{
		Name:     "finfocus-conformance",
		Tests:    r.Summary.Total,
		Failures: r.Summary.Failed + r.Summary.Errors,
		Skipped:  r.Summary.Skipped,
		Time:     fmt.Sprintf("%.1f", r.TotalTime.Seconds()),
		Testsuite: []junitTestsuite{
			{
				Name:      r.SuiteName,
				Tests:     r.Summary.Total,
				Failures:  r.Summary.Failed + r.Summary.Errors,
				Skipped:   r.Summary.Skipped,
				Time:      fmt.Sprintf("%.1f", r.TotalTime.Seconds()),
				Timestamp: r.Timestamp.Format(time.RFC3339),
				Properties: junitProperties{
					Properties: []junitProperty{
						{Name: "plugin.name", Value: r.Plugin.Name},
						{Name: "plugin.version", Value: r.Plugin.Version},
						{Name: "protocol.version", Value: r.Plugin.ProtocolVersion},
					},
				},
				Testcases: testcases,
			},
		},
	}
}

// getStatusIcon returns the appropriate icon for a test status.
func getStatusIcon(status Status) string {
	switch status {
	case StatusPass:
		return "✓"
	case StatusFail:
		return "✗"
	case StatusSkip:
		return "⊘"
	case StatusError:
		return "!"
	case StatusWarn:
		return "⚠"
	default:
		return "?"
	}
}

// formatDuration formats a duration for display in the test results.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "  --  "
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%5dμs", d.Microseconds())
	}
	return fmt.Sprintf("%5dms", d.Milliseconds())
}

// formatTotalDuration formats the total suite duration.
func formatTotalDuration(d time.Duration) string {
	if d >= time.Minute {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
