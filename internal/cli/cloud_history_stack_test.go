package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

// Uses the process-wide Pulumi runner, config, environment, and working directory.
func TestActualCloudHistoryExplicitStack(t *testing.T) {
	setupCloudModeProject(t, "true")
	now := time.Now().Truncate(time.Second)
	store, err := history.NewBoltStore(
		context.Background(),
		filepath.Join(config.ResolveConfigDir(), "history"),
		true,
		90,
	)
	require.NoError(t, err)
	require.NoError(
		t,
		store.Upsert(
			(history.StackContext{Project: "test", Stack: "dev"}).Hash(),
			history.ResourceHistoryEntry{
				URN:       "urn:pulumi:dev::test::aws:ec2/instance:Instance::wrong-dev",
				CloudID:   "i-wrong-dev",
				Type:      "aws:ec2/instance:Instance",
				Provider:  "aws",
				FirstSeen: now.Add(-time.Hour).Unix(),
				LastSeen:  now.Unix(),
				Source:    history.SourceStateSnapshot,
			},
		),
	)
	require.NoError(t, store.Close())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user":
			_, _ = w.Write([]byte(`{}`))
		case "/api/stacks/org/test/production/updates":
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprintf(
					w,
					`{"updates":[{"version":1,"kind":"update","endTime":%d}]}`,
					now.Add(-time.Hour).Unix(),
				)
				return
			}
			_, _ = w.Write([]byte(`{"updates":[]}`))
		case "/api/stacks/org/test/production/export/1":
			_, _ = w.Write(
				[]byte(
					`{"deployment":{"resources":[{"urn":"urn:pulumi:production::test::aws:ec2/instance:Instance::destroyed-prod","id":"i-production","type":"aws:ec2/instance:Instance","custom":true}]}}`,
				),
			)
		default:
			assert.Fail(t, "unexpected stack endpoint", r.URL.Path)
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
			"--stack",
			"production",
			"--from",
			now.Add(-24 * time.Hour).Format(time.RFC3339),
			"--to",
			now.Format(time.RFC3339),
			"--output",
			"json",
			"--fallback-estimate",
		},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "destroyed-prod")
	assert.NotContains(t, string(result.Stdout), "wrong-dev")
}
