package engine

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryImportOutput(t *testing.T) {
	t.Parallel()
	status := &HistoryImportStatus{Incomplete: true, UnreadableUpdates: 2}
	for _, format := range []OutputFormat{OutputJSON, OutputNDJSON, OutputTable} {
		for _, empty := range []bool{false, true} {
			t.Run(string(format)+"/"+strconv.FormatBool(empty), func(t *testing.T) {
				t.Parallel()
				var rows []CostResult
				if !empty {
					rows = []CostResult{{ResourceID: "r", TotalCost: 42, HistoryImport: status}}
				}
				var output bytes.Buffer
				require.NoError(t, RenderActualCostResultsWithHistory(&output, format, rows, false, status))
				if format == OutputTable {
					assert.Contains(t, output.String(), "2 unreadable")
					return
				}
				assertHistoryImportPayload(t, format, output.Bytes(), status, empty)
			})
		}
	}
	row := CostResult{ResourceID: "r"}
	data, err := json.Marshal(row)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "historyImport")
	for _, format := range []OutputFormat{OutputJSON, OutputNDJSON, OutputTable} {
		var before, after bytes.Buffer
		require.NoError(t, RenderActualCostResults(&before, format, []CostResult{row}, false))
		require.NoError(t, RenderActualCostResultsWithHistory(&after, format, []CostResult{row}, false, nil))
		assert.Equal(t, before.String(), after.String())
	}
}

func assertHistoryImportPayload(
	t *testing.T,
	format OutputFormat,
	data []byte,
	status *HistoryImportStatus,
	empty bool,
) {
	t.Helper()
	if format == OutputJSON {
		var got struct {
			Results       []CostResult        `json:"results"`
			HistoryImport HistoryImportStatus `json:"historyImport"`
		}
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Equal(t, *status, got.HistoryImport)
		if !empty {
			require.Len(t, got.Results, 1)
			assert.Equal(t, status, got.Results[0].HistoryImport)
		}
	} else {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		var last map[string]any
		require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &last))
		assert.Equal(t, "historyImport", last["type"])
	}
}
