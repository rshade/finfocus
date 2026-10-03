package history

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildExport_JSONAndProvider(t *testing.T) {
	t.Parallel()

	when := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	snapshots := []CostSnapshot{
		{
			Timestamp:     time.Date(2025, 3, 1, 9, 15, 0, 0, time.UTC),
			Version:       23,
			TotalMonthly:  1200,
			Currency:      "USD",
			ResourceCount: 12,
			ByProvider:    map[string]float64{"gcp": 300, "aws": 900},
		},
		{
			Timestamp:     time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			Version:       12,
			TotalMonthly:  800.5,
			Currency:      "USD",
			ResourceCount: 8,
			ByProvider:    map[string]float64{"aws": 800.5},
		},
	}
	annotations := []CostAnnotation{
		{Version: 12, Message: "Initial deployment"},
		{Version: 23, Message: "Added GCP resources"},
		{Version: 99, Message: "dropped"},
	}

	doc := BuildExport("dev", "USD", "", snapshots, annotations, when)
	require.Len(t, doc.Snapshots, 2)
	assert.Equal(t, 12, doc.Snapshots[0].Version)
	assert.Equal(t, 23, doc.Snapshots[1].Version)
	assert.Equal(t, when, doc.ExportedAt)
	require.Len(t, doc.Annotations, 2)
	assert.Equal(t, "Initial deployment", doc.Annotations[0].Message)
	assert.Equal(t, doc.Snapshots[0].Timestamp, doc.Annotations[0].Timestamp)
	assert.NotContains(t, annotationMessages(doc.Annotations), "dropped")

	var buf bytes.Buffer
	require.NoError(t, WriteExport(&buf, "json", doc))
	var decoded Export
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "dev", decoded.Stack)
	assert.Equal(t, "USD", decoded.Currency)
	require.Len(t, decoded.Snapshots, 2)
	assert.InDelta(t, 800.5, decoded.Snapshots[0].TotalMonthly, 0.001)
	assert.InDelta(t, 800.5, decoded.Snapshots[0].ByProvider["aws"], 0.001)
	assert.Empty(t, decoded.Snapshots[0].ByProvider["gcp"])

	filtered := BuildExport("dev", "USD", "gcp", snapshots, annotations, when)
	assert.InDelta(t, 0, filtered.Snapshots[0].TotalMonthly, 0.001)
	assert.InDelta(t, 300, filtered.Snapshots[1].TotalMonthly, 0.001)
	assert.Equal(t, map[string]float64{"gcp": 0}, filtered.Snapshots[0].ByProvider)
	assert.Equal(t, map[string]float64{"gcp": 300}, filtered.Snapshots[1].ByProvider)
}

func TestWriteExport_CSVAndNDJSON(t *testing.T) {
	t.Parallel()

	doc := BuildExport("dev", "USD", "", []CostSnapshot{
		{
			Timestamp:     time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			Version:       12,
			TotalMonthly:  800.5,
			ResourceCount: 8,
			ByProvider:    map[string]float64{"aws": 800.5},
		},
		{
			Timestamp:     time.Date(2025, 3, 1, 9, 15, 0, 0, time.UTC),
			Version:       23,
			TotalMonthly:  1200,
			ResourceCount: 12,
			ByProvider:    map[string]float64{"aws": 900, "gcp": 300},
		},
	}, []CostAnnotation{
		{Version: 12, Message: "Initial deployment"},
		{Version: 23, Message: "Added GCP resources"},
		{Version: 23, Message: "second note"},
	}, time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC))

	var csvBuf bytes.Buffer
	require.NoError(t, WriteExport(&csvBuf, "csv", doc))
	reader := csv.NewReader(&csvBuf)
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []string{
		"timestamp", "version", "total_monthly", "aws", "gcp", "resource_count", "message",
	}, records[0])
	assert.Equal(t, "800.50", records[1][2])
	assert.Equal(t, "800.50", records[1][3])
	assert.Equal(t, "0.00", records[1][4])
	assert.Equal(t, "8", records[1][5])
	assert.Equal(t, "Initial deployment", records[1][6])
	assert.Equal(t, "Added GCP resources; second note", records[2][6])

	var lines bytes.Buffer
	require.NoError(t, WriteExport(&lines, "ndjson", doc))
	body := strings.TrimSpace(lines.String())
	parts := strings.Split(body, "\n")
	require.Len(t, parts, 2)
	var row ExportSnapshot
	require.NoError(t, json.Unmarshal([]byte(parts[0]), &row))
	assert.Equal(t, 12, row.Version)
	assert.InDelta(t, 800.5, row.TotalMonthly, 0.001)
	assert.NotContains(t, parts[0], "by_provider")
	assert.NotContains(t, parts[1], "annotations")

	err = WriteExport(&bytes.Buffer{}, "human", doc)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported export format")
}

func annotationMessages(annotations []ExportAnnotation) string {
	parts := make([]string, len(annotations))
	for i, annotation := range annotations {
		parts[i] = annotation.Message
	}
	return strings.Join(parts, "\n")
}
