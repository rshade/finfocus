package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const preloadedCloudURN = "urn:pulumi:dev::test::aws:ec2/instance:Instance::cloud-destroyed"

func preloadInteractiveCloudHistory(
	t *testing.T,
) (*cobra.Command, *config.Config, engine.DateRange, string, *atomic.Int64) {
	t.Helper()
	setupMembershipProject(t)
	t.Setenv("FINFOCUS_HISTORY_ENABLED", "true")
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_ACCESS_TOKEN", "test-token")
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "org")
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	cfg := config.GetGlobalConfig()
	cfg.Cost.History.PulumiCloud.Enabled = config.Auto
	now := time.Now().Truncate(time.Second)
	dates := engine.DateRange{Start: now.Add(-24 * time.Hour), End: now}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch {
		case r.URL.Path == "/api/user":
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Query().Get("page") == "1":
			_, _ = fmt.Fprintf(
				w,
				`{"updates":[{"version":1,"kind":"update","endTime":%d}]}`,
				now.Add(-time.Hour).Unix(),
			)
		case r.URL.Query().Get("page") != "":
			_, _ = w.Write([]byte(`{"updates":[]}`))
		default:
			_, _ = fmt.Fprintf(
				w,
				`{"deployment":{"resources":[{"urn":%q,"id":"i-cloud-destroyed","type":"aws:ec2/instance:Instance","custom":true}]}}`,
				preloadedCloudURN,
			)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)
	directory := filepath.Join(config.ResolveConfigDir(), "history")
	store, err := history.NewBoltStore(context.Background(), directory, true, 90)
	require.NoError(t, err)
	cmd := membershipCommand()
	cmd.SetContext(context.Background())
	result, err := importActualCloudHistory(cmd, store, dates.Start, dates.End)
	require.NoError(t, err)
	require.Equal(t, 1, result.Recorded)
	require.NoError(t, store.Close())
	requests.Store(0)
	return cmd, cfg, dates, directory, &requests
}

type cloudHistoryWebActualClient struct {
	webActualClient

	actualRequests []*proto.GetActualCostRequest
}

func (c *cloudHistoryWebActualClient) GetActualCost(
	ctx context.Context,
	req *proto.GetActualCostRequest,
	opts ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	c.actualRequests = append(c.actualRequests, req)
	return c.webActualClient.GetActualCost(ctx, req, opts...)
}

//nolint:paralleltest // Installs a global Pulumi runner and changes process environment and directory.
func TestWebActualUsesPreloadedCloudHistoryWithoutRequests(t *testing.T) {
	cmd, cfg, dates, directory, requests := preloadInteractiveCloudHistory(t)
	store, err := history.NewBoltStore(context.Background(), directory, true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	var offTimes, autoTimes []time.Duration
	for _, mode := range []config.PulumiCloudMode{config.Off, config.Auto} {
		cfg.Cost.History.PulumiCloud.Enabled = mode
		for range 5 {
			client := &cloudHistoryWebActualClient{}
			start := time.Now()
			data := newWebData(
				cmd,
				overviewParams{cfg: cfg},
				dates,
				newAuditContext(context.Background(), "test", nil),
			)
			data.eng = engine.New([]*pluginhost.Client{{Name: "aws", API: client}}, nil).WithJobs(1)
			data.history = store
			publishWebTestState(t, data, nil)
			result, costErr := data.actualCosts(context.Background(), "", nil)
			duration := time.Since(start)
			require.NoError(t, costErr)
			require.NotNil(t, result)
			require.Len(t, client.actualRequests, 1)
			assert.Contains(t, client.actualRequests[0].ResourceIDs, preloadedCloudURN)
			assert.Equal(
				t,
				"i-cloud-destroyed",
				client.actualRequests[0].Properties["pulumi:cloudId"],
			)
			if mode == config.Off {
				offTimes = append(offTimes, duration)
			} else {
				autoTimes = append(autoTimes, duration)
			}
		}
	}
	t.Logf("median off=%v auto=%v", cloudHistoryMedian(offTimes), cloudHistoryMedian(autoTimes))
	assert.Less(t, cloudHistoryMedian(autoTimes)-cloudHistoryMedian(offTimes), 100*time.Millisecond)
	assert.Zero(t, requests.Load())
}

//nolint:paralleltest // Changes process environment and directory through preloadInteractiveCloudHistory.
func TestOverviewCloudHistoryDoesNotFetchAndPreservesCurrentInventory(t *testing.T) {
	cmd, cfg, dates, _, requests := preloadInteractiveCloudHistory(t)
	// User clarification (2026-10-27): overview keeps state/plan inventory;
	// preloaded identities appear in web Cost, with no historical overview rows.
	for _, loader := range []OverviewPipelineLoader{plainOverviewLoader, tuiOverviewLoader, autoDetectOverviewLoader} {
		var offTimes, autoTimes []time.Duration
		var baselineURNs []string
		for _, mode := range []config.PulumiCloudMode{config.Off, config.Auto} {
			cfg.Cost.History.PulumiCloud.Enabled = mode
			for range 5 {
				start := time.Now()
				pipe := newOverviewPipeline(
					overviewPipelineConfig{
						cmd:         cmd,
						params:      overviewParams{cfg: cfg, stateOnly: true},
						dateRange:   dates,
						loader:      loader,
						openPlugins: fakeOpenPlugins,
						newEngine:   fakeNewEngine,
						enrichRows:  passThroughEnrichment,
					},
				)
				err := pipe.Run(context.Background())
				duration := time.Since(start)
				require.NoError(t, err)
				var urns []string
				for _, row := range pipe.Result().Rows {
					urns = append(urns, row.URN)
				}
				assert.NotEmpty(t, urns)
				assert.NotContains(t, urns, preloadedCloudURN)
				if baselineURNs == nil {
					baselineURNs = append([]string(nil), urns...)
				} else {
					assert.ElementsMatch(t, baselineURNs, urns)
				}
				if mode == config.Off {
					offTimes = append(offTimes, duration)
				} else {
					autoTimes = append(autoTimes, duration)
				}
			}
		}
		assert.Less(
			t,
			cloudHistoryMedian(autoTimes)-cloudHistoryMedian(offTimes),
			100*time.Millisecond,
		)
	}
	assert.Zero(t, requests.Load())
}
