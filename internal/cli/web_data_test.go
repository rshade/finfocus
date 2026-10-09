package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestWebActualTagParserParity(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"env=prod", "team=platform"} {
		got, err := engine.ParseActualTagFilter(tag)
		require.NoError(t, err)
		want, group := parseTagFilter("tag:" + tag)
		assert.Empty(t, group)
		assert.Equal(t, want, got)
	}
	for _, tag := range []string{"broken", "=prod", "env=", "env=a=b"} {
		_, err := engine.ParseActualTagFilter(tag)
		require.Error(t, err)
	}
}

func TestWebSeparateScorerOwnership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{
		Scoring: &config.ScoringConfig{Enabled: true, Plugin: "scorer"},
		Cost:    config.CostConfig{History: config.HistoryConfig{Directory: t.TempDir()}},
	}
	data := newWebData(&cobra.Command{}, overviewParams{cfg: cfg, adapter: "pricing"},
		engine.DateRange{}, newAuditContext(ctx, "test", nil))
	pricing := &pluginhost.Client{Name: "pricing", API: &includeDismissedClient{}}
	scorer := &pluginhost.Client{Name: "scorer"}
	released := 0
	data.resolveScorer = func(_ context.Context, adapter string, scoring config.ResolvedScoring,
		clients []*pluginhost.Client, _ *auditContext,
	) (*pluginhost.Client, string, func()) {
		assert.Equal(t, "pricing", adapter)
		assert.Equal(t, "scorer", scoring.Plugin)
		assert.Equal(t, []*pluginhost.Client{pricing}, clients)
		return scorer, "", func() { released++ }
	}
	eng, cleanup := data.newEngine(ctx, data.cmd, []*pluginhost.Client{pricing})
	require.NotNil(t, eng)
	assert.Equal(t, []*pluginhost.Client{pricing, scorer}, data.clients)
	assert.Zero(t, released)
	cleanup()
	assert.Equal(t, 1, released)
}

func TestWebUnavailableSeparateScorerCleanup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{
		Scoring: &config.ScoringConfig{Enabled: true, Plugin: "scorer"},
		Cost:    config.CostConfig{History: config.HistoryConfig{Directory: t.TempDir()}},
	}
	data := newWebData(&cobra.Command{}, overviewParams{cfg: cfg, adapter: "pricing"},
		engine.DateRange{}, newAuditContext(ctx, "test", nil))
	released := 0
	data.resolveScorer = func(context.Context, string, config.ResolvedScoring,
		[]*pluginhost.Client, *auditContext,
	) (*pluginhost.Client, string, func()) {
		return nil, "scorer unavailable", func() { released++ }
	}
	eng, cleanup := data.newEngine(ctx, data.cmd, nil)
	require.NotNil(t, eng)
	assert.Empty(t, data.clients)
	assert.Zero(t, released)
	cleanup()
	assert.Equal(t, 1, released)
}

type webScorerServer struct {
	pbc.UnimplementedRecommendationScorerServiceServer

	mock *plugintesting.MockRecommendationScorer
}

func (s *webScorerServer) ScoreRecommendations(
	ctx context.Context, request *pbc.ScoreRecommendationsRequest,
) (*pbc.ScoreRecommendationsResponse, error) {
	return s.mock.ScoreRecommendations(ctx, request)
}

func TestWebSeparateScorerSharedSDKPipeline(t *testing.T) {
	t.Parallel()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pbc.RegisterRecommendationScorerServiceServer(server,
		&webScorerServer{mock: plugintesting.NewMockRecommendationScorer()})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	conn, err := grpc.NewClient("passthrough:///web-scorer",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg := &config.Config{
		Scoring: &config.ScoringConfig{Enabled: true, Plugin: "scorer"},
		Cost:    config.CostConfig{History: config.HistoryConfig{Directory: t.TempDir()}},
	}
	data := newWebData(&cobra.Command{}, overviewParams{cfg: cfg, adapter: "pricing"},
		engine.DateRange{}, newAuditContext(ctx, "test", nil))
	scorer := &pluginhost.Client{Name: "scorer", Conn: conn,
		Metadata: &proto.PluginMetadata{Capabilities: []string{pluginhost.CapabilityRecommendationScoring}}}
	resolved := 0
	data.resolveScorer = func(context.Context, string, config.ResolvedScoring,
		[]*pluginhost.Client, *auditContext,
	) (*pluginhost.Client, string, func()) {
		resolved++
		return scorer, "", func() { require.NoError(t, conn.Close()) }
	}
	_, cleanup := data.newEngine(ctx, data.cmd, nil)
	data.captureState(nil)
	data.fetchRecommendations = func(context.Context, *cobra.Command, *engine.Engine,
		[]engine.ResourceDescriptor, bool,
	) (*engine.RecommendationsResult, error) {
		return &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
			{ID: "r1", ResourceID: "i-1", Type: "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE", Description: "Resize"},
		}}, nil
	}
	for range 2 {
		result, fetchErr := data.recommendations(ctx, false)
		require.NoError(t, fetchErr)
		require.NotNil(t, result.Scoring)
		assert.Equal(t, 1, result.Scoring.Scored)
		require.NotNil(t, result.Recommendations[0].Scores)
		assert.Equal(t, engine.RecommendationStatusActive, result.Recommendations[0].Status)
	}
	assert.Equal(t, 1, resolved)
	assert.NotEqual(t, connectivity.Shutdown, conn.GetState())
	cleanup()
	assert.Equal(t, connectivity.Shutdown, conn.GetState())
}

func TestWebRecommendationsFailureCleanupAndRecovery(t *testing.T) {
	t.Parallel()
	failure := errors.New("fixture failure")
	for _, tc := range []struct {
		stage    string
		wantErr  error
		wantNil  bool
		released int
	}{{"fetch", failure, false, 0}, {"nil", nil, true, 0},
		{"score", failure, true, 1}, {"merge", nil, false, 1}} {
		t.Run(tc.stage, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			data := newWebData(&cobra.Command{}, overviewParams{cfg: &config.Config{}},
				engine.DateRange{}, newAuditContext(ctx, "test", nil))
			data.captureState(nil)
			result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{{ID: "r1"}}}
			data.fetchRecommendations = func(context.Context, *cobra.Command, *engine.Engine,
				[]engine.ResourceDescriptor, bool,
			) (*engine.RecommendationsResult, error) {
				if tc.stage == "fetch" {
					return result, failure
				}
				if tc.stage == "nil" {
					//nolint:nilnil // Exercise a malformed dependency result without an error.
					return nil, nil
				}
				return result, nil
			}
			released := 0
			data.scoreRecommendations = func(context.Context, *cobra.Command, costRecommendationsParams,
				config.ResolvedScoring, []*pluginhost.Client, cache.Cache, *auditContext,
				*engine.RecommendationsResult,
			) (bool, func(), error) {
				cleanup := func() { released++ }
				if tc.stage == "score" {
					return false, cleanup, failure
				}
				return false, cleanup, nil
			}
			data.mergeDismissed = func(_ context.Context, got *engine.RecommendationsResult) error {
				assert.Same(t, result, got)
				return failure
			}
			got, err := data.recommendations(ctx, true)
			require.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantNil, got == nil)
			if !tc.wantNil {
				assert.Same(t, result, got)
			}
			assert.Equal(t, tc.released, released)
			unlock, lockErr := data.lock(ctx)
			require.NoError(t, lockErr)
			unlock()
		})
	}
}

func TestWebRecommendationSummaryParity(t *testing.T) {
	t.Parallel()
	recs := []engine.Recommendation{
		{Type: "Delete", EstimatedSavings: 10, Currency: "USD"},
		{Type: "Resize", EstimatedSavings: 3, Currency: "USD"},
	}
	assert.Equal(t, buildJSONSummary(recs), engine.BuildRecommendationSummary(recs))
}

func TestWebActualSourcesUseRawSharedLoader(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cmd := &cobra.Command{}
	audit := newAuditContext(ctx, "test", nil)
	data := newWebData(
		cmd,
		overviewParams{pulumiState: "../../testdata/overview/state-no-changes.json"},
		engine.DateRange{},
		audit,
	)
	want, err := loadActualResources(
		ctx,
		cmd,
		costActualParams{statePath: "../../testdata/overview/state-no-changes.json"},
		audit,
	)
	require.NoError(t, err)
	got, err := data.loadResources(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// webActualClient exercises the same plugin protocol used by cost actual.
type webActualClient struct{ includeDismissedClient }

func (c *webActualClient) GetActualCost(
	_ context.Context,
	_ *proto.GetActualCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	return &proto.GetActualCostResponse{
		Results: []*proto.ActualCostResult{
			{Currency: "USD", TotalCost: 12, CostBreakdown: map[string]float64{"compute": 12}},
		},
	}, nil
}

func TestWebActualSharedEnginePipelineParity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &webActualClient{}
	eng := engine.New([]*pluginhost.Client{{Name: "fixture", API: client}}, nil)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	dates := engine.DateRange{Start: start, End: start.AddDate(0, 0, 2)}
	cmd := &cobra.Command{}
	audit := newAuditContext(ctx, "test", nil)
	data := newWebData(cmd, overviewParams{cfg: &config.Config{}}, dates, audit)
	data.eng = eng
	data.captureState(
		[]ingest.StackExportResource{
			{
				URN:    "i-1",
				Type:   "aws:ec2/instance:Instance",
				Custom: true,
				Inputs: map[string]any{"instanceType": "t3.small", "tags": map[string]any{"env": "prod"}},
			},
		},
	)
	resources, err := data.loadResources(ctx)
	require.NoError(t, err)
	for _, group := range []string{"", "resource", "type", "provider", "date", "daily", "monthly"} {
		// Each request traverses the engine's own grouping path, never adapter grouping.
		got, fetchErr := data.actualCosts(ctx, group, map[string]string{"env": "prod"})
		require.NoError(t, fetchErr)
		request := buildActualCostRequest(costActualParams{groupBy: group}, resources, dates.Start, dates.End)
		request.Tags = map[string]string{"env": "prod"}
		want, queryErr := eng.GetActualCostWithOptionsAndErrors(ctx, request)
		require.NoError(t, queryErr)
		fetchAndMergeRecommendations(ctx, eng, resources, want.Results)
		assert.Equal(t, want.Results, got.Results)
	}
}

func TestWebCapturedSourceIsIndependentAndEmptyIsValid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	data := newWebData(
		&cobra.Command{},
		overviewParams{cfg: &config.Config{}},
		engine.DateRange{},
		newAuditContext(ctx, "test", nil),
	)
	_, err := data.loadResources(ctx)
	require.Error(t, err)
	modified := time.Now()
	source := []ingest.StackExportResource{
		{
			URN:                  "raw",
			Type:                 "aws:ec2/instance:Instance",
			ID:                   "i-raw",
			External:             true,
			Modified:             &modified,
			Inputs:               map[string]any{"nested": map[string]any{"value": "original"}},
			PropertyDependencies: map[string][]string{"vpc": {"urn:vpc"}},
		},
	}
	data.captureState(source)
	want, err := ingest.MapStateResources(source)
	require.NoError(t, err)
	got, err := data.loadResources(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	nested, ok := got[0].Properties["nested"].(map[string]any)
	require.True(t, ok)
	nested["value"] = "edited"
	got[0].Refs["vpc"][0] = "edited"
	assert.Equal(t, want, data.raw)
	data.captureState(nil)
	_, err = data.loadResources(ctx)
	require.NoError(t, err)
}

func TestWebRequestGateCancellationAndLoadFailures(t *testing.T) {
	t.Parallel()
	data := newWebData(
		&cobra.Command{},
		overviewParams{pulumiState: "missing.json", cfg: &config.Config{}},
		engine.DateRange{},
		newAuditContext(context.Background(), "test", nil),
	)
	release, err := data.lock(context.Background())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = data.actualCosts(ctx, "", nil)
	require.ErrorIs(t, err, context.Canceled)
	_, err = data.recommendations(ctx, false)
	require.ErrorIs(t, err, context.Canceled)
	release()
	_, err = data.actualCosts(context.Background(), "", nil)
	require.Error(t, err)
	_, err = data.recommendations(context.Background(), true)
	require.Error(t, err)
}

func TestWebRecommendationsSharedFetchParity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &includeDismissedClient{}
	eng := engine.New([]*pluginhost.Client{{Name: "fixture", API: client}}, nil)
	data := newWebData(
		&cobra.Command{},
		overviewParams{cfg: &config.Config{}},
		engine.DateRange{},
		newAuditContext(ctx, "test", nil),
	)
	data.eng = eng
	data.captureState(
		[]ingest.StackExportResource{
			{URN: "i-1", Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.small"}},
		},
	)
	for _, include := range []bool{false, true} {
		result, err := data.recommendations(ctx, include)
		require.NoError(t, err)
		require.NotEmpty(t, result.Recommendations)
		assert.Equal(t, engine.RecommendationStatusActive, result.Recommendations[0].Status)
	}
	requests := client.requests()
	require.Len(t, requests, 2)
	assert.False(t, requests[0].IncludeDismissed)
	assert.True(t, requests[1].IncludeDismissed)
}

func TestWebDataEngineAndSessionWiring(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cmd := &cobra.Command{}
	cfg := &config.Config{
		Scoring: &config.ScoringConfig{Enabled: true, Plugin: "absent"},
		Cost:    config.CostConfig{History: config.HistoryConfig{Directory: t.TempDir()}},
	}
	data := newWebData(cmd, overviewParams{cfg: cfg}, engine.DateRange{}, newAuditContext(ctx, "test", nil))
	params := data.overviewParams()
	require.NotNil(t, params.captureState)
	params.captureState(nil)
	eng, cleanup := data.newEngine(ctx, cmd, nil)
	defer cleanup()
	require.Same(t, eng, data.eng)
	trends, total := data.tableTrends()
	assert.Empty(t, trends)
	assert.Empty(t, total)
	options := webSessionOptions(
		ctx,
		data,
		newOverviewPipeline(overviewPipelineConfig{}),
		make(chan struct{}),
		make(chan string),
	)
	require.NotNil(t, options.ActualCosts)
	require.NotNil(t, options.Recommendations)
	require.NotNil(t, options.Trends)
	result, err := options.Recommendations(ctx, false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Scoring)
	assert.Empty(t, result.Recommendations)
}

func TestWebSessionOptionsCancellationAndHandoff(t *testing.T) {
	t.Parallel()
	pipeCtx, pipeCancel := context.WithCancel(context.Background())
	defer pipeCancel()
	requestCtx, requestCancel := context.WithCancel(context.Background())
	requestCancel()
	data := newWebData(
		&cobra.Command{},
		overviewParams{cfg: &config.Config{}},
		engine.DateRange{},
		newAuditContext(pipeCtx, "test", nil),
	)
	done := make(chan struct{})
	close(done)
	passphrases := make(chan string)
	options := webSessionOptions(pipeCtx, data, newOverviewPipeline(overviewPipelineConfig{}), done, passphrases)
	require.ErrorIs(t, options.SubmitPassphrase(requestCtx, "discard"), context.Canceled)
	delivered := make(chan string, 1)
	go func() { delivered <- <-passphrases }()
	require.NoError(t, options.SubmitPassphrase(context.Background(), "operation-secret"))
	assert.Equal(t, "operation-secret", <-delivered)
	_, err := options.Preview(context.Background())
	require.Error(t, err)
	pipeCancel()
	require.ErrorIs(t, options.SubmitPassphrase(context.Background(), "discard"), context.Canceled)
}

func TestWebDomainPipelinesRejectInvalidFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	data := newWebData(
		&cobra.Command{},
		overviewParams{cfg: &config.Config{}, filter: []string{"invalid"}},
		engine.DateRange{},
		newAuditContext(ctx, "test", nil),
	)
	data.eng = engine.New(nil, nil)
	data.captureState([]ingest.StackExportResource{{URN: "r", Type: "aws:ec2/instance:Instance"}})
	_, err := data.actualCosts(ctx, "", nil)
	require.Error(t, err)
	_, err = data.recommendations(ctx, false)
	require.Error(t, err)
	data.params.filter = nil
	data.captureState([]ingest.StackExportResource{{URN: "invalid", Type: ""}})
	_, err = data.actualCosts(ctx, "", nil)
	require.Error(t, err)
}

func TestWebDataReusesAlreadyOpenScorer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &pluginhost.Client{
		Name:     "scorer",
		Metadata: &proto.PluginMetadata{Capabilities: []string{pluginhost.CapabilityRecommendationScoring}},
		API:      &includeDismissedClient{},
	}
	cfg := &config.Config{
		Scoring: &config.ScoringConfig{Enabled: true, Plugin: "scorer"},
		Cost:    config.CostConfig{History: config.HistoryConfig{Directory: t.TempDir()}},
	}
	data := newWebData(
		&cobra.Command{},
		overviewParams{cfg: cfg, adapter: "pricing"},
		engine.DateRange{},
		newAuditContext(ctx, "test", nil),
	)
	_, cleanup := data.newEngine(ctx, data.cmd, []*pluginhost.Client{client})
	defer cleanup()
	require.Len(t, data.clients, 1)
	assert.Same(t, client, data.clients[0])
}

func TestWebEstimateSourceUsesCLIPlanMapping(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	plan := "../../testdata/simple-plan.json"
	data := newWebData(
		&cobra.Command{},
		overviewParams{pulumiJSON: plan},
		engine.DateRange{},
		newAuditContext(ctx, "test", nil),
	)
	resources, err := data.estimateResources(ctx)
	require.NoError(t, err)
	expected, err := loadAndMapResources(ctx, plan, nil)
	require.NoError(t, err)
	assert.Equal(t, expected, resources)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	data.gate <- struct{}{}
	_, err = data.estimateResources(canceled)
	require.ErrorIs(t, err, context.Canceled)
	<-data.gate
}

func TestWebEstimateSourceLoadFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	data := newWebData(
		&cobra.Command{},
		overviewParams{pulumiJSON: t.TempDir() + "/missing.json"},
		engine.DateRange{},
		newAuditContext(ctx, "test", nil),
	)
	resources, err := data.estimateResources(ctx)
	require.ErrorContains(t, err, "loading Pulumi plan")
	assert.Nil(t, resources)
	assert.Empty(t, data.gate, "source failure releases the shared query gate")
}

func TestWebInitialPreviewEstimateMembership(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		t.Run(strconv.FormatBool(existing), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			data := newWebData(&cobra.Command{}, overviewParams{}, engine.DateRange{}, nil)
			source := []ingest.StackExportResource{}
			if existing {
				source = append(
					source,
					ingest.StackExportResource{
						URN:    "existing",
						Type:   "aws:ec2/instance:Instance",
						Inputs: map[string]any{"instanceType": "t3.small"},
					},
				)
			}
			data.captureState(source)
			planFile := filepath.Join(t.TempDir(), "plan.json")
			plan := &ingest.PulumiPlan{
				Steps: []ingest.PulumiStep{
					{
						URN:                  "created",
						Type:                 "aws:ec2/instance:Instance",
						Op:                   "create",
						Inputs:               map[string]any{"instanceType": "t3.large", "region": "us-east-1"},
						PropertyDependencies: map[string][]string{"vpcId": {"vpc"}},
					},
				},
			}
			if existing {
				plan.Steps = append(
					plan.Steps,
					ingest.PulumiStep{
						URN:    "existing",
						Type:   "aws:ec2/instance:Instance",
						Op:     "update",
						Inputs: map[string]any{"instanceType": "t3.large"},
					},
				)
			}
			encoded, err := json.Marshal(plan)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(planFile, encoded, 0600))
			params := data.overviewParams()
			params.pulumiJSON = planFile
			_, err = loadPlanForOverview(ctx, params, "", "", nil)
			require.NoError(t, err)
			estimates, err := data.estimateResources(ctx)
			require.NoError(t, err)
			expected, err := ingest.MapResources(plan.GetResources())
			require.NoError(t, err)
			if existing {
				require.Len(t, estimates, 2)
				assert.Equal(t, "existing", estimates[0].ID)
			} else {
				require.Len(t, estimates, 1)
			}
			assert.Equal(t, expected[0], estimates[len(estimates)-1])
			estimates[len(estimates)-1].Properties["instanceType"] = "mutated"
			estimates[len(estimates)-1].Refs["vpcId"][0] = "mutated"
			again, err := data.estimateResources(ctx)
			require.NoError(t, err)
			assert.Equal(t, expected[0], again[len(again)-1])
			deployed, err := data.loadResources(ctx)
			require.NoError(t, err)
			assert.Len(t, deployed, len(source))
		})
	}
}

func TestWebInitialPreviewSourceErrorsAndOwnership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	data := newWebData(&cobra.Command{}, overviewParams{}, engine.DateRange{}, nil)
	data.captureState(nil)
	data.capturePlan(&ingest.PulumiPlan{})
	data.planErr = errors.New("mapping failed")
	_, err := data.estimateResources(ctx)
	require.Error(t, err)
	assert.Empty(t, data.gate)
	resources := []engine.ResourceDescriptor{
		{ID: "r", OldProperties: map[string]any{"nested": map[string]any{"value": "old"}}},
	}
	copied := cloneWebResources(resources)
	copied[0].OldProperties["nested"].(map[string]any)["value"] = "mutated"
	assert.Equal(t, "old", resources[0].OldProperties["nested"].(map[string]any)["value"])
}

func TestWebInitialPreviewReplacesExistingEstimateMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	data := newWebData(&cobra.Command{}, overviewParams{}, engine.DateRange{}, nil)
	data.captureState(
		[]ingest.StackExportResource{
			{
				URN:                  "existing",
				Type:                 "aws:ec2/instance:Instance",
				Inputs:               map[string]any{"region": "us-west-2", "instanceType": "old"},
				PropertyDependencies: map[string][]string{"network": {"deployed-network"}},
			},
		},
	)
	deployed, err := data.loadResources(ctx)
	require.NoError(t, err)
	plan := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				URN:                  "existing",
				Type:                 "gcp:compute/instance:Instance",
				Op:                   "update",
				Provider:             "new-provider",
				Inputs:               map[string]any{"region": "europe-west1", "instanceType": "new"},
				PropertyDependencies: map[string][]string{"network": {"planned-network"}},
				OldState:             &ingest.PulumiState{Inputs: map[string]any{"instanceType": "old"}},
			},
		},
	}
	data.capturePlan(plan)
	mapped, err := ingest.MapResources(plan.GetResources())
	require.NoError(t, err)
	estimates, err := data.estimateResources(ctx)
	require.NoError(t, err)
	require.Len(t, estimates, 1)
	assert.Equal(t, mapped, estimates, "matching IDs use the same full mapped plan descriptor as CLI estimates")
	assert.Equal(t, "gcp", estimates[0].Provider)
	assert.Equal(t, "europe-west1", estimates[0].Properties["region"])
	assert.Equal(t, []string{"planned-network"}, estimates[0].Refs["network"])
	unchanged, err := data.loadResources(ctx)
	require.NoError(t, err)
	assert.Equal(t, deployed, unchanged, "actual/recommendation source keeps deployed provider, region and refs")
}
