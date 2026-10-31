package viewmodel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func TestBuildAnomalyDisplay(t *testing.T) {
	t.Parallel()

	period := engine.DateRange{
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name    string
		anomaly engine.Anomaly
		want    AnomalyDisplay
	}{
		{
			name: "overspend in USD",
			anomaly: engine.Anomaly{
				Period: period, Baseline: 720, Observed: 1080, Deviation: 360, PercentDrift: 50, Currency: "USD",
			},
			want: AnomalyDisplay{
				Baseline: "$720.00", Observed: "$1,080.00", Deviation: "+$360.00",
				Drift: "+50.0%", Period: "Oct 1 – Oct 11, 2026",
			},
		},
		{
			name: "underspend keeps the sign",
			anomaly: engine.Anomaly{
				Period: period, Baseline: 500, Observed: 300, Deviation: -200, PercentDrift: -40, Currency: "USD",
			},
			want: AnomalyDisplay{
				Baseline: "$500.00", Observed: "$300.00", Deviation: "-$200.00",
				Drift: "-40.0%", Period: "Oct 1 – Oct 11, 2026",
			},
		},
		{
			name: "euro uses its own symbol",
			anomaly: engine.Anomaly{
				Period: period, Baseline: 1000, Observed: 1234.5, Deviation: 234.5,
				PercentDrift: 23.46, Currency: "EUR",
			},
			want: AnomalyDisplay{
				Baseline: "€1,000.00", Observed: "€1,234.50", Deviation: "+€234.50",
				Drift: "+23.5%", Period: "Oct 1 – Oct 11, 2026",
			},
		},
		{
			name: "missing currency falls back to dollars",
			anomaly: engine.Anomaly{
				Period: period, Baseline: 10, Observed: 20, Deviation: 10, PercentDrift: 100,
			},
			want: AnomalyDisplay{
				Baseline: "$10.00", Observed: "$20.00", Deviation: "+$10.00",
				Drift: "+100.0%", Period: "Oct 1 – Oct 11, 2026",
			},
		},
		{
			name: "zero period is empty",
			anomaly: engine.Anomaly{
				Baseline: 100, Observed: 150, Deviation: 50, PercentDrift: 50, Currency: "USD",
			},
			want: AnomalyDisplay{
				Baseline: "$100.00", Observed: "$150.00", Deviation: "+$50.00", Drift: "+50.0%",
			},
		},
		{
			name: "period across a year boundary names both years",
			anomaly: engine.Anomaly{
				Period: engine.DateRange{
					Start: time.Date(2025, 12, 28, 0, 0, 0, 0, time.UTC),
					End:   time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
				},
				Baseline: 100, Observed: 150, Deviation: 50, PercentDrift: 50, Currency: "USD",
			},
			want: AnomalyDisplay{
				Baseline: "$100.00", Observed: "$150.00", Deviation: "+$50.00",
				Drift: "+50.0%", Period: "Dec 28, 2025 – Jan 4, 2026",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, BuildAnomalyDisplay(tt.anomaly))
		})
	}
}

func TestAnomalySummary(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "2 of 10 resources", AnomalySummary(2, 10))
	assert.Equal(t, "0 of 0 resources", AnomalySummary(0, 0))
}
