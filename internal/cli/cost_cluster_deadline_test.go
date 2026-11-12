package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// TestCostCluster_StalledPluginCallTimesOut installs the testdata/usagesource
// plugins and makes one call never answer. The run must end with exit 1 and
// an error that names the plugin and the call, and the stalled plugin process
// must be gone when the command returns. kubernetes is the allocator and
// serves run-rate only; prometheus serves historical only. It stays
// sequential: it shortens clusterCallTimeouts, sets FINFOCUS_HOME, and builds
// a root command.
func TestCostCluster_StalledPluginCallTimesOut(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a usage-source plugin binary")
	}
	installUsageSourcePlugins(t)
	t.Setenv("FINFOCUS_TEST_RUN_RATE_ONLY", "kubernetes")
	t.Setenv("FINFOCUS_TEST_HISTORICAL_ONLY", "prometheus")

	const deadline = time.Second
	prev := clusterCallTimeouts
	clusterCallTimeouts = engine.ClusterCallTimeouts{
		Stats: deadline, StatsPerDay: 0, StatsMax: deadline, Policy: deadline, Allocate: deadline,
	}
	t.Cleanup(func() { clusterCallTimeouts = prev })

	policy := filepath.Join(t.TempDir(), "good.hujson")
	require.NoError(t, os.WriteFile(policy, []byte(`{ "idle": "separate" }`), 0o600))
	today := time.Now().UTC().Truncate(24 * time.Hour)
	window := []string{
		"--from", today.AddDate(0, 0, -14).Format(time.DateOnly),
		"--to", today.AddDate(0, 0, -7).Format(time.DateOnly),
	}

	tests := []struct {
		name      string
		args      []string
		stall     string
		wantMsg   string
		wantAsked map[string]string
	}{
		{
			name:      "GetStats on the chosen usage source",
			args:      []string{"--usage-source", "kubernetes"},
			stall:     "kubernetes:GetStats",
			wantMsg:   "get usage stats: usage source kubernetes did not answer GetStats within 1s",
			wantAsked: map[string]string{"kubernetes": "run-rate\n"},
		},
		{
			name:    "GetStats probe while choosing a usage source",
			args:    window,
			stall:   "kubernetes:GetStats",
			wantMsg: "get usage stats: usage source kubernetes did not answer GetStats within 1s",
			// The stalled probe stops the selection: prometheus is never asked.
			wantAsked: map[string]string{"kubernetes": "historical\n"},
		},
		{
			name:      "policy check",
			args:      []string{"--usage-source", "kubernetes", "--policy", policy},
			stall:     "kubernetes:Allocate",
			wantMsg:   "allocate: allocator kubernetes did not answer Allocate within 1s",
			wantAsked: map[string]string{"kubernetes": "allocate\n"},
		},
		{
			name:      "show-policy",
			args:      []string{"--show-policy"},
			stall:     "kubernetes:Allocate",
			wantMsg:   "allocate: allocator kubernetes did not answer Allocate within 1s",
			wantAsked: map[string]string{"kubernetes": "allocate\n"},
		},
		{
			name:      "allocation",
			args:      []string{"--usage-source", "kubernetes"},
			stall:     "kubernetes:Allocate",
			wantMsg:   "allocate: allocator kubernetes did not answer Allocate within 1s",
			wantAsked: map[string]string{"kubernetes": "run-rate\nallocate\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers, pids := t.TempDir(), t.TempDir()
			t.Setenv("FINFOCUS_TEST_GETSTATS_MARKER_DIR", markers)
			t.Setenv("FINFOCUS_TEST_STALL", tt.stall)
			t.Setenv("FINFOCUS_TEST_STALL_PID_DIR", pids)

			args := append([]string{"cost", "cluster"}, tt.args...)
			start := time.Now()
			res := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
			elapsed := time.Since(start)

			require.Equal(t, 1, res.ExitCode, "stderr: %s", res.Stderr)
			code, msg := errorEnvelope(t, res.Stderr)
			assert.Equal(t, "internal_error", code)
			assert.Equal(t, tt.wantMsg, msg)
			assert.Less(t, elapsed, 30*time.Second, "the run must end at the deadline, not hang")
			assert.Equal(t, tt.wantAsked, readAskedMarkers(t, markers))

			stalled, _, _ := strings.Cut(tt.stall, ":")
			assertProcessGone(t, filepath.Join(pids, stalled))
		})
	}
}

// assertProcessGone reads the PID a stalled plugin wrote and checks that no
// such process is left. Signal 0 probes a PID only on Unix.
func assertProcessGone(t *testing.T, pidFile string) {
	t.Helper()
	data, err := os.ReadFile(pidFile)
	require.NoError(t, err, "the stalled plugin did not record its PID")
	pid, err := strconv.Atoi(string(data))
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		t.Logf("not probing plugin PID %d on Windows", pid)
		return
	}
	proc, err := os.FindProcess(pid)
	require.NoError(t, err)
	// Only "no such process" proves the PID is gone; EPERM means it still exists.
	sigErr := proc.Signal(syscall.Signal(0))
	assert.Truef(t, errors.Is(sigErr, syscall.ESRCH) || errors.Is(sigErr, os.ErrProcessDone),
		"stalled plugin PID %d is still running (signal 0: %v)", pid, sigErr)
}
