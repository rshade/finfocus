package prometheus

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/client-go/kubernetes"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// newTestPlugin stubs the live API so a test never reads the developer kubeconfig.
func newTestPlugin(cfg Config) *Plugin {
	p := New(cfg)
	p.live = func(string) (kubernetes.Interface, string, error) { return nil, "", nil }
	return p
}

const pluginToken = "bearer-token-do-not-leak"

func TestGetStats_RejectsBadWindow(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	p := newTestPlugin(Config{URL: "http://127.0.0.1:1", Token: pluginToken})
	tests := []struct {
		name       string
		start, end *timestamppb.Timestamp
	}{
		{name: "neither bound"},
		{name: "start only", start: timestamppb.New(start)},
		{name: "end only", end: timestamppb.New(end)},
		{name: "end equal to start", start: timestamppb.New(start), end: timestamppb.New(start)},
		{name: "end before start", start: timestamppb.New(end), end: timestamppb.New(start)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := p.GetStats(context.Background(), &pbc.GetStatsRequest{Start: tt.start, End: tt.end})
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
			assert.Contains(t, err.Error(), "historical only")
			assert.NotContains(t, err.Error(), pluginToken)
		})
	}
}

func TestGetStats_HistoricalMode(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		query := r.Form.Get("query")
		mu.Lock()
		queries = append(queries, query)
		mu.Unlock()
		body := `{"status":"success","data":{"resultType":"vector","result":[]}}`
		if strings.Contains(query, "kube_node_labels") && !strings.Contains(query, "cluster=") {
			body = `{"status":"success","data":{"resultType":"vector","result":[` +
				`{"metric":{"cluster":"prod","node":"n1"},"value":[1,"1"]}]}}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	resp, err := newTestPlugin(Config{URL: srv.URL}).GetStats(context.Background(), &pbc.GetStatsRequest{
		Start: timestamppb.New(start),
		End:   timestamppb.New(start.Add(2 * time.Hour)),
		Scope: "prod",
	})
	require.NoError(t, err)
	assert.Equal(t, pbc.StatsMode_STATS_MODE_HISTORICAL, resp.GetMode())

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(queries), 2)
	assert.Contains(t, queries[0], "kube_node_labels")
	assert.NotContains(t, queries[0], `cluster=`)
	assert.Contains(t, strings.Join(queries[1:], "\n"), `cluster="prod"`)
}

func TestInfo_UsageStatsOnly(t *testing.T) {
	t.Parallel()

	info := Info("v0.1.0")
	require.Len(t, info.Capabilities, 1)
	assert.Equal(t, pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS, info.Capabilities[0])
}

func TestSupports_DeclinesPricing(t *testing.T) {
	t.Parallel()

	resp, err := New(Config{}).Supports(context.Background(), &pbc.SupportsRequest{
		Resource: &pbc.ResourceDescriptor{
			Provider:     "aws",
			ResourceType: "aws:ec2/instance:Instance",
			Id:           "i-1",
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.GetSupported())
}

func TestGetStats_UnknownMetric(t *testing.T) {
	t.Parallel()

	srv := emptyPrometheus(t)
	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	resp, err := newTestPlugin(Config{URL: srv.URL}).GetStats(context.Background(), &pbc.GetStatsRequest{
		Start:   timestamppb.New(start),
		End:     timestamppb.New(start.Add(time.Hour)),
		Metrics: []string{pluginsdk.MetricCPUUsage, "gpu_seconds"},
	})
	require.NoError(t, err)
	var found bool
	for _, warning := range resp.GetWarnings() {
		assert.False(t, strings.HasPrefix(warning, "incomplete:"), warning)
		if strings.Contains(warning, "gpu_seconds") {
			found = true
		}
	}
	assert.True(t, found, "warnings: %v", resp.GetWarnings())
}

func TestGetStats_MultipleClusters(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		queries = append(queries, r.Form.Get("query"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"resultType":"vector","result":[`+
			`{"metric":{"cluster":"east","node":"a"},"value":[1,"1"]},`+
			`{"metric":{"cluster":"west","node":"b"},"value":[1,"1"]}]}}`)
	}))
	t.Cleanup(srv.Close)

	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	plugin := newTestPlugin(Config{URL: srv.URL, Token: pluginToken})
	req := &pbc.GetStatsRequest{
		Start: timestamppb.New(start),
		End:   timestamppb.New(start.Add(time.Hour)),
	}
	_, err := plugin.GetStats(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "east")
	assert.Contains(t, err.Error(), "west")
	assert.NotContains(t, err.Error(), pluginToken)
	assert.NotEqual(t, codes.Unavailable, status.Code(err))

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, queries)
	for _, query := range queries {
		assert.NotContains(t, query, "container_cpu")
	}
}

func TestGetStats_DownServerAndMissingURL(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	window := &pbc.GetStatsRequest{Start: timestamppb.New(start), End: timestamppb.New(end)}

	t.Run("down server", func(t *testing.T) {
		t.Parallel()
		down := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := down.URL
		down.Close()

		// New logs to stderr. This subtest leaves that logger in place.
		_, err := newTestPlugin(Config{URL: url, Token: pluginToken}).GetStats(context.Background(), window)
		require.Error(t, err)
		assert.NotEqual(t, codes.OK, status.Code(err))
		assert.NotContains(t, err.Error(), pluginToken)
	})

	t.Run("url credentials stay out of errors and logs", func(t *testing.T) {
		t.Parallel()
		down := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		host := strings.TrimPrefix(down.URL, "http://")
		down.Close()

		var stderr strings.Builder
		p := newTestPlugin(Config{URL: "http://urluser:urlpass@" + host + "/?api_key=querysecret"})
		p.logger = zerolog.New(&stderr)
		_, err := p.GetStats(context.Background(), window)
		require.Error(t, err)
		for _, secret := range []string{"urluser", "urlpass", "querysecret"} {
			assert.NotContains(t, err.Error(), secret)
			assert.NotContains(t, stderr.String(), secret)
		}
	})

	t.Run("missing url", func(t *testing.T) {
		t.Parallel()
		_, err := newTestPlugin(Config{Token: pluginToken}).GetStats(context.Background(), window)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "FINFOCUS_PROMETHEUS_URL")
		assert.NotContains(t, err.Error(), pluginToken)
	})

	t.Run("error body echoes the token", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			gotAuth = r.Header.Get("Authorization")
			mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"status":"error","errorType":"bad_data","error":"rejected `+
				r.Header.Get("Authorization")+`"}`)
		}))
		t.Cleanup(srv.Close)

		var stderr strings.Builder
		p := newTestPlugin(Config{URL: srv.URL, Token: pluginToken})
		p.logger = zerolog.New(&stderr)
		_, err := p.GetStats(context.Background(), window)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), pluginToken)
		assert.NotContains(t, stderr.String(), pluginToken)
		assert.Contains(t, stderr.String(), "prometheus query failed")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, "Bearer "+pluginToken, gotAuth)
	})
}

func emptyPrometheus(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGetStats_RedirectDoesNotForwardToken(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var leaked []string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		leaked = append(leaked, r.Header.Get("Authorization"))
		mu.Unlock()
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	t.Cleanup(target.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	window := &pbc.GetStatsRequest{Start: timestamppb.New(start), End: timestamppb.New(start.Add(time.Hour))}
	_, err := newTestPlugin(Config{URL: origin.URL, Token: pluginToken}).GetStats(context.Background(), window)
	require.Error(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, leaked, "the redirect must reach the second origin")
	for _, got := range leaked {
		assert.Empty(t, got, "token sent to another origin")
	}
}

type headerRecorder struct{ auth string }

func (h *headerRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	h.auth = req.Header.Get("Authorization")
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
}

func TestBearerRoundTripper_OnlyConfiguredOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		target   string
		wantAuth bool
	}{
		{name: "same origin", target: "https://prom.example:9090/api/v1/query", wantAuth: true},
		{name: "other host", target: "https://evil.example:9090/api/v1/query"},
		{name: "other port", target: "https://prom.example:9091/api/v1/query"},
		{name: "downgrade to http", target: "http://prom.example:9090/api/v1/query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := &headerRecorder{}
			rt, err := newBearerRoundTripper(rec, "https://prom.example:9090", pluginToken)
			require.NoError(t, err)
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, tt.target, nil)
			require.NoError(t, err)
			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			if tt.wantAuth {
				assert.Equal(t, "Bearer "+pluginToken, rec.auth)
			} else {
				assert.Empty(t, rec.auth)
			}
		})
	}
}
