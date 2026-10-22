package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

// demoPRRecommendations are the recommendations of rshade/finfocus-demo#1: the
// plain sum is 26.72 and the achievable figure is 20.15.
func demoPRRecommendations() []engine.Recommendation {
	modify := func(resource string, savings float64, current, recommended map[string]string) engine.Recommendation {
		return engine.Recommendation{
			ResourceID: resource, Type: "MODIFY", Currency: "USD", EstimatedSavings: savings,
			ActionDetail: &engine.RecommendationActionDetail{Modify: &engine.ModifyActionDetail{
				CurrentConfig: current, RecommendedConfig: recommended,
			}},
		}
	}
	upgrade := func(resource string, savings float64) engine.Recommendation {
		return modify(resource, savings,
			map[string]string{"instance_type": "t3.micro"}, map[string]string{"instance_type": "t3a.micro"})
	}
	graviton := func(resource string, savings float64) engine.Recommendation {
		return modify(resource, savings,
			map[string]string{"instance_type": "t3.micro", "architecture": "x86_64"},
			map[string]string{"instance_type": "t4g.micro", "architecture": "arm64"})
	}
	return []engine.Recommendation{
		upgrade("demo-instance", 0.73), graviton("demo-instance", 1.46),
		upgrade("legacy-instance", 7.01),
		upgrade("demo-instance-large", 5.84), graviton("demo-instance-large", 11.68),
	}
}

func TestRecommendationSavingsTotalsAreAchievable(t *testing.T) {
	t.Parallel()
	recs := demoPRRecommendations()

	t.Run("view total", func(t *testing.T) {
		t.Parallel()
		assert.InDelta(t, 20.15, calculateTotalSavings(recs), 1e-9)
	})

	t.Run("table summary", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		renderRecommendationsSummary(&buf, recs, "")
		assert.Contains(t, buf.String(), "Total Potential Savings: 20.15 USD")
	})

	t.Run("JSON and NDJSON summary", func(t *testing.T) {
		t.Parallel()
		summary := buildJSONSummary(recs)
		assert.InDelta(t, 20.15, summary.TotalSavings, 1e-9)
		assert.InDelta(t, 26.72, summary.SavingsByAction["MODIFY"], 1e-9)
	})
}
