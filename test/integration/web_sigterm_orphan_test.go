//go:build unix

package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	orphanTestVersion = "v0.0.0-orphantest"
	orphanWait        = 20 * time.Second
)

func processAlive(t *testing.T, pid int) bool {
	t.Helper()
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err == nil && bytes.Contains(stat, []byte(") Z ")) {
		return false
	}
	return true
}

// TestWebSIGTERM_StopsPluginAndItsChild runs the real binary with a fixture
// plugin that spawns a same-group child, as the aws-public router does, and
// asserts SIGTERM to `finfocus --web` leaves neither process behind.
type orphanFixture struct {
	binary  string
	project string
	pidFile string
	env     []string
}

func newOrphanFixture(t *testing.T) orphanFixture {
	t.Helper()
	work := t.TempDir()
	home := filepath.Join(work, "home")
	project := filepath.Join(work, "project")
	pluginDir := filepath.Join(home, "plugins", "orphanfixture", "0.1.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o750))
	require.NoError(t, os.MkdirAll(project, 0o750))
	pulumiYAML := []byte("name: orphan\nruntime: yaml\n")
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.yaml"), pulumiYAML, 0o600))

	binary := filepath.Join(work, "finfocus")
	out, err := exec.Command("go", "build",
		"-ldflags", "-X github.com/rshade/finfocus/pkg/version.version="+orphanTestVersion,
		"-o", binary, "../../cmd/finfocus").CombinedOutput()
	require.NoError(t, err, "build finfocus: %s", out)

	pluginBinary := "finfocus-plugin-orphanfixture"
	pluginPath := filepath.Join(pluginDir, pluginBinary)
	out, err = exec.Command("go", "build", "-o", pluginPath, "./testdata/orphanplugin").CombinedOutput()
	require.NoError(t, err, "build fixture plugin: %s", out)
	manifest, err := json.Marshal(map[string]any{
		"name":                "orphanfixture",
		"version":             "0.1.0",
		"description":         "router-like fixture",
		"author":              "FinFocus tests",
		"supported_providers": []string{"aws"},
		"protocols":           []string{"grpc"},
		"binary":              pluginBinary,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.manifest.json"), manifest, 0o600))

	pidFile := filepath.Join(work, "pids")
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "FINFOCUS_") && !strings.HasPrefix(entry, "PULUMI_") {
			env = append(env, entry)
		}
	}
	env = append(env,
		"FINFOCUS_HOME="+home,
		"FINFOCUS_PROJECT_DIR="+project,
		"FINFOCUS_HISTORY_ENABLED=false",
		"FINFOCUS_CACHE_ENABLED=false",
		"FINFOCUS_SKIP_MIGRATION_CHECK=1",
		"ORPHAN_PID_FILE="+pidFile,
	)
	return orphanFixture{binary: binary, project: project, pidFile: pidFile, env: env}
}

type orphanWeb struct {
	cmd    *exec.Cmd
	done   chan error
	stderr func() string
}

func (f orphanFixture) startWeb(t *testing.T) *orphanWeb {
	t.Helper()
	plan, err := filepath.Abs("../../examples/plans/aws-simple-plan.json")
	require.NoError(t, err)
	cmd := exec.Command(f.binary, "--web", "--no-browser", "--pulumi-json", plan)
	cmd.Dir = f.project
	cmd.Env = f.env
	stderrFile, err := os.Create(filepath.Join(f.project, "stderr.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stderrFile.Close() })
	cmd.Stderr = stderrFile
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	web := &orphanWeb{cmd: cmd, done: make(chan error, 1), stderr: func() string {
		data, _ := os.ReadFile(stderrFile.Name())
		return string(data)
	}}
	go func() { web.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if data, readErr := os.ReadFile(f.pidFile); readErr == nil {
			var pluginPID, childPID int
			if _, scanErr := fmt.Sscan(string(data), &pluginPID, &childPID); scanErr == nil {
				_ = syscall.Kill(pluginPID, syscall.SIGKILL)
				_ = syscall.Kill(childPID, syscall.SIGKILL)
			}
		}
		_ = cmd.Process.Kill()
	})

	urls := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "http://127.0.0.1:") {
				select {
				case urls <- line:
				default:
				}
			}
		}
	}()
	select {
	case bootstrap := <-urls:
		client := &http.Client{Timeout: 10 * time.Second}
		resp, getErr := client.Get(bootstrap)
		require.NoError(t, getErr)
		require.NoError(t, resp.Body.Close())
	case waitErr := <-web.done:
		require.FailNow(t, "web launch failed", "%v\n%s", waitErr, web.stderr())
	case <-time.After(30 * time.Second):
		require.FailNow(t, "web launch timed out", web.stderr())
	}
	return web
}

// TestWebSIGTERM_StopsPluginAndItsChild runs the real binary with a fixture
// plugin that spawns a same-group child, as the aws-public router does, and
// asserts SIGTERM to `finfocus --web` leaves neither process behind. A child
// that leaves the plugin's process group is outside what core can reap.
//
//nolint:paralleltest // Writes and executes a binary; a concurrent fork would hold the write fd (text file busy).
func TestWebSIGTERM_StopsPluginAndItsChild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-building test in short mode")
	}

	f := newOrphanFixture(t)
	web := f.startWeb(t)
	var pluginPID, childPID int

	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(f.pidFile)
		if readErr != nil {
			return false
		}
		_, scanErr := fmt.Sscan(string(data), &pluginPID, &childPID)
		return scanErr == nil
	}, orphanWait, 50*time.Millisecond, "fixture plugin never started; stderr:\n%s", web.stderr())
	require.True(t, processAlive(t, pluginPID))
	require.True(t, processAlive(t, childPID))

	require.NoError(t, web.cmd.Process.Signal(syscall.SIGTERM))
	select {
	case waitErr := <-web.done:
		require.NoError(t, waitErr, "stderr:\n%s", web.stderr())
	case <-time.After(orphanWait):
		require.FailNow(t, "finfocus --web did not exit after SIGTERM", web.stderr())
	}

	assert.Eventually(t, func() bool { return !processAlive(t, pluginPID) }, orphanWait, 50*time.Millisecond,
		"plugin pid %d survived", pluginPID)
	assert.Eventually(t, func() bool { return !processAlive(t, childPID) }, orphanWait, 50*time.Millisecond,
		"plugin child pid %d survived", childPID)
}
