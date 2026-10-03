package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/detect"
	"github.com/rshade/finfocus/internal/history"
)

func TestCollect_ParsesStackHistory(t *testing.T) {
	t.Parallel()
	updates, err := history.ParseStackHistory([]byte(
		`[{"version":7,"kind":"update","result":"succeeded",` +
			`"startTime":"2025-05-01T00:00:00Z","message":"ship","resourceChanges":{"create":1}}]`,
	))
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, "ship", updates[0].Message)
	assert.Equal(t, 1, updates[0].ResourceChanges["create"])

	dir := t.TempDir()
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "dev", "--parallel", "1")
	err = runCollect(cmd, collectDeps{
		dir:  dir,
		look: pulumiLook,
		run: scriptedHistoryPulumi(
			historyBody(t, histRow(7, "2025-05-01T00:00:00Z", "ship")),
			map[int][]byte{7: exportBody("urn:web", "m5.large")},
			[]byte(`[{"name":"dev"}]`),
		),
		price: fixedPrice(25, "aws-public"),
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Detecting Pulumi CLI... v3.120.0")
	assert.Contains(t, stdout.String(), "[v7 — 2025-05-01]")
	db := openHistory(t, dir)
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.InDelta(t, 25, snaps[0].TotalMonthly, 0.001)
	assert.Equal(t, "m5.large", snaps[0].Resources[0].SKU)
}

func TestCollect_FiltersSuccessful(t *testing.T) {
	t.Parallel()
	updates := []history.StackUpdate{
		{Version: 1, Kind: "update", Result: "failed"},
		{Version: 2, Kind: "update", Result: "succeeded"},
		{Version: 3, Kind: "import", Result: "succeeded"},
		{Version: 4, Kind: "destroy", Result: "succeeded"},
	}
	selected := history.SelectUpdates(updates, history.SelectOptions{})
	assert.Equal(t, []int{2, 4}, versionNumbers(selected))
	skipped := history.SelectUpdates(updates, history.SelectOptions{SkipDestroy: true})
	assert.Equal(t, []int{2}, versionNumbers(skipped))
}

func TestCollect_IncrementalSkips(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := historyBody(t,
		histRow(1, "2025-01-01T00:00:00Z", "one"),
		histRow(2, "2025-02-01T00:00:00Z", "two"),
	)
	exports := map[int][]byte{
		1: exportBody("urn:web", "t3.micro"),
		2: exportBody("urn:web", "t3.micro"),
	}
	deps := collectDeps{
		dir: dir, look: pulumiLook,
		run:   scriptedHistoryPulumi(body, exports, []byte(`[{"name":"dev"}]`)),
		price: fixedPrice(5, "aws-public"),
	}
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "dev", "--parallel", "1")
	require.NoError(t, runCollect(cmd, deps))
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "dev", "--parallel", "1")
	require.NoError(t, runCollect(cmd, deps))
	assert.Contains(t, stdout.String(), "No new versions to collect.")
	db := openHistory(t, dir)
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, snaps, 2)
}

func TestCollect_FailFastMissingPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "dev", "--parallel", "1")
	err := runCollect(cmd, collectDeps{
		dir:  dir,
		look: pulumiLook,
		run: scriptedHistoryPulumi(
			historyBody(t, histRow(1, "2025-01-01T00:00:00Z", "bad")),
			map[int][]byte{1: exportBody("urn:gcp", "n1-standard-1")},
			[]byte(`[{"name":"dev"}]`),
		),
		price: fixedPrice(0, "none"),
	})
	require.ErrorIs(t, err, history.ErrNoPlugin)
	require.ErrorContains(t, err, "no plugin available for resource type 'aws:ec2/instance:Instance'")
	db := openHistory(t, dir)
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, snaps)
}

func TestCollect_MissingPulumiAndStack(t *testing.T) {
	t.Parallel()
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "prod")
	err := runCollect(cmd, collectDeps{look: func(string) (string, error) { return "", errors.New("missing") }})
	require.ErrorIs(t, err, detect.ErrPulumiMissing)

	cmd, _ = preparedHistoryCmd(t, NewCostHistoryCollectCmd())
	err = runCollect(cmd, collectDeps{})
	require.ErrorIs(t, err, ErrStackRequired)

	cmd, _ = preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "prod")
	err = runCollect(cmd, collectDeps{
		look: pulumiLook,
		run:  scriptedHistoryPulumi(nil, nil, []byte(`[{"name":"dev"},{"name":"staging"}]`)),
	})
	require.ErrorContains(t, err, "stack 'prod' not found. Available stacks: dev, staging")
}

func TestCollect_AutoPruneKeepsNewest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev", viewSnap(1, time.January, 10, map[string]float64{"aws": 10}))
	cfg := config.New()
	cfg.Cost.History.Retention.AutoPrune = true
	cfg.Cost.History.Retention.MaxSnapshots = 1
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "dev", "--parallel", "1")
	err := runCollect(cmd, collectDeps{
		dir:  dir,
		look: pulumiLook,
		run: scriptedHistoryPulumi(
			historyBody(t,
				histRow(1, "2025-01-01T00:00:00Z", "one"),
				histRow(2, "2025-02-01T00:00:00Z", "two"),
			),
			map[int][]byte{
				1: exportBody("urn:web", "t3.micro"),
				2: exportBody("urn:web", "t3.micro"),
			},
			[]byte(`[{"name":"dev"}]`),
		),
		price: fixedPrice(5, "aws-public"),
		cfg:   cfg,
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Pruned 1 snapshots (retention policy).")
	assert.Contains(t, stdout.String(), "Compacted database:")
	db := openHistory(t, dir)
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, 2, snaps[0].Version)
}

func fixedPrice(monthly float64, adapter string) history.Pricer {
	return func(_ context.Context, resources []history.PriceResource) ([]history.PriceQuote, error) {
		results := make([]history.PriceQuote, 0, len(resources))
		for _, resource := range resources {
			results = append(results, history.PriceQuote{
				ResourceID:   resource.ID,
				ResourceType: resource.Type,
				Adapter:      adapter,
				Currency:     "USD",
				Monthly:      monthly,
			})
		}
		return results, nil
	}
}

func openHistory(t *testing.T, dir string) *history.CostDB {
	t.Helper()
	path := filepath.Join(dir, "dev.history.db")
	db, err := history.OpenCostDBRead(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func versionNumbers(updates []history.StackUpdate) []int {
	out := make([]int, len(updates))
	for i, update := range updates {
		out[i] = update.Version
	}
	return out
}

func historyBody(t *testing.T, rows ...map[string]any) []byte {
	t.Helper()
	return mustJSON(t, rows)
}

func histRow(version int, start, message string) map[string]any {
	return map[string]any{
		"version": version, "kind": "update", "result": "succeeded",
		"startTime": start, "message": message,
	}
}

func exportBody(urn, instance string) []byte {
	body, err := jsonMarshal(map[string]any{
		"version": 3,
		"deployment": map[string]any{
			"resources": []any{map[string]any{
				"urn": urn, "custom": true, "type": "aws:ec2/instance:Instance",
				"inputs": map[string]any{"instanceType": instance, "region": "us-east-1"},
			}},
		},
	})
	if err != nil {
		panic(err)
	}
	return body
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := jsonMarshal(value)
	require.NoError(t, err)
	return body
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}
