package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
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

// Configures the process-wide project, config, and Pulumi runner.
func TestActualCloudHistoryRootImport(t *testing.T) {
	setupMembershipProject(t)
	t.Setenv("FINFOCUS_HISTORY_ENABLED", "true")
	t.Setenv("FINFOCUS_HISTORY_RETENTION_DAYS", "10")
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "org")
	t.Setenv("PULUMI_ACCESS_TOKEN", "test-token")
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	require.NoError(t, os.MkdirAll(".finfocus", 0750))
	require.NoError(
		t,
		os.WriteFile(
			".finfocus/config.hujson",
			[]byte(`{"cost":{"history":{"pulumi_cloud":{"enabled":"auto"}}}}`),
			0600,
		),
	)
	now := time.Now().Truncate(time.Second)
	at := now.Add(-time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user":
			_, _ = w.Write([]byte(`{}`))
		case "/api/stacks/org/test/dev/updates":
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprintf(
					w,
					`{"updates":[{"version":1,"kind":"update","endTime":%d},{"version":2,"kind":"update","endTime":%d}]}`,
					at,
					now.Add(-20*24*time.Hour).Unix(),
				)
				return
			}
			_, _ = w.Write([]byte(`{"updates":[]}`))
		case "/api/stacks/org/test/dev/export/1":
			_, _ = w.Write(
				[]byte(
					`{"deployment":{"resources":[{"urn":"urn:pulumi:dev::test::aws:ec2/instance:Instance::destroyed","id":"i-destroyed","type":"aws:ec2/instance:Instance","custom":true}]}}`,
				),
			)
		default:
			assert.Fail(t, "unexpected Cloud request", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)
	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{
			"cost",
			"actual",
			"--from",
			now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			"--to",
			now.Format(time.RFC3339),
			"--output",
			"json",
			"--fallback-estimate",
		},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	store, err := history.NewBoltStore(
		context.Background(),
		filepath.Join(config.ResolveConfigDir(), "history"),
		true,
		10,
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	stack := history.StackContext{Project: "test", Stack: "dev"}
	entries, err := store.GetAllForStack(stack.Hash(), now.Add(-10*24*time.Hour).Unix(), now.Unix())
	require.NoError(t, err)
	var imported *history.ResourceHistoryEntry
	for i := range entries {
		if entries[i].CloudID == "i-destroyed" {
			imported = &entries[i]
		}
	}
	require.NotNil(t, imported, "destroyed identity must be imported before enrichment")
	assert.Equal(t, at, imported.FirstSeen)
	assert.Equal(t, at, imported.LastSeen)
	assert.Equal(t, history.SourcePulumiCloud, imported.Source)
	assert.Contains(t, string(result.Stdout), "destroyed")
	from := now.Add(-30 * 24 * time.Hour)
	enriched := enrichWithHistoricalResources(context.Background(), store, nil, from, now)
	var resources []engine.ResourceDescriptor
	for _, resource := range enriched {
		if resource.Properties["pulumi:cloudId"] == "i-destroyed" {
			resources = append(resources, resource)
		}
	}
	require.Len(t, resources, 1)
	billing := &cloudBillingClient{}
	eng := engine.New([]*pluginhost.Client{{Name: "aws", API: billing}}, nil).WithJobs(1)
	priced, err := eng.GetActualCostWithOptionsAndErrors(
		context.Background(),
		engine.ActualCostRequest{Resources: resources, From: from, To: now},
	)
	require.NoError(t, err)
	require.Len(t, priced.Results, 1)
	assert.InDelta(t, 42.0, priced.Results[0].TotalCost, 0.00001)
	require.Len(t, billing.requests, 1)
	assert.Equal(t, from.Unix(), billing.requests[0].StartTime)
	assert.Equal(t, now.Unix(), billing.requests[0].EndTime)
}

type cloudBillingClient struct {
	proto.CostSourceClient

	requests []*proto.GetActualCostRequest
}

func (c *cloudBillingClient) GetActualCost(
	_ context.Context,
	in *proto.GetActualCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	c.requests = append(c.requests, in)
	return &proto.GetActualCostResponse{Results: []*proto.ActualCostResult{{Currency: "USD", TotalCost: 42}}}, nil
}

func (c *cloudBillingClient) Supports(
	context.Context,
	*pbc.SupportsRequest,
	...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func setupCloudModeProject(t *testing.T, mode string) {
	t.Helper()
	setupMembershipProject(t)
	t.Setenv("FINFOCUS_HISTORY_ENABLED", "true")
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "org")
	t.Setenv("PULUMI_ACCESS_TOKEN", "test-token")
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	require.NoError(t, os.MkdirAll(".finfocus", 0750))
	require.NoError(
		t,
		os.WriteFile(
			".finfocus/config.hujson",
			[]byte(`{"cost":{"history":{"pulumi_cloud":{"enabled":"`+mode+`"}}}}`),
			0600,
		),
	)
}

// Configures process-wide project and config.
func TestActualCloudHistoryIncompleteEmptyOutput(t *testing.T) {
	for _, format := range []string{"json", "ndjson", "table"} {
		t.Run(format, func(t *testing.T) {
			setupCloudModeProject(t, "auto")
			now := time.Now().Truncate(time.Second)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/user" {
					_, _ = w.Write([]byte(`{}`))
					return
				}
				if strings.HasSuffix(r.URL.Path, "/updates") {
					if r.URL.Query().Get("page") == "1" {
						_, _ = fmt.Fprintf(
							w,
							`{"updates":[{"version":1,"kind":"update","endTime":%d}]}`,
							now.Add(-time.Hour).Unix(),
						)
						return
					}
					_, _ = w.Write([]byte(`{"updates":[]}`))
					return
				}
				http.NotFound(w, r)
			}))
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			result := axtest.Run(
				context.Background(),
				t,
				NewRootCmd("test"),
				[]string{
					"cost",
					"actual",
					"--from",
					now.Add(-24 * time.Hour).Format(time.RFC3339),
					"--to",
					now.Format(time.RFC3339),
					"--output",
					format,
					"--filter",
					"type=unmatched",
				},
			)
			require.Equal(t, 0, result.ExitCode, string(result.Stderr))
			assert.Contains(t, string(result.Stderr), "1 unreadable")
			output := string(result.Stdout)
			if format == "table" {
				assert.Contains(t, output, "1 unreadable")
			} else {
				assert.Contains(t, output, `"historyImport"`)
				assert.Contains(t, output, `"unreadableUpdates"`)
			}
		})
	}
}

// Configures process-wide config and environment.
func TestActualCloudVerboseCounters(t *testing.T) {
	setupCloudModeProject(t, "auto")
	now := time.Now().Truncate(time.Second)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"updates":[]}`)) }),
	)
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)
	args := []string{
		"cost",
		"actual",
		"--from",
		now.Add(-time.Hour).Format(time.RFC3339),
		"--to",
		now.Format(time.RFC3339),
		"--output",
		"json",
	}
	first := axtest.Run(context.Background(), t, NewRootCmd("test"), append(args, "--verbose"))
	require.Equal(t, 0, first.ExitCode, string(first.Stderr))
	second := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
	require.Equal(t, 0, second.ExitCode, string(second.Stderr))
	assert.Equal(t, string(first.Stdout), string(second.Stdout))
	prior := -1
	for _, label := range []string{"Updates listed", "Skipped (not state changing)", "Served from cache", "Fetched", "Identities recorded", "Unreadable"} {
		pos := strings.Index(string(first.Stderr), label)
		assert.Greater(t, pos, prior, label)
		prior = pos
		assert.NotContains(t, string(second.Stderr), label)
	}
}

// Configures process-wide config and environment.
func TestActualCloudHistorySkipsMismatchedInputStack(t *testing.T) {
	state, err := os.ReadFile("testdata/pulumi/stack-export-dev.json")
	require.NoError(t, err)
	setupCloudModeProject(t, "auto")
	statePath := filepath.Join(t.TempDir(), "production.json")
	require.NoError(
		t,
		os.WriteFile(
			statePath,
			[]byte(strings.ReplaceAll(string(state), "urn:pulumi:dev::", "urn:pulumi:production::")),
			0o600,
		),
	)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)
	now := time.Now().Truncate(time.Second)

	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{
			"cost", "actual",
			"--pulumi-state", statePath,
			"--stack", "dev",
			"--from", now.Add(-24 * time.Hour).Format(time.RFC3339),
			"--to", now.Format(time.RFC3339),
			"--output", "json",
		},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Zero(t, requests.Load(), "a state input for another stack must not import this stack's Cloud history")
	assert.Contains(t, string(result.Stderr), "history was not applied")

	store, err := history.NewBoltStore(
		context.Background(),
		filepath.Join(config.ResolveConfigDir(), "history"),
		true,
		90,
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	stack := history.StackContext{Project: "test", Stack: "dev"}
	entries, err := store.GetAllForStack(stack.Hash(), 0, now.Add(time.Hour).Unix())
	require.NoError(t, err)
	assert.Empty(t, entries, "another stack's resources must not be recorded under the selected stack")
}
