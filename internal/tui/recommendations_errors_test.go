package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func recommendationPluginErrors() []engine.RecommendationError {
	return []engine.RecommendationError{{
		PluginName: "aws-ce",
		Error:      "rpc error: code = DataLoss desc = invalid AWS recommendation data: rightsizing",
	}}
}

func TestGolden_RecommendationsList_PluginErrorsEmpty(t *testing.T) {
	t.Parallel()

	model := NewRecommendationsViewModelWithProfile(nil, "").WithPluginErrors(recommendationPluginErrors())
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	output := model.renderListView()
	testGolden(t, "recommendations_list_plugin_errors_empty", output)

	plain := ansi.Strip(output)
	assert.Contains(t, plain, "\nPLUGIN ERRORS ")
	assert.Contains(
		t,
		plain,
		"  aws-ce: rpc error: code = DataLoss desc = invalid AWS recommendation data: rightsizing",
	)
}

func TestGolden_RecommendationsList_PluginErrorsPartial(t *testing.T) {
	t.Parallel()

	recs := []engine.Recommendation{
		{ResourceID: "i-plain", Type: "RIGHTSIZE", Description: "Downsize", EstimatedSavings: 5, Currency: "USD"},
	}
	model := NewRecommendationsViewModelWithProfile(recs, "").WithPluginErrors(recommendationPluginErrors())
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	output := model.renderListView()
	testGolden(t, "recommendations_list_plugin_errors_partial", output)

	plain := ansi.Strip(output)
	assert.Contains(t, plain, "\nPLUGIN ERRORS ")
	assert.Contains(t, plain, "i-plain")
}

func TestRecommendationsList_NoPluginErrorsHasNoSection(t *testing.T) {
	t.Parallel()

	model := NewRecommendationsViewModelWithProfile(nil, "")
	assert.NotContains(t, ansi.Strip(model.renderListView()), "PLUGIN ERRORS")
}
