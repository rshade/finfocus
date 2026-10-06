package prometheus

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/prometheus/collect"
)

// PluginName is the registry and binary name suffix.
const PluginName = "prometheus"

// historicalOnly is the rejection for a stats request that is not a window.
const historicalOnly = "prometheus usage source is historical only"

// Plugin serves historical usage stats. It embeds BasePlugin so the required
// cost methods exist, and advertises usage stats only.
type Plugin struct {
	*pluginsdk.BasePlugin

	cfg    Config
	logger zerolog.Logger
	// live opens a client-go client for the request scope. Tests replace it.
	// Nil skips the live API. The default is openKubeconfig.
	live func(scope string) (kubernetes.Interface, string, error)
}

// New builds the plugin. cfg comes from LoadConfig. Logs go to stderr; stdout
// is the plugin port handshake.
func New(cfg Config) *Plugin {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", PluginName).Logger()
	return &Plugin{
		BasePlugin: pluginsdk.NewBasePlugin(PluginName),
		cfg:        cfg,
		logger:     logger,
		live:       openKubeconfig,
	}
}

// Info declares usage stats and nothing else. inferCapabilities would also
// advertise pricing because BasePlugin implements the cost methods.
func Info(version string) *pluginsdk.PluginInfo {
	return pluginsdk.NewPluginInfo(PluginName, version,
		pluginsdk.WithCapabilities(pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS),
	)
}

// GetStats answers a window with historical CPU and memory usage.
// A request missing either bound, or whose end is not after its start, is
// rejected before any query.
func (p *Plugin) GetStats(ctx context.Context, req *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error) {
	if err := validateWindow(req); err != nil {
		return nil, err
	}
	if p.cfg.URL == "" {
		return nil, status.Error(codes.FailedPrecondition, missingPrometheusURL)
	}
	live, apiHost, kubeErr := p.openLive(req.GetScope())
	client, err := p.apiClient()
	if err != nil {
		return nil, p.fail(err)
	}
	selector := maps.Clone(req.GetSelector())
	namespace := selector["namespace"]
	delete(selector, "namespace")
	resp, err := collect.Collect(ctx, client, req.GetStart().AsTime(), req.GetEnd().AsTime(), collect.Options{
		Namespace:     namespace,
		Cluster:       req.GetScope(),
		PodLabels:     selector,
		Live:          live,
		APIHost:       apiHost,
		KubeconfigErr: kubeErr,
	})
	if status.Code(err) == codes.InvalidArgument {
		return nil, err
	}
	if err != nil {
		return nil, p.fail(err)
	}
	resp.Mode = pbc.StatsMode_STATS_MODE_HISTORICAL
	applyMetricFilter(resp, req.GetMetrics())
	return resp, nil
}

// Supports declines every pricing request. This source reports usage only.
func (p *Plugin) Supports(context.Context, *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: false, Reason: "prometheus plugin reports usage only"}, nil
}

func (p *Plugin) openLive(scope string) (kubernetes.Interface, string, error) {
	if p.live == nil {
		return nil, "", nil
	}
	return p.live(scope)
}

func (p *Plugin) fail(err error) error {
	redacted := redact(err, p.cfg.Token)
	p.logger.Warn().Err(redacted).Str("operation", "GetStats").Msg("prometheus query failed")
	return redacted
}

func (p *Plugin) apiClient() (v1.API, error) {
	roundTripper := http.DefaultTransport
	if p.cfg.Token != "" {
		roundTripper = bearerRoundTripper{base: roundTripper, token: p.cfg.Token}
	}
	client, err := api.NewClient(api.Config{Address: p.cfg.URL, RoundTripper: roundTripper})
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (b bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(cloned)
}

func validateWindow(req *pbc.GetStatsRequest) error {
	start, end := req.GetStart(), req.GetEnd()
	if start == nil || end == nil || !end.AsTime().After(start.AsTime()) {
		return status.Error(codes.InvalidArgument, historicalOnly)
	}
	return nil
}

func redact(err error, token string) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if token != "" {
		message = strings.ReplaceAll(message, token, "redacted")
	}
	code := codes.Unavailable
	switch {
	case errors.Is(err, context.Canceled):
		code = codes.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		code = codes.DeadlineExceeded
	}
	return status.Error(code, message)
}

func servedMetric(name string) bool {
	switch name {
	case pluginsdk.MetricCPUUsage, pluginsdk.MetricMemUsage,
		pluginsdk.MetricCPUAllocatable, pluginsdk.MetricMemAllocatable:
		return true
	default:
		return false
	}
}

// applyMetricFilter keeps requested metrics. An empty list keeps this
// source's defaults. Unknown names are warned once and do not use the
// incomplete: prefix.
func applyMetricFilter(resp *pbc.GetStatsResponse, requested []string) {
	if len(requested) == 0 {
		return
	}
	wanted := make(map[string]bool, len(requested))
	reported := make(map[string]bool, len(requested))
	for _, name := range requested {
		if servedMetric(name) {
			wanted[name] = true
			continue
		}
		if !reported[name] {
			reported[name] = true
			resp.Warnings = append(resp.Warnings, "unknown metric "+strconv.Quote(name)+" ignored")
		}
	}
	rows := make([]*pbc.UsageRow, 0, len(resp.GetRows()))
	for _, row := range resp.GetRows() {
		if wanted[row.GetMetric()] {
			rows = append(rows, row)
		}
	}
	resp.Rows = rows
}
