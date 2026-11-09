package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func failedRecommendationsResult() *engine.RecommendationsResult {
	return &engine.RecommendationsResult{
		Errors: []engine.RecommendationError{{
			PluginName: "aws-ce",
			Error:      "rpc error: code = DataLoss desc = invalid AWS recommendation data: rightsizing",
		}},
	}
}

func TestRenderRecommendationsTable_EmptyWithPluginFailure(t *testing.T) {
	t.Parallel()

	for name, render := range map[string]func(*bytes.Buffer, *engine.RecommendationsResult) error{
		"plain": func(w *bytes.Buffer, r *engine.RecommendationsResult) error {
			return renderRecommendationsTableWithVerbose(w, r, false)
		},
		"styled": func(w *bytes.Buffer, r *engine.RecommendationsResult) error {
			return renderStyledRecommendationsOutput(w, r, false)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			require.NoError(t, render(&buf, failedRecommendationsResult()))
			out := buf.String()
			assert.Contains(t, out, "No recommendations available: 1 plugin failed (see ERRORS below).\n")
			assert.Contains(t, out, "ERRORS\n======\naws-ce: rpc error: code = DataLoss")
			assert.NotContains(t, out, "No recommendations available.\n")
		})
	}
}

func TestRenderRecommendationsTable_EmptyCountsDistinctPlugins(t *testing.T) {
	t.Parallel()

	result := failedRecommendationsResult()
	result.Errors = append(result.Errors,
		engine.RecommendationError{PluginName: "aws-ce", Error: "second batch failed"},
		engine.RecommendationError{PluginName: "kubecost", Error: "unavailable"},
	)
	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsTableWithVerbose(&buf, result, false))
	assert.Contains(t, buf.String(), "No recommendations available: 2 plugins failed (see ERRORS below).\n")
}

func TestRenderRecommendationsTable_EmptyWithoutFailureUnchanged(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsTableWithVerbose(&buf, &engine.RecommendationsResult{}, false))
	out := buf.String()
	assert.True(t, strings.HasSuffix(out, "\nNo recommendations available.\n"), out)
	assert.NotContains(t, out, "ERRORS")
}

func TestRenderRecommendationsNDJSON_SummaryCarriesErrors(t *testing.T) {
	t.Parallel()

	var failed bytes.Buffer
	require.NoError(t, renderRecommendationsNDJSON(&failed, failedRecommendationsResult(), nil))
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.SplitN(failed.String(), "\n", 2)[0]), &summary))
	require.Contains(t, summary, "errors")
	assert.Equal(t,
		[]any{map[string]any{
			"pluginName": "aws-ce",
			"error":      "rpc error: code = DataLoss desc = invalid AWS recommendation data: rightsizing",
		}},
		summary["errors"])

	var clean bytes.Buffer
	require.NoError(t, renderRecommendationsNDJSON(&clean, &engine.RecommendationsResult{}, nil))
	assert.NotContains(t, clean.String(), `"errors"`)
}

func TestRenderRecommendationsOutput_PluginFailureExitsZero(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := RenderRecommendationsOutput(t.Context(), cmd, "table", failedRecommendationsResult(), false, nil)
	require.NoError(t, err, "a plugin failure is reported, not turned into a non-zero exit")
	assert.Contains(t, out.String(), "1 plugin failed")
}
