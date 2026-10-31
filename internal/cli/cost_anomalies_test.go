package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/ax-go"

	"github.com/rshade/finfocus/internal/engine"
)

func anomalyTestRows() []engine.OverviewRow {
	period := engine.DateRange{
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC),
	}
	return []engine.OverviewRow{
		{
			URN: "urn:steady", Type: "aws:s3/bucket:Bucket", Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{MTDCost: 10, Currency: "USD", Period: period},
		},
		{
			URN: "urn:drifting", Type: "aws:ec2/instance:Instance", Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{MTDCost: 360, Currency: "USD", Period: period},
			CostDrift: &engine.CostDriftData{
				ExtrapolatedMonthly: 1080, Projected: 720, Delta: 360, PercentDrift: 50, IsWarning: true,
			},
		},
		{
			URN: "urn:failed", Type: "aws:rds/instance:Instance", Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{MTDCost: 5, Currency: "USD", Period: period},
			CostDrift: &engine.CostDriftData{
				ExtrapolatedMonthly: 900, Projected: 100, Delta: 800, PercentDrift: 800, IsWarning: true,
			},
			Error: &engine.OverviewRowError{URN: "urn:failed", ErrorType: engine.ErrorTypeUnknown, Message: "boom"},
		},
	}
}

func anomaliesTestDeps(loader OverviewPipelineLoader) anomaliesDeps {
	return anomaliesDeps{
		loader:      loader,
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	}
}

func runAnomaliesTest(t *testing.T, deps anomaliesDeps, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newCostAnomaliesCmd(deps)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetContext(context.Background())
	require.NoError(t, cmd.ParseFlags(args))
	err := cmd.RunE(cmd, cmd.Flags().Args())
	return buf.String(), err
}

func TestCostAnomalies_TakesNoPositionalArguments(t *testing.T) {
	t.Parallel()
	cmd := NewCostAnomaliesCmd()

	require.Error(t, cmd.Args(cmd, []string{"extra"}))
	require.NoError(t, cmd.Args(cmd, nil))
}

//nolint:paralleltest // NewRootCmd sets the resolved project dir, which is process-wide state.
func TestCostAnomalies_MountedUnderCost(t *testing.T) {
	root := NewRootCmd("test")

	found, _, err := root.Find([]string{"cost", "anomalies"})

	require.NoError(t, err)
	assert.Equal(t, "anomalies", found.Name())
	assert.Equal(t, "cost", found.Parent().Name())
}

func TestCostAnomalies_JSONMatchesSharedFunction(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	rows := anomalyTestRows()

	out, err := runAnomaliesTest(t, anomaliesTestDeps(fakePipelineLoader(rows)), "--output", "json")
	require.NoError(t, err)

	var want bytes.Buffer
	require.NoError(t, engine.RenderAnomaliesAsJSON(&want, engine.SelectAnomalies(rows)))
	assert.JSONEq(t, want.String(), out)

	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, 3, decoded.Considered)
	require.Len(t, decoded.Anomalies, 1, "steady row has no drift and the failed row is an error")
	assert.Equal(t, "urn:drifting", decoded.Anomalies[0].Resource)
}

func TestCostAnomalies_EmptyStackExitsZeroWithConsidered(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	out, err := runAnomaliesTest(t, anomaliesTestDeps(fakePipelineLoader(nil)), "--output", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `{"considered":0,"anomalies":[]}`, out)
}

func TestCostAnomalies_NDJSONOneLinePerAnomaly(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	out, err := runAnomaliesTest(t, anomaliesTestDeps(fakePipelineLoader(anomalyTestRows())), "--output", "ndjson")

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], `"resource":"urn:drifting"`)
}

func TestCostAnomalies_DefaultsToPlainTable(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	out, err := runAnomaliesTest(t, anomaliesTestDeps(fakePipelineLoader(anomalyTestRows())))

	require.NoError(t, err)
	assert.Contains(t, out, "RESOURCE")
	assert.Contains(t, out, "urn:drifting")
	assert.Contains(t, out, "1 of 3 resources")
	assert.NotContains(t, out, "Overview:", "the overview pre-flight summary must not leak into stdout")
}

func TestCostAnomalies_UnknownOutputFailsBeforeLoad(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	var loads atomic.Int32
	loader := func(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
		loads.Add(1)
		return fakePipelineLoader(nil)(ctx, p)
	}

	_, err := runAnomaliesTest(t, anomaliesTestDeps(loader), "--output", "yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
	assert.Zero(t, loads.Load(), "state must not be loaded for an invalid --output")
	assertValidationExit(t, err)
}

func TestCostAnomalies_BadDateIsValidationError(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	_, err := runAnomaliesTest(t, anomaliesTestDeps(fakePipelineLoader(nil)), "--from", "not-a-date")

	require.Error(t, err)
	assertValidationExit(t, err)
}

func anomalyFilterRows() []engine.OverviewRow {
	rows := anomalyTestRows()
	rows[0].CostDrift = &engine.CostDriftData{
		ExtrapolatedMonthly: 50, Projected: 20, Delta: 30, PercentDrift: 150, IsWarning: true,
	}
	return rows
}

func TestCostAnomalies_FilterNarrowsRows(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	out, err := runAnomaliesTest(t, anomaliesTestDeps(overviewFilteringLoader(anomalyFilterRows())),
		"--output", "json", "--filter", "type=aws:ec2/instance:Instance")
	require.NoError(t, err)

	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, 1, decoded.Considered)
	require.Len(t, decoded.Anomalies, 1)
	assert.Equal(t, "urn:drifting", decoded.Anomalies[0].Resource)
}

func TestCostAnomalies_UnknownFilterKeyFailsBeforeLoad(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	var loads atomic.Int32
	loader := func(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
		loads.Add(1)
		return fakePipelineLoader(nil)(ctx, p)
	}

	_, err := runAnomaliesTest(t, anomaliesTestDeps(loader), "--filter", "tag:env=prod")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown filter key")
	assert.Zero(t, loads.Load())
	assertValidationExit(t, err)
}

func TestCostAnomalies_AcceptsStateWithPreviewLikeOverview(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	plan := filepath.Join(dir, "plan.json")
	require.NoError(t, os.WriteFile(state, []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(plan, []byte("{}"), 0o600))

	out, err := runAnomaliesTest(t, anomaliesTestDeps(fakePipelineLoader(anomalyTestRows())),
		"--pulumi-state", state, "--pulumi-json", plan, "--output", "json")

	require.NoError(t, err, "overview and --web accept a state and a preview together")
	assert.Contains(t, out, `"considered": 3`)
}

func TestCostAnomalies_FormatJSONSelectsJSON(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	var buf bytes.Buffer
	cmd := newCostAnomaliesCmd(anomaliesTestDeps(fakePipelineLoader(anomalyTestRows())))
	cmd.PersistentFlags().String(axFormatFlag, "", "")
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetContext(context.Background())
	require.NoError(t, cmd.ParseFlags([]string{"--" + axFormatFlag, "json"}))

	require.NoError(t, cmd.RunE(cmd, nil))

	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, 3, decoded.Considered)
}

// overviewFilteringLoader applies the overview's own filter step to the rows,
// as plainOverviewLoader does after merging resources.
func overviewFilteringLoader(rows []engine.OverviewRow) OverviewPipelineLoader {
	return func(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
		filtered, err := validateAndApplyOverviewFilters(rows, p.cfg.params.filter)
		if err != nil {
			return nil, err
		}
		return fakePipelineLoader(filtered)(ctx, p)
	}
}

func assertValidationExit(t *testing.T, err error) {
	t.Helper()
	assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
}
