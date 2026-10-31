package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// RenderAnomaliesAsTable writes the anomaly list as an aligned table followed
// by a count line. It prints no total: rows may be in different currencies.
func RenderAnomaliesAsTable(w io.Writer, report AnomalyReport) error {
	if len(report.Anomalies) == 0 {
		if _, err := fmt.Fprintf(w, "No anomalies found (%d resources considered)\n", report.Considered); err != nil {
			return fmt.Errorf("writing anomalies: %w", err)
		}
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, defaultTabPadding, ' ', 0)
	fmt.Fprintln(tw, "RESOURCE\tTYPE\tBASELINE\tOBSERVED\tDEVIATION\tDRIFT %\tCURRENCY")
	for _, a := range report.Anomalies {
		fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%+.2f\t%+.1f%%\t%s\n",
			a.Resource, a.Type, a.Baseline, a.Observed, a.Deviation, a.PercentDrift, a.Currency)
	}
	fmt.Fprintf(tw, "\n%d of %d resources\n", len(report.Anomalies), report.Considered)
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("writing anomalies: %w", err)
	}
	return nil
}

// RenderAnomaliesAsJSON writes the report as one indented JSON document with
// the considered count and the anomaly array.
func RenderAnomaliesAsJSON(w io.Writer, report AnomalyReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling anomalies: %w", err)
	}
	if _, err = fmt.Fprintf(w, "%s\n", data); err != nil {
		return fmt.Errorf("writing anomalies: %w", err)
	}
	return nil
}

// RenderAnomaliesAsNDJSON writes one anomaly per line. An empty report writes
// nothing.
func RenderAnomaliesAsNDJSON(w io.Writer, report AnomalyReport) error {
	for _, a := range report.Anomalies {
		data, err := json.Marshal(a)
		if err != nil {
			return fmt.Errorf("marshaling anomaly: %w", err)
		}
		if _, err = fmt.Fprintf(w, "%s\n", data); err != nil {
			return fmt.Errorf("writing NDJSON line: %w", err)
		}
	}
	return nil
}
