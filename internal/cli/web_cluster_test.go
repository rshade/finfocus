package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/webui"
)

// clusterNodePricingClient prices cluster nodes through the real engine: n1 at
// 70 and n2 at 30, monthly and over any window. Other nodes get no price.
type clusterNodePricingClient struct{ includeDismissedClient }

func (*clusterNodePricingClient) GetProjectedCost(
	_ context.Context, in *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	out := &proto.GetProjectedCostResponse{}
	for _, resource := range in.Resources {
		out.Results = append(out.Results, &proto.CostResult{
			Currency: "USD", MonthlyCost: clusterNodePrices[resource.ID],
		})
	}
	return out, nil
}

func (*clusterNodePricingClient) GetActualCost(
	_ context.Context, in *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	out := &proto.GetActualCostResponse{}
	for _, id := range in.ResourceIDs {
		out.Results = append(out.Results, &proto.ActualCostResult{Currency: "USD", TotalCost: clusterNodePrices[id]})
	}
	return out, nil
}

// clusterWebFixture is a running dashboard whose session answers cluster
// queries through webData with one usage-source and allocator plugin and an
// engine that prices through clusterNodePricingClient.
type clusterWebFixture struct {
	data   *webData
	plugin *fakeClusterPlugin
	srv    *webui.Server
	client *http.Client
}

func newClusterWebFixture(t *testing.T, plugin *fakeClusterPlugin) *clusterWebFixture {
	t.Helper()
	ctx := context.Background()
	data := newWebData(&cobra.Command{}, overviewParams{cfg: &config.Config{}}, engine.DateRange{}, nil)
	data.eng = engine.New([]*pluginhost.Client{{Name: "pricing", API: &clusterNodePricingClient{}}}, nil)
	data.clients = []*pluginhost.Client{startClusterPlugin(t, "kubernetes", plugin,
		pluginhost.CapabilityUsageStats, pluginhost.CapabilityAllocation)}

	session := webui.NewSession(ctx, webui.SessionOptions{Cluster: data.cluster, ClusterPolicy: data.clusterPolicy})
	session.SetData(data.eng, nil, engine.DateRange{}, "dev")
	srv, err := webui.New(webui.Options{Session: session})
	require.NoError(t, err)
	require.NoError(t, srv.Listen())
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.NoError(t, srv.Shutdown(shutdownCtx))
	})
	return &clusterWebFixture{data: data, plugin: plugin, srv: srv, client: authedWebClient(t, srv)}
}

// post sends body to path and returns the status and the decoded answer.
func (f *clusterWebFixture) post(t *testing.T, path string, body any) (int, map[string]any) {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, f.srv.BaseURL()+path, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", f.srv.BaseURL())
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := f.client.Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded), string(raw))
	return resp.StatusCode, decoded
}

// cliClusterJSON is what `cost cluster --output json` prints for the query,
// built from the fixture's plugins and engine.
func cliClusterJSON(t *testing.T, f *clusterWebFixture, query clusterQuery) map[string]any {
	t.Helper()
	ctx := context.Background()
	if query.groupBy == "" {
		query.groupBy = defaultClusterGroupBy // the CLI flag's default
	}
	in, err := parseClusterQuery(ctx, query)
	require.NoError(t, err)
	report, err := buildClusterReport(ctx, f.data.clients, f.data.eng, in)
	require.NoError(t, err)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	require.NoError(t, renderClusterResult(cmd, outputFormatJSON, report))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
	return decoded
}

// TestWebClusterMatchesCLIJSON is the parity check for `POST /api/cluster/query`:
// apart from status and display, the body is the CLI's JSON document for the
// same arguments.
//
//nolint:paralleltest // isolateClusterTestConfig sets FINFOCUS_HOME and resets the process-wide config
func TestWebClusterMatchesCLIJSON(t *testing.T) {
	isolateClusterTestConfig(t)
	from, to := clusterWindowDays(t)

	tests := []struct {
		name   string
		plugin *fakeClusterPlugin
		web    webui.ClusterQuery
		cli    clusterQuery
		check  func(t *testing.T, body map[string]any)
	}{
		{name: "run rate", plugin: &fakeClusterPlugin{},
			check: func(t *testing.T, body map[string]any) {
				assert.InDelta(t, 100, body["total"], 1e-9)
				assert.InDelta(t, 40, body["idle"], 1e-9)
			}},
		{name: "window", plugin: &fakeClusterPlugin{},
			web: webui.ClusterQuery{From: from, To: to}, cli: clusterQuery{from: from, to: to},
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, engine.ModeHistorical, body["mode"])
			}},
		{name: "selector scoped by label", plugin: &fakeClusterPlugin{},
			web: webui.ClusterQuery{Selector: []string{"team=web"}, GroupBy: "controller"},
			cli: clusterQuery{selectors: []string{"team=web"}, groupBy: "controller"},
			check: func(t *testing.T, body map[string]any) {
				assert.NotContains(t, body, "idle")
				assert.Equal(t, true, body["selector_scoped"])
			}},
		{name: "incomplete", plugin: &fakeClusterPlugin{unpriced: true, warnings: []string{
			"incomplete: ns/p-1: gap in cpu_usage", "incomplete: ns/p-2: gap in cpu_usage",
		}},
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, true, body["incomplete"])
				display, ok := body["display"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, []any{"gap in cpu_usage for 2 resources"}, display["incompleteCauses"])
				assert.Equal(t, []any{"n3: priced at $0; treated as unpriced"}, display["unpriced"])
			}},
		{name: "policy text", plugin: &fakeClusterPlugin{},
			web: webui.ClusterQuery{Policy: `{ "idle": "separate" }`},
			cli: clusterQuery{policyText: new(`{ "idle": "separate" }`)},
			check: func(t *testing.T, body map[string]any) {
				policy, ok := body["policy"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, requestPolicySource, policy["source"])
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClusterWebFixture(t, tt.plugin)
			status, body := f.post(t, "/api/cluster/query", tt.web)
			require.Equal(t, http.StatusOK, status, body)
			assert.Equal(t, "ok", body["status"])
			require.Contains(t, body, "display")
			tt.check(t, body)

			delete(body, "status")
			delete(body, "display")
			assert.Equal(t, cliClusterJSON(t, f, tt.cli), body)
		})
	}
}

//nolint:paralleltest // isolateClusterTestConfig sets FINFOCUS_HOME and resets the process-wide config
func TestWebClusterAnswers(t *testing.T) {
	isolateClusterTestConfig(t)

	t.Run("policy matches --show-policy", func(t *testing.T) {
		f := newClusterWebFixture(t, &fakeClusterPlugin{})
		status, body := f.post(t, "/api/cluster/policy", webui.ClusterPolicyQuery{})
		require.Equal(t, http.StatusOK, status, body)

		in, err := parseClusterQuery(context.Background(), webClusterQuery(webui.ClusterQuery{}))
		require.NoError(t, err)
		policy, err := buildClusterPolicy(context.Background(), f.data.clients, in)
		require.NoError(t, err)
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		require.NoError(t, renderPolicy(cmd, outputFormatJSON, policy))
		var want map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &want))

		assert.Equal(t, "ok", body["status"])
		assert.Equal(t, map[string]any{"source": "built-in defaults", "digest": "0123456789ab"}, body["display"])
		delete(body, "status")
		delete(body, "display")
		assert.Equal(t, want, body)
		stats := f.plugin.statsCallCount()
		assert.Zero(t, stats)
	})
	t.Run("input error carries the CLI message", func(t *testing.T) {
		f := newClusterWebFixture(t, &fakeClusterPlugin{})
		status, body := f.post(t, "/api/cluster/query", webui.ClusterQuery{Selector: []string{"team"}})
		require.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_cluster_query", body["code"])
		assert.Equal(t, `invalid --selector "team": want key=value`, body["error"])
	})
	t.Run("missing allocator is unavailable", func(t *testing.T) {
		f := newClusterWebFixture(t, &fakeClusterPlugin{})
		f.data.clients = []*pluginhost.Client{startClusterPlugin(t, "usage", &fakeClusterPlugin{},
			pluginhost.CapabilityUsageStats)}
		status, body := f.post(t, "/api/cluster/query", webui.ClusterQuery{})
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "unavailable", body["status"])
		assert.Equal(t, pluginhost.CapabilityAllocation, body["capability"])
	})
	t.Run("discovered policy is named by tier, never by path", func(t *testing.T) {
		home := config.ResolveConfigDir()
		policyFile := filepath.Join(home, config.AllocationPolicyFile)
		require.NoError(t, os.WriteFile(policyFile, []byte(`{"idle":"separate"}`), 0o600))
		t.Cleanup(func() { _ = os.Remove(policyFile) })
		f := newClusterWebFixture(t, &fakeClusterPlugin{})
		status, body := f.post(t, "/api/cluster/query", webui.ClusterQuery{})
		require.Equal(t, http.StatusOK, status, body)
		policy, ok := body["policy"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, config.GlobalAllocationPolicyLabel, policy["source"])
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), home)
	})
}
