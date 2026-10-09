package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOverviewRowResultJSONShape pins the settled serializable form of an
// overview row (T014a): field names and values the web payloads and the CLI
// JSON output share.
func TestOverviewRowResultJSONShape(t *testing.T) {
	t.Parallel()

	mtd := 12.5
	monthly := 40.0
	delta := 27.5
	row := ComputeOverviewRowResult(OverviewRow{
		URN:    "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:   "aws:ec2/instance:Instance",
		Status: StatusActive,
		ActualCost: &ActualCostData{
			MTDCost:  mtd,
			Currency: "USD",
			Period: DateRange{
				Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC),
			},
		},
		ProjectedCost: &ProjectedCostData{MonthlyCost: monthly, Currency: "USD"},
		ComputedDelta: &delta,
		Recommendations: []Recommendation{{
			ResourceID:       "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:             "rightsizing",
			Description:      "downsize instance",
			EstimatedSavings: 10,
			Currency:         "USD",
		}},
	})

	data, err := json.Marshal(row)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	// Identity and cost fields use the settled camelCase names.
	assert.Equal(t, "urn:pulumi:dev::app::aws:ec2/instance:Instance::web", decoded["urn"])
	assert.Equal(t, "aws:ec2/instance:Instance", decoded["type"])
	assert.Equal(t, "web", decoded["displayName"])
	assert.Equal(t, "active", decoded["status"])
	assert.InDelta(t, mtd, decoded["actualMtd"], 1e-9)
	assert.InDelta(t, monthly, decoded["projected"], 1e-9)
	assert.InDelta(t, delta, decoded["delta"], 1e-9)

	// Display strings are server-computed (FR-011b).
	assert.Equal(t, FormatOverviewCurrency(mtd), decoded["actualDisplay"])
	assert.Equal(t, FormatOverviewCurrency(monthly), decoded["projectedDisplay"])
	assert.Equal(t, FormatOverviewDelta(delta), decoded["deltaDisplay"])
	assert.Equal(t, "1", decoded["recsDisplay"])
	assert.InDelta(t, 1, decoded["activeRecs"], 1e-9)

	// The source row rides along under "source" with the full OverviewRow schema.
	source, ok := decoded["source"].(map[string]any)
	require.True(t, ok, "source must be an object")
	assert.Equal(t, "urn:pulumi:dev::app::aws:ec2/instance:Instance::web", source["urn"])
	assert.InDelta(t, monthly, source["projectedCost"].(map[string]any)["monthlyCost"], 1e-9)

	// BaselineProjectedCost is internal to delta math and never serialized.
	assert.NotContains(t, decoded, "baselineProjectedCost")
	assert.NotContains(t, decoded, "BaselineProjectedCost")
}

// TestOverviewTotalsJSONShape pins the settled totals form (T014a).
func TestOverviewTotalsJSONShape(t *testing.T) {
	t.Parallel()

	totals := OverviewTotals{
		TotalActual:    10,
		TotalProjected: 25,
		TotalDelta:     15,
		TotalSavings:   5,
		Currency:       "USD",
		Errors:         []OverviewRowError{{URN: "urn:x", ErrorType: ErrorTypeUnknown, Message: "boom"}},
	}
	data, err := json.Marshal(totals)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.InDelta(t, 10.0, decoded["totalActual"], 1e-9)
	assert.InDelta(t, 25.0, decoded["totalProjected"], 1e-9)
	assert.InDelta(t, 15.0, decoded["totalDelta"], 1e-9)
	assert.InDelta(t, 5.0, decoded["totalSavings"], 1e-9)
	assert.Equal(t, "USD", decoded["currency"])
	assert.Len(t, decoded["errors"], 1)
}

// TestRenderOverviewAsJSONMatchesRowResultShape proves the settled row form
// and the CLI JSON output are two views of the same values: every row in the
// RenderOverviewAsJSON resources array equals the corresponding
// OverviewRowResult.Source.
func TestRenderOverviewAsJSONMatchesRowResultShape(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{{
		URN:           "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs",
		Type:          "aws:s3/bucket:Bucket",
		Status:        StatusActive,
		ProjectedCost: &ProjectedCostData{MonthlyCost: 3.5, Currency: "USD"},
	}}
	result := ComputeOverviewResult(rows, time.Now().Day())

	var buf bytes.Buffer
	stackCtx := StackContext{
		StackName:      "dev",
		TimeWindow:     DateRange{Start: time.Now().Add(-24 * time.Hour), End: time.Now()},
		TotalResources: 1,
	}
	require.NoError(t, RenderOverviewAsJSON(context.Background(), &buf, result, stackCtx, nil))

	var output OverviewJSONOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))
	require.Len(t, output.Resources, 1)
	assert.Equal(t, result.Rows[0].Source, output.Resources[0])
	assert.InDelta(t, result.Summary.TotalProjected, output.Summary.ProjectedMonthly, 1e-9)
}
