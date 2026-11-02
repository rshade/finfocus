package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestCollectDailyPoints_IgnoresEvenSplit(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(72 * time.Hour)
	points, report := collectDailyPoints(&engine.CostResultWithErrors{
		Results: []engine.CostResult{{
			ResourceID: "urn:odd",
			Currency:   "USD",
			StartDate:  start,
			EndDate:    end,
			TotalCost:  100,
			DailyCosts: []float64{50, 50},
			DailyObservations: []engine.DailyObservation{{
				Timestamp: start, Cost: 4, Currency: "USD",
			}},
		}},
	}, nil, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))

	require.Len(t, points, 1)
	assert.InDelta(t, 4, points[0].Amount, 0)
	assert.Equal(t, start, points[0].Day)
	assert.Empty(t, report.Errors)
	assert.Equal(t, 0, report.SkippedResources)
}

func TestCollectDaily_DefaultWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 31, 15, 4, 0, 0, time.UTC)
	var gotFrom, gotTo time.Time
	cmd, _, _ := dailyCmd(t)
	err := runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return now },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{ID: "urn:a"}}, nil
		},
		price: func(
			_ context.Context, _ *cobra.Command, _ []engine.ResourceDescriptor, from, to time.Time,
		) (*engine.CostResultWithErrors, error) {
			gotFrom, gotTo = from, to
			return &engine.CostResultWithErrors{}, nil
		},
		cfg:     config.New(),
		dir:     t.TempDir(),
		project: "demo",
		stack:   "dev",
	})
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), gotFrom)
	assert.Equal(t, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), gotTo)
	assert.Equal(t, 30*24*time.Hour, gotTo.Sub(gotFrom))
}

func TestCollectDaily_StoresReplacesAndKeepsTimeline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedTimeline(t, dir)
	seedDaily(t, dir, history.DailyPoint{
		ResourceID: "urn:bad",
		Day:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Currency:   "USD",
		Amount:     7,
		WrittenAt:  time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
	})

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	written := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	calls := 0
	deps := dailyCollectDeps{
		now: func() time.Time { return written },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{ID: "urn:ok"}}, nil
		},
		price:   scriptedDailyPrice(&calls, start, end),
		cfg:     config.New(),
		dir:     dir,
		project: "demo",
		stack:   "dev",
	}

	cmd, stdout, stderr := dailyCmd(t, "--from", "2026-10-01", "--to", "2026-10-03", "--output", "json")
	err := runDailyCollect(cmd, deps)
	require.ErrorContains(t, err, "1 resources failed")
	assert.Contains(t, stderr.String(), "urn:skip has no daily costs")
	assert.Contains(t, stderr.String(), "plugin down")

	var report dailyCollectReport
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.Equal(t, 2, report.StoredResources)
	assert.Equal(t, 6, report.StoredPoints)
	assert.Equal(t, 1, report.SkippedResources)
	require.Len(t, report.Errors, 1)
	assert.Equal(t, "urn:bad", report.Errors[0].Resource)

	cmd, stdout, _ = dailyCmd(t, "--from", "2026-10-01", "--to", "2026-10-03")
	require.NoError(t, runDailyCollect(cmd, deps))
	assert.Contains(t, stdout.String(), "Stored 1 resources (1 points).")

	points := readDaily(t, dir)
	assert.InDelta(t, 1, dailyAmount(points, "urn:ok", start, "USD"), 0)
	assert.InDelta(t, 5, dailyAmount(points, "urn:ok", start.AddDate(0, 0, 1), "USD"), 0)
	assert.InDelta(t, 8, dailyAmount(points, "urn:ok", start, "EUR"), 0)
	require.True(t, dailyHas(points, "urn:zero", start))
	assert.InDelta(t, 0, dailyAmount(points, "urn:zero", start, "USD"), 0)
	assert.InDelta(t, 7, dailyAmount(points, "urn:bad", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "USD"), 0)
	assert.False(t, dailyHas(points, "urn:skip", start))

	snaps := readTimeline(t, dir)
	require.Len(t, snaps, 1)
	assert.InDelta(t, 12, snaps[0].TotalMonthly, 0)
	assert.Equal(t, 3, snaps[0].Version)
}

func TestCollectDaily_PrunesByAgeOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldDay := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	seedDaily(t, dir, history.DailyPoint{
		ResourceID: "urn:old", Day: oldDay, Currency: "USD", Amount: 3,
		WrittenAt: oldDay,
	})
	seedDaily(t, dir, history.DailyPoint{
		ResourceID: "urn:fail", Day: oldDay, Currency: "USD", Amount: 8,
		WrittenAt: oldDay,
	})
	cfg := config.New()
	cfg.Cost.History.Retention.AutoPrune = true
	cfg.Cost.History.Retention.MaxSnapshots = 1
	cfg.Cost.History.Retention.MaxAgeDays = 1
	now := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	kept := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	cmd, stdout, _ := dailyCmd(t, "--from", "2026-10-02", "--to", "2026-10-03")
	err := runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return now },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{ID: "urn:new"}}, nil
		},
		price: func(
			_ context.Context, _ *cobra.Command, _ []engine.ResourceDescriptor, _, _ time.Time,
		) (*engine.CostResultWithErrors, error) {
			return &engine.CostResultWithErrors{
				Results: []engine.CostResult{{
					ResourceID: "urn:new", Currency: "USD",
					StartDate: kept, EndDate: kept.AddDate(0, 0, 1),
					DailyObservations: []engine.DailyObservation{{
						Timestamp: kept, Cost: 4, Currency: "USD",
					}},
				}},
				Errors: []engine.ErrorDetail{{
					ResourceID: "urn:fail", Error: errors.New("plugin down"),
				}},
			}, nil
		},
		cfg: cfg, dir: dir, project: "demo", stack: "dev",
	})
	require.ErrorContains(t, err, "1 resources failed")
	assert.Contains(t, stdout.String(), "Pruned 1 daily points")
	points := readDaily(t, dir)
	require.Len(t, points, 2)
	assert.InDelta(t, 4, dailyAmount(points, "urn:new", kept, "USD"), 0)
	assert.InDelta(t, 8, dailyAmount(points, "urn:fail", oldDay, "USD"), 0)
	assert.False(t, dailyHas(points, "urn:old", oldDay))
}

func TestCollectDaily_RequiresFromWhenToIsSet(t *testing.T) {
	t.Parallel()

	cmd, _, _ := dailyCmd(t, "--to", "2026-10-03")
	err := runDailyCollect(cmd, dailyCollectDeps{
		now:     func() time.Time { return time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC) },
		dir:     t.TempDir(),
		project: "demo",
		stack:   "dev",
		cfg:     config.New(),
	})
	require.ErrorContains(t, err, "--from is required")
}

func scriptedDailyPrice(calls *int, start, end time.Time) dailyPricer {
	return func(
		_ context.Context, _ *cobra.Command, _ []engine.ResourceDescriptor, _, _ time.Time,
	) (*engine.CostResultWithErrors, error) {
		*calls++
		if *calls > 1 {
			day := start.AddDate(0, 0, 1)
			return &engine.CostResultWithErrors{Results: []engine.CostResult{{
				ResourceID: "urn:ok", Currency: "USD",
				StartDate: day, EndDate: end,
				DailyObservations: []engine.DailyObservation{{
					Timestamp: day, Cost: 5, Currency: "USD",
				}},
			}}}, nil
		}
		return &engine.CostResultWithErrors{
			Results: []engine.CostResult{
				{
					ResourceID: "urn:ok", Currency: "USD", StartDate: start, EndDate: end,
					TotalCost: 99,
					DailyObservations: []engine.DailyObservation{
						{Timestamp: start, Cost: 1, Currency: "USD"},
						{Timestamp: start.AddDate(0, 0, 1), Cost: 2, Currency: "USD"},
					},
				},
				{
					ResourceID: "urn:skip", Currency: "USD", StartDate: start, EndDate: end,
					TotalCost: 50, DailyCosts: []float64{25, 25},
				},
				{
					ResourceID: "urn:zero", Currency: "USD", StartDate: start, EndDate: end,
					DailyObservations: []engine.DailyObservation{
						{Timestamp: start, Cost: 0, Currency: "USD"},
						{Timestamp: start.AddDate(0, 0, 1), Cost: 4, Currency: "USD"},
					},
				},
				{
					ResourceID: "urn:ok", Currency: "EUR", StartDate: start, EndDate: end,
					DailyObservations: []engine.DailyObservation{
						{Timestamp: start, Cost: 8, Currency: "EUR"},
						{Timestamp: start.AddDate(0, 0, 1), Cost: 9, Currency: "EUR"},
					},
				},
			},
			Errors: []engine.ErrorDetail{{ResourceID: "urn:bad", Error: errors.New("plugin down")}},
		}, nil
	}
}

func dailyCmd(t *testing.T, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryCollectDailyCmd(), args...)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd, stdout, stderr
}

func seedTimeline(t *testing.T, dir string) {
	t.Helper()
	name, err := history.CostFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenCostDBFor(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	require.NoError(t, db.Put(history.CostSnapshot{
		Timestamp:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Version:      3,
		Currency:     "USD",
		TotalMonthly: 12,
	}, history.CostAnnotation{Version: 3}))
	require.NoError(t, db.Close())
}

func seedDaily(t *testing.T, dir string, point history.DailyPoint) {
	t.Helper()
	name, err := history.DailyFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenDailyDB(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	require.NoError(t, db.Replace([]history.DailyPoint{point}))
	require.NoError(t, db.Close())
}

func readDaily(t *testing.T, dir string) []history.DailyPoint {
	t.Helper()
	name, err := history.DailyFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenDailyDB(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	points, err := db.Points()
	require.NoError(t, err)
	return points
}

func readTimeline(t *testing.T, dir string) []history.CostSnapshot {
	t.Helper()
	name, err := history.CostFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenCostDBFor(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	return snaps
}

func dailyAmount(points []history.DailyPoint, id string, day time.Time, currency string) float64 {
	for _, point := range points {
		if point.ResourceID == id && point.Day.Equal(day) && point.Currency == currency {
			return point.Amount
		}
	}
	return 0
}

func dailyHas(points []history.DailyPoint, id string, day time.Time) bool {
	for _, point := range points {
		if point.ResourceID == id && point.Day.Equal(day) && point.Currency == "USD" {
			return true
		}
	}
	return false
}

func TestDailyActualRequest_StoresWhatThePluginReported(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	resources := []engine.ResourceDescriptor{{ID: "urn:a"}}
	request := dailyActualRequest(resources, from, to)

	assert.Equal(t, resources, request.Resources)
	assert.Equal(t, from, request.From)
	assert.Equal(t, to, request.To)
	assert.True(t, request.RetainDailyObservations)
	assert.True(t, request.SkipStateEstimate, "a state estimate would replace a reported $0 day and fill gaps")
}

func TestCollectDailyPoints_CountsResourcesTheEngineOmitted(t *testing.T) {
	t.Parallel()

	resources := []engine.ResourceDescriptor{
		{ID: "urn:priced", Type: "aws:ec2/instance:Instance"},
		{ID: "urn:declined", Type: "aws:rds/instance:Instance"},
		{ID: "urn:stack", Type: "pulumi:pulumi:Stack"},
	}
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	points, report := collectDailyPoints(&engine.CostResultWithErrors{
		Results: []engine.CostResult{{
			ResourceID:        "urn:priced",
			DailyObservations: []engine.DailyObservation{{Timestamp: day, Cost: 3, Currency: "USD"}},
		}},
	}, resources, day.Add(48*time.Hour))

	require.Len(t, points, 1)
	assert.Equal(t, 1, report.SkippedResources, "a declined resource is reported, internal Pulumi types are not")
	assert.Equal(t, []string{"urn:declined"}, report.skippedIDs)
	assert.Empty(t, report.Errors)
}

// dailyTerraformPlugin resolves Terraform types to Pulumi tokens but prices
// only those tokens, as a plugin that advertises resolution does.
type dailyTerraformPlugin struct {
	proto.CostSourceClient

	mu    sync.Mutex
	types []string
	day   time.Time
}

func (*dailyTerraformPlugin) ResolveResourceTypes(
	_ context.Context, _ *pbc.ResolveResourceTypesRequest, _ ...grpc.CallOption,
) (*pbc.ResolveResourceTypesResponse, error) {
	return &pbc.ResolveResourceTypesResponse{Mappings: map[string]*pbc.ResourceTypeMapping{
		"aws_instance": {PulumiToken: "aws:ec2/instance:Instance", Supported: true},
	}}, nil
}

func (*dailyTerraformPlugin) Supports(
	_ context.Context, _ *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (p *dailyTerraformPlugin) GetActualCost(
	_ context.Context, in *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	p.mu.Lock()
	p.types = append(p.types, in.ResourceType)
	p.mu.Unlock()
	if in.ResourceType != "aws:ec2/instance:Instance" {
		return nil, errors.New("unsupported resource type " + in.ResourceType)
	}
	return &proto.GetActualCostResponse{Results: []*proto.ActualCostResult{{
		Currency:     "USD",
		TotalCost:    3,
		Observations: []proto.ActualCostPoint{{Timestamp: p.day, Cost: 3, Currency: "USD"}},
	}}}, nil
}

func TestPriceDailyWithEngine_ResolvesTerraformTypes(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	plugin := &dailyTerraformPlugin{day: day}
	clients := []*pluginhost.Client{{
		Name: "aws-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "aws-plugin",
			SupportedProviders: []string{"aws"},
			Capabilities:       []string{"actual_costs", capabilityResolveResourceTypes},
		},
		API:   plugin,
		Close: func() error { return nil },
	}}
	resources := []engine.ResourceDescriptor{{
		ID:         "aws_instance.web",
		Type:       "aws_instance",
		Provider:   "aws",
		Properties: map[string]any{"instanceType": "t3.micro"},
	}}

	priced, err := priceDailyWithEngine(
		t.Context(), engine.New(clients, nil), clients, nil, resources, day, day.Add(24*time.Hour),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"aws:ec2/instance:Instance"}, plugin.types, "the plugin prices the resolved token")
	require.Len(t, priced.Results, 1)
	require.Len(t, priced.Results[0].DailyObservations, 1)
}

func TestCollectDailyPoints_AValidationFailureRejectsTheWholeResource(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	nan := engine.CostResult{
		ResourceID:        "urn:mixed",
		DailyObservations: []engine.DailyObservation{{Timestamp: day, Cost: math.NaN(), Currency: "EUR"}},
	}
	valid := engine.CostResult{
		ResourceID:        "urn:mixed",
		DailyObservations: []engine.DailyObservation{{Timestamp: day, Cost: 2, Currency: "USD"}},
	}
	for name, results := range map[string][]engine.CostResult{
		"invalid then valid": {nan, valid},
		"valid then invalid": {valid, nan},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			points, report := collectDailyPoints(
				&engine.CostResultWithErrors{Results: results}, nil, day.Add(48*time.Hour),
			)
			assert.Empty(t, points, "no partial data for a resource that failed validation")
			require.Len(t, report.Errors, 1)
			assert.Equal(t, "urn:mixed", report.Errors[0].Resource)
		})
	}
}

func TestCollectDaily_DryRunWritesAndPrunesNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldDay := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	seedDaily(t, dir, history.DailyPoint{
		ResourceID: "urn:old", Day: oldDay, Currency: "USD", Amount: 3, WrittenAt: oldDay,
	})
	cfg := config.New()
	cfg.Cost.History.Retention.AutoPrune = true
	cfg.Cost.History.Retention.MaxAgeDays = 1
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	cmd, stdout, _ := dailyCmd(t, "--dry-run", "--from", "2026-10-02", "--to", "2026-10-03", "--output", "json")
	require.NoError(t, runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return day.Add(32 * time.Hour) },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{ID: "urn:new"}}, nil
		},
		price: func(
			context.Context, *cobra.Command, []engine.ResourceDescriptor, time.Time, time.Time,
		) (*engine.CostResultWithErrors, error) {
			return &engine.CostResultWithErrors{Results: []engine.CostResult{{
				ResourceID:        "urn:new",
				DailyObservations: []engine.DailyObservation{{Timestamp: day, Cost: 4, Currency: "USD"}},
			}}}, nil
		},
		cfg: cfg, dir: dir, project: "demo", stack: "dev",
	}))

	points := readDaily(t, dir)
	require.Len(t, points, 1, "a rehearsal stores no new point")
	assert.True(t, dailyHas(points, "urn:old", oldDay), "a rehearsal prunes nothing")
	var report dailyCollectReport
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.True(t, report.DryRun)
	assert.Equal(t, 1, report.StoredPoints, "the report says what a real run would store")
	assert.Zero(t, report.PrunedPoints)
}

// Config, the working directory, and the history directory are process-wide.
func TestDailyTarget_ExplicitPulumiInputIgnoresTerraformDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# fixture\n"), 0o600))
	t.Chdir(dir)
	cmd, _, _ := dailyCmd(t, "--pulumi-state", "state.json", "--stack", "org/app/dev")

	target, err := dailyTarget(cmd, dailyCollectDeps{cfg: config.New()}, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "app", target.Project)
	assert.Equal(t, "dev", target.Stack, "explicit Pulumi input is a Pulumi stack, not the Terraform directory")
}

func TestCollectDailyPoints_SumsResultsThatShareAURN(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	result := func(cost float64) engine.CostResult {
		return engine.CostResult{
			ResourceID:        "urn:replaced",
			DailyObservations: []engine.DailyObservation{{Timestamp: day, Cost: cost, Currency: "USD"}},
		}
	}
	points, report := collectDailyPoints(
		&engine.CostResultWithErrors{Results: []engine.CostResult{result(2), result(3)}}, nil, day.Add(48*time.Hour),
	)

	require.Len(t, points, 1, "one key is one point, or the second write silently replaces the first")
	assert.InDelta(t, 5, points[0].Amount, 0)
	assert.Equal(t, 1, report.StoredPoints)
}

func TestCollectDaily_RejectsAnInputFromAnotherStack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	priced := false
	cmd, _, _ := dailyCmd(t, "--from", "2026-10-01", "--to", "2026-10-02")
	err := runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC) },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{
				ID: "urn:pulumi:prod::demo::aws:ec2/instance:Instance::web", Type: "aws:ec2/instance:Instance",
			}}, nil
		},
		price: func(
			context.Context, *cobra.Command, []engine.ResourceDescriptor, time.Time, time.Time,
		) (*engine.CostResultWithErrors, error) {
			priced = true
			return &engine.CostResultWithErrors{}, nil
		},
		cfg: config.New(), dir: dir, project: "demo", stack: "dev",
	})

	require.ErrorContains(t, err, "prod")
	assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
	assert.False(t, priced, "a mismatched input is rejected before any pricing")
	assert.Empty(t, readDailyIfPresent(t, dir), "nothing is written to the other stack's database")
}

func TestLoadDailyResources_ClassifiesAMissingInputFileAsValidation(t *testing.T) {
	t.Parallel()

	cmd, _, _ := dailyCmd(t, "--from", "2026-10-01", "--pulumi-state", filepath.Join(t.TempDir(), "absent.json"))
	_, err := loadDailyResources(t.Context(), cmd)
	require.Error(t, err)
	assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err), "an unreadable input file is a usage error, exit 2")
}

func TestReconcileDailyTarget_FindsTheProjectOfABareStack(t *testing.T) {
	t.Parallel()

	appResource := engine.ResourceDescriptor{ID: "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"}
	plain := engine.ResourceDescriptor{ID: "i-123"}

	t.Run("a URN names the project", func(t *testing.T) {
		t.Parallel()
		target, err := reconcileDailyTarget(
			t.TempDir(), historyTarget{Stack: "dev"}, []engine.ResourceDescriptor{appResource},
		)
		require.NoError(t, err)
		assert.Equal(t, "app", target.Project)
	})

	t.Run("one existing daily file names the project", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "app@dev.daily.db"), nil, 0o600))
		target, err := reconcileDailyTarget(dir, historyTarget{Stack: "dev"}, []engine.ResourceDescriptor{plain})
		require.NoError(t, err)
		assert.Equal(t, "app", target.Project)
	})

	t.Run("several existing daily files are ambiguous", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		for _, name := range []string{"app@dev.daily.db", "web@dev.daily.db"} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
		}
		_, err := reconcileDailyTarget(dir, historyTarget{Stack: "dev"}, []engine.ResourceDescriptor{plain})
		require.ErrorContains(t, err, "several projects")
	})

	t.Run("a known project is left alone", func(t *testing.T) {
		t.Parallel()
		target, err := reconcileDailyTarget(
			t.TempDir(), historyTarget{Project: "chosen", Stack: "dev"}, []engine.ResourceDescriptor{appResource},
		)
		require.NoError(t, err)
		assert.Equal(t, "chosen", target.Project)
	})
}

func readDailyIfPresent(t *testing.T, dir string) []history.DailyPoint {
	t.Helper()
	name, err := history.DailyFileNameFor("demo", "dev")
	require.NoError(t, err)
	if _, statErr := os.Stat(filepath.Join(dir, name)); statErr != nil {
		return nil
	}
	return readDaily(t, dir)
}

func TestCollectDailyPoints_AnOverflowingSumFailsOnlyItsResource(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	result := func(id string, cost float64) engine.CostResult {
		return engine.CostResult{
			ResourceID:        id,
			DailyObservations: []engine.DailyObservation{{Timestamp: day, Cost: cost, Currency: "USD"}},
		}
	}
	points, report := collectDailyPoints(&engine.CostResultWithErrors{Results: []engine.CostResult{
		result("urn:big", math.MaxFloat64),
		result("urn:ok", 3),
		result("urn:big", math.MaxFloat64),
	}}, nil, day.Add(48*time.Hour))

	require.Len(t, points, 1, "the healthy resource is still stored")
	assert.Equal(t, "urn:ok", points[0].ResourceID)
	require.Len(t, report.Errors, 1)
	assert.Equal(t, "urn:big", report.Errors[0].Resource)
}

func TestCollectDaily_TerraformStateNeedsAnExplicitFrom(t *testing.T) {
	t.Parallel()

	cmd, _, _ := dailyCmd(t, "--terraform-state", filepath.Join(t.TempDir(), "terraform.tfstate"))
	err := runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC) },
		dir: t.TempDir(), project: "demo", stack: "dev", cfg: config.New(),
	})
	require.ErrorContains(t, err, "--from is required when using --terraform-state")
	assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
}

func TestDailyWindow_OnlyFromPricesThroughNow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 31, 15, 4, 0, 0, time.UTC)
	cmd, _, _ := dailyCmd(t, "--from", "2026-10-01")
	from, to, err := dailyWindow(cmd, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), from)
	assert.Equal(t, now, to, "spec: only --from prices through the current time, so today's partial day is stored")

	cmd, _, _ = dailyCmd(t)
	from, to, err = dailyWindow(cmd, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), from, "no flags: the last 30 complete UTC days")
	assert.Equal(t, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), to)
}

func TestCollectDaily_RejectsAnInputFromAnotherProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	priced := false
	cmd, _, _ := dailyCmd(t, "--from", "2026-10-01", "--to", "2026-10-02")
	err := runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC) },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{
				ID: "urn:pulumi:dev::other::aws:ec2/instance:Instance::web", Type: "aws:ec2/instance:Instance",
			}}, nil
		},
		price: func(
			context.Context, *cobra.Command, []engine.ResourceDescriptor, time.Time, time.Time,
		) (*engine.CostResultWithErrors, error) {
			priced = true
			return &engine.CostResultWithErrors{}, nil
		},
		cfg: config.New(), dir: dir, project: "demo", stack: "dev",
	})

	require.ErrorContains(t, err, "other")
	assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
	assert.False(t, priced, "same stack name, different project: nothing is priced or written")
	assert.Empty(t, readDailyIfPresent(t, dir))
}

func TestCollectDailyPoints_AFailedReplacementIsNotMaskedByItsTwin(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	priced := func() *engine.CostResultWithErrors {
		return &engine.CostResultWithErrors{
			Results: []engine.CostResult{{
				ResourceID:        "urn:r",
				DailyObservations: []engine.DailyObservation{{Timestamp: day, Cost: 2, Currency: "USD"}},
			}},
			Errors: []engine.ErrorDetail{{ResourceID: "urn:r", Error: errors.New("plugin down")}},
		}
	}

	t.Run("two resources share the URN and one failed", func(t *testing.T) {
		t.Parallel()
		resources := []engine.ResourceDescriptor{{ID: "urn:r"}, {ID: "urn:r"}}
		points, report := collectDailyPoints(priced(), resources, day.Add(48*time.Hour))
		assert.Empty(t, points, "a partial sum would understate the day")
		require.Len(t, report.Errors, 1, "the failure must exempt the resource from pruning")
		assert.Equal(t, "urn:r", report.Errors[0].Resource)
	})

	t.Run("one resource answered after a failed attempt", func(t *testing.T) {
		t.Parallel()
		points, report := collectDailyPoints(
			priced(), []engine.ResourceDescriptor{{ID: "urn:r"}}, day.Add(48*time.Hour),
		)
		assert.Len(t, points, 1, "a fallback plugin's success stands")
		assert.Empty(t, report.Errors)
	})
}

// The working directory is process-wide.
func TestDailyTarget_ExplicitInputIgnoresProjectedTimelines(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := t.TempDir()
	for _, name := range []string{"other@dev.history.db", "third@dev.history.db"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
	}
	cmd, _, _ := dailyCmd(t, "--pulumi-state", "state.json", "--stack", "dev")

	target, err := dailyTarget(cmd, dailyCollectDeps{cfg: config.New()}, dir)
	require.NoError(t, err, "several projected timelines must not make an explicit input ambiguous")
	assert.Empty(t, target.Project, "the input's own URNs name the project, not a timeline of another project")
	assert.Equal(t, "dev", target.Stack)

	app := []engine.ResourceDescriptor{{ID: "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"}}
	target, err = reconcileDailyTarget(dir, target, app)
	require.NoError(t, err)
	assert.Equal(t, "app", target.Project)
}
