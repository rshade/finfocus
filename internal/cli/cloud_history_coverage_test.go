package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cloudhistory"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

type cloudHistoryStoreWithoutCache struct{ history.Store }

// Changes global config, environment, and working directory.
func TestCloudHistoryRequestUnavailableContext(t *testing.T) {
	setupCloudModeProject(t, "auto")
	cfg := config.New()
	cfg.Cost.History.PulumiCloud.Enabled = config.Auto
	config.SetGlobalConfig(cfg)
	t.Cleanup(config.ResetGlobalConfigForTest)
	store, err := history.NewBoltStore(context.Background(), t.TempDir(), true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	cmd := membershipCommand()
	cmd.SetContext(context.Background())
	now := time.Now()
	_, _, err = cloudHistoryRequest(cmd, cfg, cloudHistoryStoreWithoutCache{Store: store}, now.Add(-time.Hour), now)
	require.ErrorContains(t, err, "does not support Cloud cache")
	_, err = importActualCloudHistory(cmd, cloudHistoryStoreWithoutCache{Store: store}, now.Add(-time.Hour), now)
	require.ErrorContains(t, err, "does not support Cloud cache")
	t.Chdir(t.TempDir())
	_, req, err := cloudHistoryRequest(cmd, cfg, store, now.Add(-time.Hour), now)
	require.NoError(t, err)
	assert.Empty(t, req.Stack.Project)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: test\nruntime: go\n"), 0600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: ["), 0600))
	_, req, err = cloudHistoryRequest(cmd, cfg, store, now.Add(-time.Hour), now)
	require.NoError(t, err)
	assert.Empty(t, req.Stack.Stack, "malformed settings must leave detection unavailable")
}

// Changes global configuration, process environment, and the working directory.
func TestCloudHistoryContextMissingOrInvalidProject(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Chdir(t.TempDir())
	cmd := membershipCommand()
	original := context.Background()
	cmd.SetContext(original)
	assert.Equal(t, original, actualHistoryContext(cmd))
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: ["), 0600))
	assert.Equal(t, original, actualHistoryContext(cmd))
}

// Uses the process-wide retention environment override.
func TestCloudHistoryRetentionInvalidOverride(t *testing.T) {
	cfg := config.New()
	retentionDays := 17
	cfg.Cost.History.RetentionDays = &retentionDays
	for _, value := range []string{"invalid", "0", "-1"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(config.HistoryEnvRetentionDays, value)
			assert.Equal(t, 17, resolvedHistoryRetention(context.Background(), cfg))
		})
	}
}

func TestCloudHistoryReportLoggingAndDisabledOutput(t *testing.T) {
	t.Parallel()
	var logs, stdout, stderr bytes.Buffer
	logger := zerolog.New(&logs).Level(zerolog.DebugLevel)
	cmd := &cobra.Command{}
	cmd.SetContext(logger.WithContext(context.Background()))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	result := cloudhistory.Result{Listed: 6, Skipped: 1, FromCache: 2, Fetched: 3, Recorded: 4, Unreadable: 5}
	require.NoError(t, reportActualCloudHistory(cmd, config.Off, result, true))
	assert.Empty(t, logs.String())
	assert.Empty(t, stderr.String())
	require.NoError(t, reportActualCloudHistory(cmd, config.Auto, result, false))
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
	for _, field := range []string{`"listed":6`, `"skipped":1`, `"from_cache":2`, `"fetched":3`, `"recorded":4`, `"unreadable":5`} {
		assert.Contains(t, logs.String(), field)
	}
	result.Reason = "history_locked"
	require.NoError(t, reportActualCloudHistory(cmd, config.Auto, result, false))
	assert.Contains(t, stderr.String(), "history.db is locked")
}

// Uses process-wide project configuration and Pulumi login environment.
func TestCloudHistoryReportOutputFailure(t *testing.T) {
	setupCloudModeProject(t, "auto")
	cfg := config.New()
	cfg.Cost.History.PulumiCloud.Enabled = config.Auto
	config.SetGlobalConfig(cfg)
	t.Cleanup(config.ResetGlobalConfigForTest)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"updates":[]}`)) }),
	)
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)
	store, err := history.NewBoltStore(context.Background(), t.TempDir(), true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	cmd := membershipCommand()
	cmd.SetContext(context.Background())
	writeErr := errors.New("summary output unavailable")
	cmd.SetErr(errWriter{err: writeErr})
	now := time.Now()
	_, err = runActualCloudHistory(cmd, store, now.Add(-time.Hour), now, nil, true)
	require.ErrorIs(t, err, writeErr)
}
