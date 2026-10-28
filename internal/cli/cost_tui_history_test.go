package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
)

func TestIncompleteActualTablePreservesRendering(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, group                  string
		breakdown, confidence, empty bool
	}{
		{name: "breakdown and confidence", breakdown: true, confidence: true},
		{name: "daily", group: "daily"},
		{name: "monthly", group: "monthly"},
		{name: "empty daily", group: "daily", empty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			view := historyRenderingView(tc.empty)
			baseline := renderHistoryView(t, view, "table", tc.group, tc.breakdown, tc.confidence)
			require.Contains(t, baseline, "ERRORS")
			view.HistoryImport = &engine.HistoryImportStatus{Incomplete: true, UnreadableUpdates: 2}
			incomplete := renderHistoryView(t, view, "table", tc.group, tc.breakdown, tc.confidence)
			assert.Equal(
				t,
				baseline+"Cloud history import incomplete: 2 unreadable updates; totals may be incomplete.\n",
				incomplete,
			)
		})
	}
}

func TestIncompleteActualMachineOutputPreservesGrouping(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"json", "ndjson"} {
		for _, group := range []string{"", "daily", "monthly"} {
			for _, empty := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/empty=%t", format, group, empty), func(t *testing.T) {
					t.Parallel()
					view := historyRenderingView(empty)
					status := &engine.HistoryImportStatus{Incomplete: true, UnreadableUpdates: 2}
					for i := range view.Results {
						view.Results[i].HistoryImport = status
					}
					baseline := renderHistoryView(t, view, format, group, false, false)
					view.HistoryImport = status
					output := renderHistoryView(t, view, format, group, true, true)
					if format == "json" {
						var envelope struct {
							Results       json.RawMessage            `json:"results"`
							HistoryImport engine.HistoryImportStatus `json:"historyImport"`
						}
						require.NoError(t, json.Unmarshal([]byte(output), &envelope))
						assert.JSONEq(t, baseline, string(envelope.Results))
						assert.Equal(t, *status, envelope.HistoryImport)
					} else {
						require.True(t, strings.HasPrefix(output, baseline))
						tail := strings.TrimPrefix(output, baseline)
						assert.JSONEq(t,
							`{"type":"historyImport","historyImport":{"incomplete":true,"unreadableUpdates":2}}`,
							tail,
						)
					}
				})
			}
		}
	}
}

func historyRenderingView(empty bool) *engine.CostResultWithErrors {
	view := projectedView()
	view.Results[0].StartDate = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	view.Results[0].EndDate = view.Results[0].StartDate.Add(48 * time.Hour)
	view.Results[0].DailyCosts = []float64{40, 60}
	if empty {
		view.Results = nil
	}
	view.Errors = []engine.ErrorDetail{
		{ResourceID: "unpriced", PluginName: "fixture", Error: errors.New("billing unavailable")},
	}
	return view
}

func renderHistoryView(
	t *testing.T,
	view *engine.CostResultWithErrors,
	format, group string,
	breakdown, confidence bool,
) string {
	t.Helper()
	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	require.NoError(
		t,
		cli.RenderActualCostOutput(context.Background(), cmd, format, view, group, false, breakdown, confidence),
	)
	return output.String()
}
