package webui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func clusterReportFixture() engine.ClusterReport {
	idle := 40.0
	return engine.ClusterReport{
		Mode:     engine.ModeRunRate,
		Period:   "monthly",
		Currency: "USD",
		GroupBy:  "namespace",
		Total:    100,
		Idle:     &idle,
		Groups: []engine.ClusterGroup{
			{Key: "payments", CPUCost: 30, MemCost: 30, TotalCost: 60, Rows: 2},
			{Key: "__idle__", CPUCost: 40, TotalCost: 40, Rows: 2},
		},
		Capacity: []engine.CapacityGroup{},
		Priced: []engine.PricedSummary{
			{Kind: "node", ID: "n1", ResourceType: "aws:ec2/instance:Instance", Monthly: 100, Priced: true},
		},
		Policy: engine.ClusterPolicy{
			Source: "built-in defaults", Digest: "0123456789abcdef", Effective: json.RawMessage(`{"idle":"share"}`),
		},
	}
}

func newClusterServer(
	t *testing.T,
	query func(context.Context, ClusterQuery) (engine.ClusterReport, error),
	policy func(context.Context, ClusterPolicyQuery) (engine.ClusterPolicy, error),
) *Server {
	t.Helper()
	session := NewSession(context.Background(), SessionOptions{Cluster: query, ClusterPolicy: policy})
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
	return newRunningServer(t, Options{Session: session})
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

func TestClusterQuery_ReturnsReportAndDisplay(t *testing.T) {
	t.Parallel()
	var got ClusterQuery
	s := newClusterServer(t, func(_ context.Context, q ClusterQuery) (engine.ClusterReport, error) {
		got = q
		return clusterReportFixture(), nil
	}, nil)

	resp := overviewRequest(t, s, http.MethodPost, "/api/cluster/query", `{"context":"prod","namespace":"payments",`+
		`"selector":["team=web"],"groupBy":"label:team","from":"2026-11-01","to":"2026-11-08",`+
		`"usageSource":"prometheus","allocator":"kubernetes","policy":"{\"idle\":\"separate\"}"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body ClusterResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.Equal(t, ClusterQuery{
		Context: "prod", Namespace: "payments", Selector: []string{"team=web"}, GroupBy: "label:team",
		From: "2026-11-01", To: "2026-11-08", UsageSource: "prometheus", Allocator: "kubernetes",
		Policy: `{"idle":"separate"}`,
	}, got)
	assert.Equal(t, clusterStatusOK, body.Status)
	assert.Equal(t, clusterReportFixture(), body.ClusterReport)
	assert.Equal(t, "$100.00 USD", body.Display.Total)
	assert.Equal(t, "40.0%", body.Display.IdleShare)
	assert.Equal(t, "built-in defaults · 0123456789ab", body.Display.Policy)
	require.Len(t, body.Display.Groups, 2)
	assert.Equal(t, "60.0%", body.Display.Groups[0].Share)
}

func TestClusterQuery_Unavailable(t *testing.T) {
	t.Parallel()
	for _, capability := range []string{"usage_stats", "allocation"} {
		t.Run(capability, func(t *testing.T) {
			t.Parallel()
			s := newClusterServer(t, func(context.Context, ClusterQuery) (engine.ClusterReport, error) {
				return engine.ClusterReport{}, &engine.ClusterPluginMissingError{Capability: capability}
			}, nil)

			resp := overviewRequest(t, s, http.MethodPost, "/api/cluster/query", `{}`)
			require.Equal(t, http.StatusOK, resp.StatusCode, "a missing plugin is a status, never a server error")
			assert.Equal(t, map[string]any{
				"status":     "unavailable",
				"code":       "cluster_plugin_missing",
				"capability": capability,
				"error":      (&engine.ClusterPluginMissingError{Capability: capability}).Error(),
			}, decodeBody(t, resp))
		})
	}
}

func TestClusterQuery_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		err      error
		nilQuery bool
		status   int
		code     string
		message  string
	}{
		{name: "input error keeps the CLI text", body: `{"groupBy":"daily"}`,
			err:    &engine.ClusterInputError{Err: errors.New(`invalid --group-by "daily"`)},
			status: http.StatusBadRequest, code: "invalid_cluster_query", message: `invalid --group-by "daily"`},
		{name: "plugin failure is generic", body: `{}`,
			err:    errors.New("allocate: rpc error: code = Internal desc = secret plugin detail"),
			status: http.StatusBadGateway, code: "cluster_unavailable", message: "cluster data unavailable"},
		{name: "bad JSON", body: `{`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "a policy path is refused", body: `{"policyPath":"/etc/passwd"}`,
			status: http.StatusBadRequest, code: "invalid_request"},
		{name: "no callback", body: `{}`, nilQuery: true,
			status: http.StatusServiceUnavailable, code: "not_ready", message: "cluster data is loading"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			query := func(context.Context, ClusterQuery) (engine.ClusterReport, error) {
				called = true
				return engine.ClusterReport{}, tt.err
			}
			if tt.nilQuery {
				query = nil
			}
			s := newClusterServer(t, query, nil)

			resp := overviewRequest(t, s, http.MethodPost, "/api/cluster/query", tt.body)
			require.Equal(t, tt.status, resp.StatusCode)
			body := decodeBody(t, resp)
			assert.Equal(t, tt.code, body["code"])
			if tt.message != "" {
				assert.Equal(t, tt.message, body["error"])
			}
			if tt.err == nil {
				assert.False(t, called, "a rejected request never reaches the session")
			}
		})
	}
}

func TestClusterQuery_NotReadyBeforeEngine(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{
		Cluster: func(context.Context, ClusterQuery) (engine.ClusterReport, error) {
			t.Error("the callback must not run before the session has an engine")
			return engine.ClusterReport{}, nil
		},
	})
	s := newRunningServer(t, Options{Session: session})

	resp := overviewRequest(t, s, http.MethodPost, "/api/cluster/query", `{}`)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "not_ready", decodeBody(t, resp)["code"])
}

func TestClusterQuery_HasNoGenericQueryDeadline(t *testing.T) {
	t.Parallel()
	var hasDeadline bool
	s := newClusterServer(t, func(ctx context.Context, _ ClusterQuery) (engine.ClusterReport, error) {
		_, hasDeadline = ctx.Deadline()
		return clusterReportFixture(), nil
	}, nil)

	resp := overviewRequest(t, s, http.MethodPost, "/api/cluster/query", `{}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, hasDeadline, "each cluster plugin call carries its own deadline, as on the CLI")
}

func TestClusterPolicy(t *testing.T) {
	t.Parallel()
	var got ClusterPolicyQuery
	s := newClusterServer(t, nil, func(_ context.Context, q ClusterPolicyQuery) (engine.ClusterPolicy, error) {
		got = q
		return clusterReportFixture().Policy, nil
	})

	resp := overviewRequest(t, s, http.MethodPost, "/api/cluster/policy", `{"allocator":"kubernetes","policy":"{}"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body ClusterPolicyResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, ClusterPolicyQuery{Allocator: "kubernetes", Policy: "{}"}, got)
	assert.Equal(t, clusterStatusOK, body.Status)
	assert.Equal(t, clusterReportFixture().Policy, body.ClusterPolicy)
	assert.Equal(t, ClusterPolicyDisplay{Source: "built-in defaults", Digest: "0123456789ab"}, body.Display)
}

func TestClusterPolicy_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "no allocator", err: &engine.ClusterPluginMissingError{Capability: "allocation"},
			status: http.StatusOK, code: "cluster_plugin_missing"},
		{name: "rejected policy", err: &engine.ClusterInputError{Err: errors.New("allocator rejected the policy")},
			status: http.StatusBadRequest, code: "invalid_cluster_query"},
		{name: "plugin failure", err: errors.New("boom"), status: http.StatusBadGateway, code: "cluster_unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := newClusterServer(t, nil, func(context.Context, ClusterPolicyQuery) (engine.ClusterPolicy, error) {
				return engine.ClusterPolicy{}, tt.err
			})
			resp := overviewRequest(t, s, http.MethodPost, "/api/cluster/policy", `{}`)
			require.Equal(t, tt.status, resp.StatusCode)
			assert.Equal(t, tt.code, decodeBody(t, resp)["code"])
		})
	}
}
