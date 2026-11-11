package cli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const snapshotInstanceURN = "urn:pulumi:dev::test-stack::aws:ec2/instance:Instance::new"

// snapshotPlugin answers recommendation queries with a fixed list or error.
// Other RPCs are unused.
type snapshotPlugin struct {
	proto.CostSourceClient

	mu    sync.Mutex
	recs  []*proto.Recommendation
	err   error
	calls []*proto.GetRecommendationsRequest
}

func (p *snapshotPlugin) GetRecommendations(
	_ context.Context,
	in *proto.GetRecommendationsRequest,
	_ ...grpc.CallOption,
) (*proto.GetRecommendationsResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, in)
	if p.err != nil {
		return nil, p.err
	}
	return &proto.GetRecommendationsResponse{Recommendations: p.recs}, nil
}

func (p *snapshotPlugin) Supports(
	context.Context,
	*pbc.SupportsRequest,
	...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (p *snapshotPlugin) requests() []*proto.GetRecommendationsRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*proto.GetRecommendationsRequest(nil), p.calls...)
}

func spotRecommendation(id string) *proto.Recommendation {
	return &proto.Recommendation{
		ID:          id,
		ActionType:  "MIGRATE",
		Description: "Move new to spot",
		ResourceID:  snapshotInstanceURN,
		Impact: &proto.RecommendationImpact{
			EstimatedSavings: 42.5,
			Currency:         "USD",
		},
		Metadata:             map[string]string{"account": "123456789012"},
		Reasoning:            []string{"interruptions restart the instance"},
		PricingCategory:      "DYNAMIC",
		SpotInterruptionRisk: 0.12,
	}
}

// setupDismissSnapshot isolates the home, cache, and working directory,
// installs plugin as the only plugin dismiss can open, and returns the plan
// path and a counter of plugin opens.
func setupDismissSnapshot(t *testing.T, plugin *snapshotPlugin) (string, *int) {
	t.Helper()
	plan, err := os.ReadFile("testdata/pulumi/preview-create-only.json")
	require.NoError(t, err)
	isolateHistoryHome(t)
	t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())
	t.Setenv("FINFOCUS_HISTORY_ENABLED", "false")
	work := t.TempDir()
	t.Chdir(work)
	planPath := filepath.Join(work, "preview.json")
	require.NoError(t, os.WriteFile(planPath, plan, 0o600))

	opened := 0
	cli.SetDismissPluginOpenerForTest(t, func(context.Context) ([]*pluginhost.Client, func(), error) {
		opened++
		return []*pluginhost.Client{{Name: "aws", API: plugin}}, func() {}, nil
	})
	return planPath, &opened
}

func runRecommendationsCmd(t *testing.T, args ...string) axtest.Result {
	t.Helper()
	root := cli.NewRootCmd("test-version")
	return axtest.Run(context.Background(), t, root, append([]string{"cost", "recommendations"}, args...))
}

func loadDismissal(t *testing.T, id string) (*config.DismissalRecord, bool) {
	t.Helper()
	store, err := config.NewDismissalStore("")
	require.NoError(t, err)
	require.NoError(t, store.Load())
	return store.Get(id)
}

func assertSpotSnapshot(t *testing.T, record *config.DismissalRecord) {
	t.Helper()
	require.NotNil(t, record.LastKnown, "the dismissal must record the last-known recommendation")
	assert.Equal(t, &config.LastKnownRecommendation{
		Description:          "Move new to spot",
		EstimatedSavings:     42.5,
		Currency:             "USD",
		Type:                 "MIGRATE",
		ResourceID:           snapshotInstanceURN,
		PricingCategory:      "DYNAMIC",
		SpotInterruptionRisk: 0.12,
	}, record.LastKnown)
}

//nolint:paralleltest // sets FINFOCUS_HOME, the working directory, and the dismiss plugin opener
func TestDismissCmd_RecordsLastKnownSnapshot(t *testing.T) {
	plugin := &snapshotPlugin{recs: []*proto.Recommendation{spotRecommendation("rec-spot-1")}}
	planPath, _ := setupDismissSnapshot(t, plugin)

	result := runRecommendationsCmd(t, "dismiss", "rec-spot-1",
		"--reason", "business-constraint", "--pulumi-json", planPath, "--yes")
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))

	record, ok := loadDismissal(t, "rec-spot-1")
	require.True(t, ok)
	assert.Equal(t, config.StatusDismissed, record.Status)
	assertSpotSnapshot(t, record)

	store, err := config.NewDismissalStore("")
	require.NoError(t, err)
	body, err := os.ReadFile(store.FilePath())
	require.NoError(t, err)
	assert.NotContains(t, string(body), "123456789012", "metadata must not reach dismissed.json")
	assert.NotContains(t, string(body), "interruptions restart", "reasoning must not reach dismissed.json")

	reqs := plugin.requests()
	require.Len(t, reqs, 1)
	assert.False(t, reqs[0].IncludeDismissed, "a new dismissal uses the default query")
}

//nolint:paralleltest // sets FINFOCUS_HOME, the working directory, and the dismiss plugin opener
func TestSnoozeCmd_RecordsLastKnownSnapshot(t *testing.T) {
	plugin := &snapshotPlugin{recs: []*proto.Recommendation{spotRecommendation("rec-spot-2")}}
	planPath, _ := setupDismissSnapshot(t, plugin)
	until := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	result := runRecommendationsCmd(t, "snooze", "rec-spot-2",
		"--until", until, "--pulumi-json", planPath, "--yes")
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))

	record, ok := loadDismissal(t, "rec-spot-2")
	require.True(t, ok)
	assert.Equal(t, config.StatusSnoozed, record.Status)
	assertSpotSnapshot(t, record)
}

// A recommendation that is already dismissed is excluded from the default
// query, so snoozing it asks the plugin to include dismissed ones. When the
// plugin no longer returns it, the earlier snapshot is kept.
//
//nolint:paralleltest // sets FINFOCUS_HOME, the working directory, and the dismiss plugin opener
func TestSnoozeCmd_AlreadyDismissedKeepsSnapshot(t *testing.T) {
	plugin := &snapshotPlugin{recs: []*proto.Recommendation{spotRecommendation("rec-spot-3")}}
	planPath, _ := setupDismissSnapshot(t, plugin)

	result := runRecommendationsCmd(t, "dismiss", "rec-spot-3",
		"--reason", "deferred", "--pulumi-json", planPath, "--yes")
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))

	plugin.mu.Lock()
	plugin.recs = nil
	plugin.mu.Unlock()
	until := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	result = runRecommendationsCmd(t, "snooze", "rec-spot-3",
		"--until", until, "--pulumi-json", planPath, "--yes")
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))

	reqs := plugin.requests()
	require.Len(t, reqs, 2)
	assert.True(t, reqs[1].IncludeDismissed, "an already-dismissed id needs the include-dismissed query")

	record, ok := loadDismissal(t, "rec-spot-3")
	require.True(t, ok)
	assert.Equal(t, config.StatusSnoozed, record.Status)
	assert.Len(t, record.History, 2)
	assertSpotSnapshot(t, record)
}

//nolint:paralleltest // sets FINFOCUS_HOME, the working directory, and the dismiss plugin opener
func TestDismissCmd_LookupMissStillDismisses(t *testing.T) {
	tests := []struct {
		name   string
		plugin *snapshotPlugin
	}{
		{
			name:   "id not returned",
			plugin: &snapshotPlugin{recs: []*proto.Recommendation{spotRecommendation("rec-other")}},
		},
		{name: "plugin error", plugin: &snapshotPlugin{err: errors.New("Unavailable: plugin down")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planPath, _ := setupDismissSnapshot(t, tt.plugin)

			result := runRecommendationsCmd(t, "dismiss", "rec-gone",
				"--reason", "not-applicable", "--pulumi-json", planPath, "--yes")
			require.Equal(t, 0, result.ExitCode, string(result.Stderr))
			assert.Equal(t, "Recommendation rec-gone dismissed.\n  Local state updated.\n", string(result.Stdout))

			record, ok := loadDismissal(t, "rec-gone")
			require.True(t, ok)
			assert.Equal(t, config.StatusDismissed, record.Status)
			assert.Nil(t, record.LastKnown)
		})
	}
}

//nolint:paralleltest // sets FINFOCUS_HOME, the working directory, and the dismiss plugin opener
func TestDismissCmd_WithoutPlanOpensNoPlugins(t *testing.T) {
	plugin := &snapshotPlugin{recs: []*proto.Recommendation{spotRecommendation("rec-local")}}
	_, opened := setupDismissSnapshot(t, plugin)

	result := runRecommendationsCmd(t, "dismiss", "rec-local", "--reason", "not-applicable", "--yes")
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))

	assert.Zero(t, *opened)
	record, ok := loadDismissal(t, "rec-local")
	require.True(t, ok)
	assert.Nil(t, record.LastKnown)
}

//nolint:paralleltest // sets FINFOCUS_HOME, the working directory, and the dismiss plugin opener
func TestDismissCmd_DryRunSkipsLookupAndWrite(t *testing.T) {
	for _, args := range [][]string{
		{"dismiss", "rec-dry", "--reason", "not-applicable"},
		{"snooze", "rec-dry", "--until", time.Now().AddDate(0, 1, 0).Format("2006-01-02")},
	} {
		t.Run(args[0], func(t *testing.T) {
			plugin := &snapshotPlugin{recs: []*proto.Recommendation{spotRecommendation("rec-dry")}}
			planPath, _ := setupDismissSnapshot(t, plugin)

			result := runRecommendationsCmd(t, append(args, "--pulumi-json", planPath, "--yes", "--dry-run")...)
			require.Equal(t, 0, result.ExitCode, string(result.Stderr))

			assert.Empty(t, plugin.requests(), "dry run must not query the plugin")
			store, err := config.NewDismissalStore("")
			require.NoError(t, err)
			assert.NoFileExists(t, store.FilePath())
		})
	}
}
