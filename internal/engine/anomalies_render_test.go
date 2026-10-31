package engine

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleAnomalyReport() AnomalyReport {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	period := DateRange{Start: start, End: start.AddDate(0, 0, 10)}
	return AnomalyReport{
		Considered: 7,
		Anomalies: []Anomaly{
			{
				Resource: "urn:pulumi:dev::app::aws:ec2/instance:Instance::web", Type: "aws:ec2/instance:Instance",
				Period: period, Baseline: 720, Observed: 1080, Deviation: 360, PercentDrift: 50,
				Severity: AnomalySeverityWarning, Method: AnomalyMethodDrift, Currency: "USD",
			},
			{
				Resource: "urn:pulumi:dev::app::aws:rds/instance:Instance::db", Type: "aws:rds/instance:Instance",
				Period: period, Baseline: 500, Observed: 300, Deviation: -200, PercentDrift: -40,
				Severity: AnomalySeverityWarning, Method: AnomalyMethodDrift, Currency: "EUR",
			},
		},
	}
}

func TestRenderAnomaliesAsJSON(t *testing.T) {
	t.Parallel()

	t.Run("carries considered and every anomaly key", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsJSON(&buf, sampleAnomalyReport()))

		var decoded struct {
			Considered int              `json:"considered"`
			Anomalies  []map[string]any `json:"anomalies"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
		assert.Equal(t, 7, decoded.Considered)
		require.Len(t, decoded.Anomalies, 2)
		for _, key := range []string{
			"resource", "type", "period", "baseline", "observed", "deviation",
			"percentDrift", "severity", "method", "currency",
		} {
			assert.Contains(t, decoded.Anomalies[0], key)
		}
		assert.Equal(t, "warning", decoded.Anomalies[0]["severity"])
		assert.Equal(t, "drift", decoded.Anomalies[0]["method"])
	})

	t.Run("empty list is an array not null", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsJSON(&buf, SelectAnomalies(nil)))

		assert.JSONEq(t, `{"considered":0,"anomalies":[]}`, buf.String())
	})

	t.Run("output ends with a newline", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsJSON(&buf, sampleAnomalyReport()))

		assert.True(t, strings.HasSuffix(buf.String(), "\n"))
	})
}

func TestRenderAnomaliesAsNDJSON(t *testing.T) {
	t.Parallel()

	t.Run("one object per line", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsNDJSON(&buf, sampleAnomalyReport()))

		lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
		require.Len(t, lines, 2)
		for _, line := range lines {
			var anomaly Anomaly
			require.NoError(t, json.Unmarshal([]byte(line), &anomaly))
			assert.Equal(t, AnomalyMethodDrift, anomaly.Method)
		}
	})

	t.Run("empty list writes nothing", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsNDJSON(&buf, SelectAnomalies(nil)))

		assert.Empty(t, buf.String())
	})
}

func TestRenderAnomaliesAsTable(t *testing.T) {
	t.Parallel()

	t.Run("lists columns, rows, and the considered count", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsTable(&buf, sampleAnomalyReport()))
		out := buf.String()

		for _, col := range []string{"RESOURCE", "TYPE", "BASELINE", "OBSERVED", "DEVIATION", "DRIFT %", "CURRENCY"} {
			assert.Contains(t, out, col)
		}
		assert.Contains(t, out, "aws:ec2/instance:Instance")
		assert.Contains(t, out, "1080.00")
		assert.Contains(t, out, "+360.00")
		assert.Contains(t, out, "-200.00")
		assert.Contains(t, out, "+50.0%")
		assert.Contains(t, out, "-40.0%")
		assert.Contains(t, out, "2 of 7 resources")
	})

	t.Run("never prints a total across currencies", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsTable(&buf, sampleAnomalyReport()))

		assert.NotContains(t, strings.ToLower(buf.String()), "total")
	})

	t.Run("empty list says so and keeps the considered count", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, RenderAnomaliesAsTable(&buf, AnomalyReport{Considered: 4, Anomalies: []Anomaly{}}))

		assert.Contains(t, buf.String(), "No anomalies")
		assert.Contains(t, buf.String(), "4 resources")
		assert.NotContains(t, buf.String(), "RESOURCE")
	})
}
