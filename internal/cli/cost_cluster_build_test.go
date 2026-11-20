package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rshade/ax-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func clusterWindowDays(t *testing.T) (string, string) {
	t.Helper()
	today := time.Now().UTC().Truncate(24 * time.Hour)
	return today.AddDate(0, 0, -14).Format(time.DateOnly), today.AddDate(0, 0, -7).Format(time.DateOnly)
}

// TestParseClusterQuery stays sequential: with no policy text it resolves the
// policy from FINFOCUS_HOME, which isolateClusterTestConfig sets.
//
//nolint:paralleltest // isolateClusterTestConfig sets FINFOCUS_HOME and resets the process-wide config
func TestParseClusterQuery(t *testing.T) {
	isolateClusterTestConfig(t)
	from, to := clusterWindowDays(t)
	policyFile := filepath.Join(t.TempDir(), "policy.hujson")
	require.NoError(t, os.WriteFile(policyFile, []byte(`{ "idle": "separate" }`), 0o600))

	t.Run("defaults", func(t *testing.T) {
		in, err := parseClusterQuery(context.Background(), clusterQuery{groupBy: "namespace"})
		require.NoError(t, err)
		assert.Equal(t, "namespace", in.groupBy)
		assert.True(t, in.from.IsZero())
		assert.True(t, in.to.IsZero())
		assert.Empty(t, in.selector)
		assert.Empty(t, in.policy.Source, "no file and no text means the allocator's defaults")
		assert.Nil(t, in.policy.JSON)
	})
	t.Run("values", func(t *testing.T) {
		in, err := parseClusterQuery(context.Background(), clusterQuery{
			context: "prod", namespace: "payments", selectors: []string{"team=web", "tier=api"},
			groupBy: "label:team", from: from, to: to, usageSource: "kubernetes", allocator: "kubernetes",
		})
		require.NoError(t, err)
		assert.Equal(t, "prod", in.scope)
		assert.Equal(t, "payments", in.namespace)
		assert.Equal(t, map[string]string{"team": "web", "tier": "api"}, in.selector)
		assert.Equal(t, "label:team", in.groupBy)
		assert.Equal(t, from, in.from.Format(time.DateOnly))
		assert.Equal(t, to, in.to.Format(time.DateOnly))
		assert.Equal(t, "kubernetes", in.usageSource)
		assert.Equal(t, "kubernetes", in.allocator)
	})
	t.Run("policy file", func(t *testing.T) {
		in, err := parseClusterQuery(context.Background(), clusterQuery{groupBy: "namespace", policyPath: policyFile})
		require.NoError(t, err)
		assert.Equal(t, policyFile, in.policy.Source)
		assert.JSONEq(t, `{"idle":"separate"}`, string(in.policy.JSON))
	})
	t.Run("policy text", func(t *testing.T) {
		text := "{\n  // split idle out\n  \"idle\": \"separate\",\n}"
		in, err := parseClusterQuery(context.Background(), clusterQuery{groupBy: "namespace", policyText: &text})
		require.NoError(t, err)
		assert.Equal(t, requestPolicySource, in.policy.Source)
		assert.JSONEq(t, `{"idle":"separate"}`, string(in.policy.JSON))
	})
	t.Run("discovered policy named by tier", func(t *testing.T) {
		global := filepath.Join(os.Getenv("FINFOCUS_HOME"), config.AllocationPolicyFile)
		require.NoError(t, os.WriteFile(global, []byte(`{"idle":"share"}`), 0o600))
		t.Cleanup(func() { _ = os.Remove(global) })

		in, err := parseClusterQuery(context.Background(), clusterQuery{groupBy: "namespace", policyByTier: true})
		require.NoError(t, err)
		assert.Equal(t, config.GlobalAllocationPolicyLabel, in.policy.Source)
		in, err = parseClusterQuery(context.Background(), clusterQuery{groupBy: "namespace"})
		require.NoError(t, err)
		assert.Equal(t, global, in.policy.Source, "the CLI keeps the path")
	})
	t.Run("blank policy text uses the discovered policy", func(t *testing.T) {
		blank := "  \n"
		in, err := parseClusterQuery(context.Background(), clusterQuery{groupBy: "namespace", policyText: &blank})
		require.NoError(t, err)
		assert.Empty(t, in.policy.Source)
	})
}

//nolint:paralleltest // isolateClusterTestConfig sets FINFOCUS_HOME and resets the process-wide config
func TestParseClusterQuery_InputErrors(t *testing.T) {
	isolateClusterTestConfig(t)
	from, to := clusterWindowDays(t)
	badPolicy := `{ "idle": `

	tests := []struct {
		name  string
		query clusterQuery
		want  string
	}{
		{name: "group-by", query: clusterQuery{groupBy: "daily"}, want: "invalid --group-by"},
		{name: "explicit empty group-by", query: clusterQuery{}, want: `invalid --group-by ""`},
		{name: "selector", query: clusterQuery{groupBy: "namespace", selectors: []string{"team"}},
			want: `invalid --selector "team": want key=value`},
		{
			name:  "to without from",
			query: clusterQuery{groupBy: "namespace", to: to},
			want:  "--from is required when --to is set",
		},
		{
			name:  "reversed window",
			query: clusterQuery{groupBy: "namespace", from: to, to: from},
			want:  "'to' date must be after 'from' date",
		},
		{
			name:  "unparseable date",
			query: clusterQuery{groupBy: "namespace", from: "notadate"},
			want:  "unable to parse date",
		},
		{name: "policy text syntax", query: clusterQuery{groupBy: "namespace", policyText: &badPolicy},
			want: "allocation policy request body is not valid HuJSON"},
		{
			name:  "unreadable policy file",
			query: clusterQuery{groupBy: "namespace", policyPath: filepath.Join(t.TempDir(), "absent")},
			want:  "cannot read allocation policy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseClusterQuery(context.Background(), tt.query)
			input, ok := errors.AsType[*engine.ClusterInputError](err)
			require.True(t, ok, "want a ClusterInputError, got %T: %v", err, err)
			assert.Contains(t, input.Error(), tt.want)
		})
	}
}

func clusterInputFor(t *testing.T, mutate func(*clusterInput)) clusterInput {
	t.Helper()
	in := clusterInput{groupBy: "namespace"}
	if mutate != nil {
		mutate(&in)
	}
	return in
}

func TestBuildClusterReport(t *testing.T) {
	t.Parallel()

	allCaps := []string{pluginhost.CapabilityUsageStats, pluginhost.CapabilityAllocation}
	from := time.Now().UTC().Truncate(time.Hour).Add(-48 * time.Hour)
	to := from.Add(24 * time.Hour)

	tests := []struct {
		name       string
		mutate     func(*clusterInput)
		wantMode   string
		wantTotal  float64
		wantIdle   *float64
		wantGroups []string
	}{
		{name: "run rate", wantMode: engine.ModeRunRate, wantTotal: 100, wantIdle: new(40.0),
			wantGroups: []string{"ns-n1", "ns-n2", "__idle__"}},
		{
			name:       "window",
			mutate:     func(in *clusterInput) { in.from, in.to = from, to },
			wantMode:   engine.ModeHistorical,
			wantTotal:  100,
			wantIdle:   new(40.0),
			wantGroups: []string{"ns-n1", "ns-n2", "__idle__"},
		},
		{name: "namespace scoped", mutate: func(in *clusterInput) { in.namespace = "ns-n1" },
			wantMode: engine.ModeRunRate, wantTotal: 60, wantGroups: []string{"ns-n1", "ns-n2"}},
		{name: "grouped by node", mutate: func(in *clusterInput) { in.groupBy = "node" },
			wantMode: engine.ModeRunRate, wantTotal: 100, wantIdle: new(40.0), wantGroups: []string{"n1", "n2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := startClusterPlugin(t, "kubernetes", &fakeClusterPlugin{}, allCaps...)
			report, err := buildClusterReport(context.Background(), []*pluginhost.Client{client}, clusterPricer{},
				clusterInputFor(t, tt.mutate))
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, report.Mode)
			assert.InDelta(t, tt.wantTotal, report.Total, 1e-9)
			if tt.wantIdle == nil {
				assert.Nil(t, report.Idle)
			} else {
				require.NotNil(t, report.Idle)
				assert.InDelta(t, *tt.wantIdle, *report.Idle, 1e-9)
			}
			keys := make([]string, 0, len(report.Groups))
			for _, g := range report.Groups {
				keys = append(keys, g.Key)
			}
			assert.ElementsMatch(t, tt.wantGroups, keys)
			assert.Equal(t, "built-in defaults", report.Policy.Source)
			assert.Equal(t, "0123456789abcdef0123", report.Policy.Digest)
			assert.NotNil(t, report.Capacity)
		})
	}
}

func TestBuildClusterReport_Errors(t *testing.T) {
	t.Parallel()

	usageOnly := []string{pluginhost.CapabilityUsageStats}
	allocOnly := []string{pluginhost.CapabilityAllocation}
	allCaps := append(append([]string{}, usageOnly...), allocOnly...)
	rejected := config.AllocationPolicy{JSON: []byte(`{"bogus":1}`), Source: requestPolicySource}

	tests := []struct {
		name        string
		clients     func(t *testing.T, plugin *fakeClusterPlugin) []*pluginhost.Client
		mutate      func(*clusterInput)
		wantMissing string
		wantInput   string
		wantStats   int
	}{
		{
			name: "no usage source",
			clients: func(t *testing.T, p *fakeClusterPlugin) []*pluginhost.Client {
				return []*pluginhost.Client{startClusterPlugin(t, "alloc", p, allocOnly...)}
			},
			wantMissing: pluginhost.CapabilityUsageStats,
		},
		{
			name: "no allocator",
			clients: func(t *testing.T, p *fakeClusterPlugin) []*pluginhost.Client {
				return []*pluginhost.Client{startClusterPlugin(t, "usage", p, usageOnly...)}
			},
			wantMissing: pluginhost.CapabilityAllocation,
		},
		{
			name: "unknown allocator name",
			clients: func(t *testing.T, p *fakeClusterPlugin) []*pluginhost.Client {
				return []*pluginhost.Client{startClusterPlugin(t, "kubernetes", p, allCaps...)}
			},
			mutate:    func(in *clusterInput) { in.allocator = "opencost" },
			wantInput: `plugin "opencost" is not installed or lacks allocation (available: kubernetes)`,
		},
		{
			name: "several allocators",
			clients: func(t *testing.T, p *fakeClusterPlugin) []*pluginhost.Client {
				return []*pluginhost.Client{
					startClusterPlugin(t, "kubernetes", p, allCaps...),
					startClusterPlugin(t, "opencost", p, allocOnly...),
				}
			},
			wantInput: "several plugins provide allocation (kubernetes, opencost); choose one with --allocator",
		},
		{
			name: "policy rejected before usage is gathered",
			clients: func(t *testing.T, p *fakeClusterPlugin) []*pluginhost.Client {
				return []*pluginhost.Client{startClusterPlugin(t, "kubernetes", p, allCaps...)}
			},
			mutate: func(in *clusterInput) { in.policy = rejected },
			wantInput: `allocator kubernetes rejected the allocation policy request body: ` +
				`allocation policy: unknown field "bogus"`,
		},
		{
			name: "usage source rejects the request",
			clients: func(t *testing.T, p *fakeClusterPlugin) []*pluginhost.Client {
				return []*pluginhost.Client{startClusterPlugin(t, "kubernetes", p, allCaps...)}
			},
			mutate:    func(in *clusterInput) { in.scope = "missing" },
			wantInput: `usage source kubernetes rejected the request: context "missing" not found`,
			wantStats: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plugin := &fakeClusterPlugin{}
			_, err := buildClusterReport(context.Background(), tt.clients(t, plugin), clusterPricer{},
				clusterInputFor(t, tt.mutate))
			require.Error(t, err)
			if tt.wantMissing != "" {
				missing, ok := errors.AsType[*engine.ClusterPluginMissingError](err)
				require.True(t, ok, "want a ClusterPluginMissingError, got %T: %v", err, err)
				assert.Equal(t, tt.wantMissing, missing.Capability)
				_, isInput := errors.AsType[*engine.ClusterInputError](err)
				assert.False(t, isInput, "a missing plugin is not the caller's input")
				return
			}
			input, ok := errors.AsType[*engine.ClusterInputError](err)
			require.True(t, ok, "want a ClusterInputError, got %T: %v", err, err)
			assert.Equal(t, tt.wantInput, input.Error())
			stats := plugin.statsCallCount()
			assert.Equal(t, tt.wantStats, stats)
		})
	}
}

type failingClusterPricer struct{ clusterPricer }

func (failingClusterPricer) GetProjectedCostWithErrors(
	context.Context, []engine.ResourceDescriptor,
) (*engine.CostResultWithErrors, error) {
	return nil, errors.New("pricing plugin crashed")
}

func TestBuildClusterReport_PluginFailureIsNotInput(t *testing.T) {
	t.Parallel()

	client := startClusterPlugin(t, "kubernetes", &fakeClusterPlugin{},
		pluginhost.CapabilityUsageStats, pluginhost.CapabilityAllocation)
	_, err := buildClusterReport(context.Background(), []*pluginhost.Client{client}, failingClusterPricer{},
		clusterInputFor(t, nil))
	require.Error(t, err)
	_, isInput := errors.AsType[*engine.ClusterInputError](err)
	assert.False(t, isInput)
	_, isMissing := errors.AsType[*engine.ClusterPluginMissingError](err)
	assert.False(t, isMissing)
}

func TestBuildClusterPolicy(t *testing.T) {
	t.Parallel()

	allocOnly := []string{pluginhost.CapabilityAllocation}
	tests := []struct {
		name          string
		policy        config.AllocationPolicy
		caps          []string
		wantSource    string
		wantEffective string
		wantMissing   bool
		wantInput     string
	}{
		{name: "built-in defaults", caps: allocOnly, wantSource: "built-in defaults",
			wantEffective: `{"idle":"share"}`},
		{name: "request text", caps: allocOnly,
			policy:     config.AllocationPolicy{JSON: []byte(`{"idle":"separate"}`), Source: requestPolicySource},
			wantSource: requestPolicySource, wantEffective: `{"idle":"separate"}`},
		{
			name:      "rejected text",
			caps:      allocOnly,
			policy:    config.AllocationPolicy{JSON: []byte(`{"idle":"never"}`), Source: requestPolicySource},
			wantInput: `allocator kubernetes rejected the allocation policy request body: idle: unsupported value "never"`,
		},
		{name: "no allocator", caps: []string{pluginhost.CapabilityUsageStats}, wantMissing: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plugin := &fakeClusterPlugin{}
			client := startClusterPlugin(t, "kubernetes", plugin, tt.caps...)
			got, err := buildClusterPolicy(context.Background(), []*pluginhost.Client{client},
				clusterInputFor(t, func(in *clusterInput) { in.policy = tt.policy }))
			switch {
			case tt.wantMissing:
				missing, ok := errors.AsType[*engine.ClusterPluginMissingError](err)
				require.True(t, ok, "got %T: %v", err, err)
				assert.Equal(t, pluginhost.CapabilityAllocation, missing.Capability)
			case tt.wantInput != "":
				input, ok := errors.AsType[*engine.ClusterInputError](err)
				require.True(t, ok, "got %T: %v", err, err)
				assert.Equal(t, tt.wantInput, input.Error())
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.wantSource, got.Source)
				assert.JSONEq(t, tt.wantEffective, string(got.Effective))
				assert.Equal(t, "0123456789abcdef0123", got.Digest)
			}
			stats := plugin.statsCallCount()
			assert.Zero(t, stats, "showing the policy gathers no usage")
		})
	}
}

// TestClusterCommandError pins how the CLI maps builder errors: input errors
// exit 2 with their own message (an ax error keeps its code), everything else
// passes through.
func TestClusterCommandError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plain := errors.New("boom")
	assert.Same(t, plain, clusterCommandError(ctx, plain))

	missing := &engine.ClusterPluginMissingError{Capability: pluginhost.CapabilityAllocation}
	assert.Equal(t, error(missing), clusterCommandError(ctx, missing))

	mapped := clusterCommandError(ctx, &engine.ClusterInputError{Err: errors.New("invalid --group-by")})
	axErr, ok := errors.AsType[*ax.Error](mapped)
	require.True(t, ok)
	assert.Equal(t, ax.ExitValidation, axErr.ExitCode())
	assert.Equal(t, "invalid --group-by", axErr.Message)

	inner := ax.NewError(ctx, "config_invalid", "allocation policy x is not valid HuJSON",
		ax.WithErrorExitCode(ax.ExitValidation))
	mapped = clusterCommandError(ctx, &engine.ClusterInputError{Err: inner})
	axErr, ok = errors.AsType[*ax.Error](mapped)
	require.True(t, ok)
	assert.Equal(t, "config_invalid", axErr.ErrorCode, "an ax error keeps its own code")
}
