package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/sourcepos"
)

func TestOverviewDetailRecommendations(t *testing.T) {
	t.Parallel()
	recs := []engine.Recommendation{
		{Description: "active", EstimatedSavings: 12.345},
		{Description: "dismissed", Status: engine.RecommendationStatusDismissed},
		{Description: "snoozed", Status: engine.RecommendationStatusSnoozed},
	}
	detail := OverviewDetailRecommendations(recs)
	require.Len(t, detail, 1)
	assert.Equal(t, "active", detail[0].Recommendation.Description)
	assert.Equal(t, engine.FormatOverviewCurrency(12.345), detail[0].SavingsDisplay)
}

func TestBuildOverviewDetailDisplayDeclaredAt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		location *sourcepos.Location
		want     string
	}{
		{name: "no location", location: nil, want: ""},
		{
			name:     "file and line",
			location: &sourcepos.Location{File: "src/app.ts", Line: 12, Column: 3},
			want:     "src/app.ts:12",
		},
		{
			name:     "nested path",
			location: &sourcepos.Location{File: "infra/net/vpc.py", Line: 4},
			want:     "infra/net/vpc.py:4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := engine.OverviewRowResult{Location: tt.location}
			assert.Equal(t, tt.want, BuildOverviewDetailDisplay(row, 7).DeclaredAt)
		})
	}
}
