package history

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiffDocumentJSON(t *testing.T) {
	t.Parallel()

	diff := CostDiff{
		From:     CostDiffPoint{Version: 1, Timestamp: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC), TotalMonthly: 10},
		To:       CostDiffPoint{Version: 2, TotalMonthly: 30},
		Delta:    20,
		Currency: "USD",
		Added: []CostDiffItem{{
			URN: "urn:new", Type: "aws:rds:Instance", MonthlyCost: 30, Delta: 30, SKU: "db.t3", Region: "us-east-1",
		}},
		UnchangedCount: 1,
		UnchangedTotal: 5,
	}

	data, err := json.Marshal(NewDiffDocument("dev", diff))

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"stack": "dev",
		"from": {"version": 1, "timestamp": "2025-03-15T00:00:00Z", "total_monthly": 10},
		"to": {"version": 2, "timestamp": "", "total_monthly": 30},
		"delta": 20,
		"currency": "USD",
		"added": [{"urn": "urn:new", "type": "aws:rds:Instance", "monthly_cost": 30, "delta": 30,
			"sku": "db.t3", "region": "us-east-1"}],
		"removed": [],
		"changed": [],
		"unchanged_count": 1,
		"unchanged_total": 5
	}`, string(data))
}

func TestViewDocumentJSON(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(ViewDocument{Stack: "dev"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"stack": "dev", "snapshots": null, "annotations": null}`, string(data))
}
