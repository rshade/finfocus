package engine

import (
	"encoding/json"
	"fmt"
	"io"
)

// HistoryImportStatus marks cost reports whose Cloud identity import was incomplete.
type HistoryImportStatus struct {
	Incomplete        bool `json:"incomplete"`
	UnreadableUpdates int  `json:"unreadableUpdates"`
}

// RenderActualCostResultsWithHistory renders explicit import status even for empty reports.
// A nil status preserves the existing output byte for byte.
func RenderActualCostResultsWithHistory(
	writer io.Writer,
	format OutputFormat,
	results []CostResult,
	showConfidence bool,
	status *HistoryImportStatus,
) error {
	if status == nil {
		return RenderActualCostResults(writer, format, results, showConfidence)
	}
	rows := make([]CostResult, len(results))
	for i, result := range results {
		rows[i] = result
		rows[i].HistoryImport = status
		if !showConfidence {
			rows[i].Confidence = ConfidenceUnknown
		}
	}
	switch format {
	case OutputJSON:
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(struct {
			Results       []CostResult         `json:"results"`
			HistoryImport *HistoryImportStatus `json:"historyImport"`
		}{rows, status})
	case OutputNDJSON:
		encoder := json.NewEncoder(writer)
		for _, row := range rows {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
		return encoder.Encode(struct {
			Type          string               `json:"type"`
			HistoryImport *HistoryImportStatus `json:"historyImport"`
		}{"historyImport", status})
	case OutputTable:
		if err := RenderActualCostResults(writer, format, rows, showConfidence); err != nil {
			return err
		}
		_, err := fmt.Fprintf(
			writer,
			"Cloud history import incomplete: %d unreadable updates; totals may be incomplete.\n",
			status.UnreadableUpdates,
		)
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
