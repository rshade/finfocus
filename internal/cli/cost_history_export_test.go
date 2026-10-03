package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

func TestExport_FormatsAndFilters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	january := viewSnap(12, time.January, 800.5, map[string]float64{"aws": 800.5})
	january.ResourceCount = 8
	march := viewSnap(23, time.March, 1200, map[string]float64{"aws": 900, "gcp": 300})
	march.ResourceCount = 12
	seedHistory(t, dir, "dev", january, march)

	jsonCmd, jsonOut := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "json")
	require.NoError(t, runExport(jsonCmd, dir))
	var doc history.Export
	require.NoError(t, json.Unmarshal(jsonOut.Bytes(), &doc))
	assert.Equal(t, "dev", doc.Stack)
	assert.Equal(t, "USD", doc.Currency)
	require.Len(t, doc.Snapshots, 2)
	assert.Equal(t, 12, doc.Snapshots[0].Version)
	assert.InDelta(t, 800.5, doc.Snapshots[0].TotalMonthly, 0.001)
	require.NotEmpty(t, doc.Annotations)
	assert.Equal(t, "change", doc.Annotations[0].Message)

	csvCmd, csvOut := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "csv")
	require.NoError(t, runExport(csvCmd, dir))
	csvBody := csvOut.String()
	assert.Contains(t, csvBody, "timestamp,version,total_monthly,aws,gcp,resource_count,message")
	assert.Contains(t, csvBody, "800.50")
	assert.Contains(t, csvBody, "change")

	ndCmd, ndOut := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "ndjson")
	require.NoError(t, runExport(ndCmd, dir))
	lines := strings.Split(strings.TrimSpace(ndOut.String()), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"version":12`)
	assert.NotContains(t, lines[0], "by_provider")

	fromCmd, fromOut := preparedHistoryCmd(
		t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "json", "--from", "2025-03-01",
	)
	require.NoError(t, runExport(fromCmd, dir))
	var fromDoc history.Export
	require.NoError(t, json.Unmarshal(fromOut.Bytes(), &fromDoc))
	require.Len(t, fromDoc.Snapshots, 1)
	assert.Equal(t, 23, fromDoc.Snapshots[0].Version)

	toCmd, toOut := preparedHistoryCmd(
		t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "json", "--to", "2025-02-01",
	)
	require.NoError(t, runExport(toCmd, dir))
	var toDoc history.Export
	require.NoError(t, json.Unmarshal(toOut.Bytes(), &toDoc))
	require.Len(t, toDoc.Snapshots, 1)
	assert.Equal(t, 12, toDoc.Snapshots[0].Version)

	provCmd, provOut := preparedHistoryCmd(
		t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "json", "--provider", "gcp",
	)
	require.NoError(t, runExport(provCmd, dir))
	var provDoc history.Export
	require.NoError(t, json.Unmarshal(provOut.Bytes(), &provDoc))
	require.Len(t, provDoc.Snapshots, 2)
	assert.InDelta(t, 0, provDoc.Snapshots[0].TotalMonthly, 0.001)
	assert.InDelta(t, 300, provDoc.Snapshots[1].TotalMonthly, 0.001)
	assert.Equal(t, map[string]float64{"gcp": 300}, provDoc.Snapshots[1].ByProvider)
}

func TestExport_RejectsFormatBeforeOpening(t *testing.T) {
	t.Parallel()
	missing := t.TempDir()

	bad, _ := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "human")
	err := runExport(bad, missing)
	require.ErrorIs(t, err, errExportFormat)

	unknown, _ := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "yaml")
	err = runExport(unknown, missing)
	require.ErrorContains(t, err, "unsupported export format")

	noStack, _ := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--format", "json")
	err = runExport(noStack, missing)
	require.ErrorIs(t, err, ErrStackRequired)

	dates, _ := preparedHistoryCmd(
		t, NewCostHistoryExportCmd(),
		"--stack", "dev", "--format", "json", "--from", "2025-06-01", "--to", "2025-01-01",
	)
	err = runExport(dates, missing)
	require.ErrorContains(t, err, "invalid date range")

	absent, _ := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "json")
	err = runExport(absent, missing)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no cost history for stack 'dev'")
}

func TestExport_MixedCurrencyKeepsDominant(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := viewSnap(1, time.January, 10, map[string]float64{"aws": 10})
	second := viewSnap(2, time.February, 12, map[string]float64{"aws": 12})
	other := viewSnap(3, time.March, 9, map[string]float64{"aws": 9})
	other.Currency = "EUR"
	seedHistory(t, dir, "dev", first, second, other)

	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--stack", "dev", "--format", "json")
	stderr := &bytes.Buffer{}
	cmd.SetErr(stderr)
	require.NoError(t, runExport(cmd, dir))
	assert.Contains(t, stderr.String(), "Mixed currencies")
	assert.Contains(t, stdout.String(), `"currency": "USD"`)
	assert.NotContains(t, stdout.String(), `"version": 3`)
}

func TestRenderCostOutput_HistorySparkline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	urn := "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"
	first := viewSnap(1, time.January, 4, map[string]float64{"aws": 4})
	first.Resources = []history.CostResource{{
		URN: urn, Type: "aws:ec2/instance:Instance", Provider: "aws", MonthlyCost: 4,
	}}
	second := viewSnap(2, time.June, 24, map[string]float64{"aws": 24})
	second.Resources = []history.CostResource{{
		URN: urn, Type: "aws:ec2/instance:Instance", Provider: "aws", MonthlyCost: 24,
	}}
	seedHistory(t, filepath.Join(home, "history"), "dev", first, second)

	results := &engine.CostResultWithErrors{Results: []engine.CostResult{{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   urn,
		Adapter:      "aws-public",
		Currency:     "USD",
		Monthly:      24,
		Hourly:       0.03,
		TotalCost:    24,
		CostPeriod:   "30 days",
	}}}

	projected := renderCostCommand(t, "projected", []string{"--stack", "dev"})
	require.NoError(t, RenderCostOutput(context.Background(), projected.cmd, "table", results, false))
	projectedOut := projected.stdout.String()
	assert.Contains(t, projectedOut, "Trend")
	assert.Contains(t, projectedOut, "▁")
	assert.Contains(t, projectedOut, "█")

	actual := renderCostCommand(t, "actual", []string{"--stack", "dev"})
	require.NoError(t, RenderActualCostOutput(
		context.Background(), actual.cmd, "table", results, "", false, false, false,
	))
	actualOut := actual.stdout.String()
	assert.Contains(t, actualOut, "Trend")
	assert.Contains(t, actualOut, "▁")
	assert.Contains(t, actualOut, "█")

	missing := renderCostCommand(t, "projected", []string{"--stack", "other"})
	require.NoError(t, RenderCostOutput(context.Background(), missing.cmd, "table", results, false))
	assert.NotContains(t, missing.stdout.String(), "Trend")
}

type renderedCostCommand struct {
	cmd    *cobra.Command
	stdout *bytes.Buffer
}

func renderCostCommand(t *testing.T, name string, args []string) renderedCostCommand {
	t.Helper()
	parent := &cobra.Command{Use: "cost"}
	parent.PersistentFlags().String("stack", "", "")
	child := &cobra.Command{Use: name}
	parent.AddCommand(child)
	found, rest, err := parent.Find(append([]string{name}, args...))
	require.NoError(t, err)
	stdout := &bytes.Buffer{}
	found.SetOut(stdout)
	found.SetErr(&bytes.Buffer{})
	found.SetContext(context.Background())
	require.NoError(t, found.ParseFlags(rest))
	return renderedCostCommand{cmd: found, stdout: stdout}
}
