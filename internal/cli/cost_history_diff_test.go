package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func TestDiff_JSONOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fromAt := time.Date(2025, time.March, 15, 10, 0, 0, 0, time.UTC)
	toAt := time.Date(2025, time.June, 1, 14, 30, 0, 0, time.UTC)
	seedHistory(
		t,
		dir,
		"dev",
		cliDiffSnap(
			35,
			fromAt,
			1600,
			cliRes("urn:pulumi:dev::app::aws:ec2:Instance::old-worker", "aws:ec2:Instance", 35),
		),
		cliDiffSnap(
			42,
			toAt,
			2000.50,
			cliRes("urn:pulumi:dev::app::aws:rds:Instance::user-analytics-db", "aws:rds:Instance", 350),
		),
	)
	cmd, stdout := preparedHistoryCmd(
		t, NewCostHistoryDiffCmd(), "--stack", "dev", "--from", "v35", "--to", "v42", "--output", "json",
	)
	require.NoError(t, runDiff(cmd, dir))
	var got struct {
		Stack    string  `json:"stack"`
		Delta    float64 `json:"delta"`
		Currency string  `json:"currency"`
		From     struct {
			Version int `json:"version"`
		} `json:"from"`
		To struct {
			Version int `json:"version"`
		} `json:"to"`
		Added []struct {
			URN         string  `json:"urn"`
			MonthlyCost float64 `json:"monthly_cost"`
		} `json:"added"`
		Removed []struct {
			URN string `json:"urn"`
		} `json:"removed"`
		Changed        []any   `json:"changed"`
		UnchangedCount int     `json:"unchanged_count"`
		UnchangedTotal float64 `json:"unchanged_total"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	assert.Equal(t, "dev", got.Stack)
	assert.Equal(t, 35, got.From.Version)
	assert.Equal(t, 42, got.To.Version)
	assert.InDelta(t, 400.50, got.Delta, 0.001)
	assert.Equal(t, "USD", got.Currency)
	require.Len(t, got.Added, 1)
	assert.Equal(t, "urn:pulumi:dev::app::aws:rds:Instance::user-analytics-db", got.Added[0].URN)
	assert.InDelta(t, 350, got.Added[0].MonthlyCost, 0.001)
	require.Len(t, got.Removed, 1)
	assert.Equal(t, "urn:pulumi:dev::app::aws:ec2:Instance::old-worker", got.Removed[0].URN)
	assert.Empty(t, got.Changed)
	assert.Equal(t, 0, got.UnchangedCount)
}

func TestDiff_DateLookup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev",
		cliDiffSnap(1, time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC), 5,
			cliRes("urn:a", "aws:ec2:Instance", 5)),
		cliDiffSnap(2, time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC), 8,
			cliRes("urn:b", "aws:ec2:Instance", 8)),
		cliDiffSnap(3, time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC), 9,
			cliRes("urn:b", "aws:ec2:Instance", 9)),
	)
	cmd, stdout := preparedHistoryCmd(
		t, NewCostHistoryDiffCmd(),
		"--stack", "dev", "--from", "2025-03-15", "--to", "2025-06-01", "--output", "json",
	)
	require.NoError(t, runDiff(cmd, dir))
	var got struct {
		From struct {
			Version int `json:"version"`
		} `json:"from"`
		To struct {
			Version int `json:"version"`
		} `json:"to"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	assert.Equal(t, 2, got.From.Version)
	assert.Equal(t, 3, got.To.Version)

	table, tableOut := preparedHistoryCmd(
		t, NewCostHistoryDiffCmd(), "--stack", "dev", "--from", "v2",
	)
	require.NoError(t, runDiff(table, dir))
	text := tableOut.String()
	assert.Contains(t, text, "Cost Delta:")
	assert.Contains(t, text, "Changed Resources")
	assert.Contains(t, text, "b")
}

func TestDiff_RequiresFromAndRejectsBadOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev", cliDiffSnap(1, time.Now().UTC(), 1, cliRes("urn:a", "aws:ec2:Instance", 1)))
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryDiffCmd(), "--stack", "dev")
	err := runDiff(cmd, dir)
	require.ErrorContains(t, err, "--from is required")

	cmd, _ = preparedHistoryCmd(t, NewCostHistoryDiffCmd(), "--stack", "dev", "--from", "v1", "--output", "yaml")
	err = runDiff(cmd, dir)
	require.ErrorContains(t, err, "unsupported output format")
}

func cliDiffSnap(version int, at time.Time, total float64, resources ...history.CostResource) history.CostSnapshot {
	if resources == nil {
		resources = []history.CostResource{}
	}
	return history.CostSnapshot{
		Timestamp:     at,
		Version:       version,
		TotalMonthly:  total,
		Currency:      "USD",
		ResourceCount: len(resources),
		ByProvider:    map[string]float64{"aws": total},
		ByType:        map[string]float64{},
		Resources:     resources,
	}
}

func cliRes(urn, typ string, monthly float64) history.CostResource {
	return history.CostResource{
		URN: urn, Type: typ, Provider: "aws", MonthlyCost: monthly, SKU: "db.r5.large", Region: "us-east-1",
	}
}
