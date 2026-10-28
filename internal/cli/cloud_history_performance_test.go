package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cloudhistory"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

func cloudHistoryMedian(samples []time.Duration) time.Duration {
	slices.Sort(samples)
	return samples[len(samples)/2]
}

func TestCloudHistorySkippedImportLatency(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		file       bool
	}{{"unset", "", false}, {"auto file", "auto", true}} {
		t.Run(tc.name, func(t *testing.T) {
			setupMembershipProject(t)
			t.Setenv("FINFOCUS_HISTORY_ENABLED", "true")
			t.Setenv("PULUMI_HOME", t.TempDir())
			t.Setenv("PULUMI_ACCESS_TOKEN", "test-token")
			config.ResetGlobalConfigForTest()
			t.Cleanup(config.ResetGlobalConfigForTest)
			cfg := config.GetGlobalConfig()
			if tc.mode == "auto" {
				cfg.Cost.History.PulumiCloud.Enabled = config.Auto
			}
			var requests, constructed atomic.Int64
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					requests.Add(1)
					w.WriteHeader(http.StatusInternalServerError)
				}),
			)
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			if tc.file {
				t.Setenv("PULUMI_BACKEND_URL", "file:///tmp/cloud-history-latency")
			}
			store, err := history.NewBoltStore(context.Background(), t.TempDir(), true, 90)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.Flags().String("stack", "dev", "")
			now := time.Now()
			samples := make([]time.Duration, 0, 5)
			for range 5 {
				start := time.Now()
				_, importErr := importActualCloudHistory(cmd, store, now.Add(-time.Hour), now)
				samples = append(samples, time.Since(start))
				require.NoError(t, importErr)
			}
			t.Logf("median skipped import=%v", cloudHistoryMedian(samples))
			assert.Less(t, cloudHistoryMedian(samples), 100*time.Millisecond)
			deps, req, err := cloudHistoryRequest(cmd, cfg, store, now.Add(-time.Hour), now)
			require.NoError(t, err)
			deps.NewClient = func(s esc.Session) pulumicloud.Client { constructed.Add(1); return pulumicloud.NewClient(s, nil) }
			_, err = cloudhistory.Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Zero(t, constructed.Load())
			assert.Zero(t, requests.Load())
		})
	}
}
