//go:build unix

package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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
	// launchLog lists every launch's plugin and child pid, one pair per line.
	launchLog string
	served    string
	env       []string
}

func newOrphanFixture(t *testing.T) orphanFixture {
	t.Helper()
	work := t.TempDir()
	home := filepath.Join(work, "home")
	project := filepath.Join(work, "project")
	pluginDir := filepath.Join(home, "plugins", "orphanfixture", "0.1.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o750))
	require.NoError(t, os.MkdirAll(project, 0o750))

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
	served := filepath.Join(work, "served")
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
		"ORPHAN_LAUNCH_LOG="+filepath.Join(work, "launches"),
		"ORPHAN_SERVED_FILE="+served,
	)
	return orphanFixture{
		binary: binary, project: project, pidFile: pidFile, launchLog: filepath.Join(work, "launches"),
		served: served, env: env,
	}
}

// recordedPIDs returns every plugin and child pid the fixture recorded: each
// launch in the launch log and the last pair in the pid file. Cleanup kills
// them all, so a failed assertion with several launches leaks no child.
func (f orphanFixture) recordedPIDs() []int {
	var pids []int
	for _, path := range []string{f.launchLog, f.pidFile} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(data)) {
			if pid, convErr := strconv.Atoi(field); convErr == nil && pid > 1 {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

type orphanWeb struct {
	cmd    *exec.Cmd
	done   chan error
	exited chan struct{}
	stderr func() string
}

func (f orphanFixture) startWeb(t *testing.T) *orphanWeb {
	t.Helper()
	plan, err := filepath.Abs("../../examples/plans/aws-simple-plan.json")
	require.NoError(t, err)
	web, _, _ := f.launch(t, "--web", "--no-browser", "--pulumi-json", plan)
	return web
}

// launch starts the binary with args, follows the printed bootstrap link, and
// returns the dashboard origin and a client holding the session cookie.
func (f orphanFixture) launch(t *testing.T, args ...string) (*orphanWeb, string, *http.Client) {
	t.Helper()
	cmd := exec.Command(f.binary, args...)
	cmd.Dir = f.project
	cmd.Env = f.env
	stderrFile, err := os.Create(filepath.Join(f.project, "stderr.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stderrFile.Close() })
	cmd.Stderr = stderrFile
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	web := &orphanWeb{cmd: cmd, done: make(chan error, 1), exited: make(chan struct{}), stderr: func() string {
		data, _ := os.ReadFile(stderrFile.Name())
		return string(data)
	}}
	go func() {
		web.done <- cmd.Wait()
		close(web.exited)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		select {
		case <-web.exited:
		case <-time.After(orphanWait):
		}
		for _, pid := range f.recordedPIDs() {
			if processAlive(t, pid) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
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
	var bootstrap string
	select {
	case bootstrap = <-urls:
	case waitErr := <-web.done:
		require.FailNow(t, "web launch failed", "%v\n%s", waitErr, web.stderr())
	case <-time.After(30 * time.Second):
		require.FailNow(t, "web launch timed out", web.stderr())
	}
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	resp, err := client.Get(bootstrap)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	parsed, err := url.Parse(bootstrap)
	require.NoError(t, err)
	return web, parsed.Scheme + "://" + parsed.Host, client
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
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(f.served)
		return statErr == nil
	}, orphanWait, 50*time.Millisecond, "plugin never served a pricing request; stderr:\n%s", web.stderr())
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

// startStacks runs `finfocus --web --stack dev,prod` in a Pulumi project whose
// pulumi CLI is the e2e fixture script, then activates both stacks.
func (f orphanFixture) startStacks(t *testing.T) (*orphanWeb, func() map[string]string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	script, err := os.ReadFile(filepath.Join(root, "test/e2e/fixtures/web/pulumi.sh"))
	require.NoError(t, err)
	bin := filepath.Join(f.project, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "pulumi"), script, 0o755))
	projectFile := filepath.Join(f.project, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(projectFile, []byte("name: orphan-fixture\nruntime: yaml\n"), 0o600))
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(projectFile, old, old))
	f.env = append(f.env,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WEB_STACKS=dev,prod",
		"WEB_STATE="+filepath.Join(root, "testdata/overview/state-no-changes.json"),
		"WEB_PLAN="+filepath.Join(root, "testdata/overview/plan-no-changes.json"),
	)
	web, base, client := f.launch(t, "--web", "--no-browser", "--stack", "dev,prod")
	for _, name := range []string{"dev", "prod"} {
		req, reqErr := http.NewRequest(http.MethodPost, base+"/api/stacks/"+name+"/activate", strings.NewReader("{}"))
		require.NoError(t, reqErr)
		req.Header.Set("Origin", base)
		req.Header.Set("Content-Type", "application/json")
		resp, postErr := client.Do(req)
		require.NoError(t, postErr)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusAccepted, resp.StatusCode, "activate %s", name)
	}
	states := func() map[string]string {
		resp, getErr := client.Get(base + "/api/stacks")
		if getErr != nil {
			return nil
		}
		defer resp.Body.Close()
		var body struct {
			Stacks []struct {
				Name  string `json:"name"`
				State string `json:"state"`
			} `json:"stacks"`
		}
		if json.NewDecoder(resp.Body).Decode(&body) != nil {
			return nil
		}
		out := map[string]string{}
		for _, stack := range body.Stacks {
			out[stack.Name] = stack.State
		}
		return out
	}
	return web, states
}

// TestWebSIGTERM_StacksSharePluginProcess activates two stacks whose plugin
// environment matches and asserts the plugin was launched once for both, then
// that SIGTERM leaves neither the shared plugin nor its child behind.
//
//nolint:paralleltest // Writes and executes a binary; a concurrent fork would hold the write fd (text file busy).
func TestWebSIGTERM_StacksSharePluginProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-building test in short mode")
	}

	f := newOrphanFixture(t)
	web, states := f.startStacks(t)

	require.Eventually(t, func() bool {
		got := states()
		return got["dev"] == "ready" && got["prod"] == "ready"
	}, orphanWait, 100*time.Millisecond, "both stacks must load; stderr:\n%s", web.stderr())
	data, err := os.ReadFile(f.launchLog)
	require.NoError(t, err)
	launches := strings.Fields(string(data))
	require.Len(t, launches, 2, "one launch (plugin and child pid) for both stacks; stderr:\n%s", web.stderr())
	pluginPID, err := strconv.Atoi(launches[0])
	require.NoError(t, err)
	childPID, err := strconv.Atoi(launches[1])
	require.NoError(t, err)
	require.True(t, processAlive(t, pluginPID))

	require.NoError(t, web.cmd.Process.Signal(syscall.SIGTERM))
	select {
	case waitErr := <-web.done:
		require.NoError(t, waitErr, "stderr:\n%s", web.stderr())
	case <-time.After(orphanWait):
		require.FailNow(t, "finfocus --web did not exit after SIGTERM", web.stderr())
	}

	assert.Eventually(t, func() bool { return !processAlive(t, pluginPID) }, orphanWait, 50*time.Millisecond,
		"shared plugin pid %d survived", pluginPID)
	assert.Eventually(t, func() bool { return !processAlive(t, childPID) }, orphanWait, 50*time.Millisecond,
		"shared plugin child pid %d survived", childPID)
}
