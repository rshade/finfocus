package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mcpTestVersion   = "v0.0.0-mcptest"
	mcpSessionBudget = 3 * time.Minute
	mcpSimplePlan    = "../../examples/plans/aws-simple-plan.json"
	mcpToolsGolden   = "../../internal/cli/testdata/mcp/tools.golden"
)

// mcpTestEnv holds the binaries and isolated directories shared by the MCP
// server integration tests.
type mcpTestEnv struct {
	binary      string
	home        string
	recorder    string
	recordedDir string
	workDir     string
}

// newMCPTestEnv builds finfocus (with a real version, which the MCP handshake
// requires) and the recorder plugin, installing the recorder into an isolated
// FINFOCUS_HOME so tools/call exercises a real plugin subprocess.
func newMCPTestEnv(t *testing.T) *mcpTestEnv {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping binary-building MCP test in short mode")
	}

	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	binDir := t.TempDir()
	home := t.TempDir()
	env := &mcpTestEnv{
		binary:      filepath.Join(binDir, "finfocus"+exe),
		home:        home,
		recorder:    filepath.Join(home, "plugins", "recorder", "0.1.0", "finfocus-plugin-recorder"+exe),
		recordedDir: t.TempDir(),
		workDir:     t.TempDir(),
	}

	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/rshade/finfocus/pkg/version.version="+mcpTestVersion,
		"-o", env.binary, "../../cmd/finfocus")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build finfocus: %s", out)

	require.NoError(t, os.MkdirAll(filepath.Dir(env.recorder), 0o755))
	build = exec.Command("go", "build", "-o", env.recorder, "../../plugins/recorder/cmd")
	out, err = build.CombinedOutput()
	require.NoError(t, err, "build recorder: %s", out)
	manifest, err := os.ReadFile("../../plugins/recorder/plugin.manifest.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(env.recorder), "plugin.manifest.json"), manifest, 0o644))

	return env
}

func (e *mcpTestEnv) environ() []string {
	return append(os.Environ(),
		"FINFOCUS_HOME="+e.home,
		"HOME="+e.home,
		"PULUMI_HOME="+filepath.Join(e.home, "pulumi"),
		"FINFOCUS_SKIP_MIGRATION_CHECK=1",
		"FINFOCUS_RECORDER_OUTPUT_DIR="+e.recordedDir,
	)
}

// mcpSession is a live finfocus MCP server driven over newline-delimited
// JSON-RPC on stdio.
type mcpSession struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr *bytes.Buffer
	nextID int
}

func startMCPSession(t *testing.T, env *mcpTestEnv, entryArgs ...string) *mcpSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), mcpSessionBudget)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, env.binary, entryArgs...)
	cmd.Env = env.environ()
	cmd.Dir = env.workDir
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1<<20), 16<<20)
	s := &mcpSession{t: t, cmd: cmd, stdin: stdin, stdout: scanner, stderr: &stderr, nextID: 1}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	s.request("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "finfocus-integration", "version": "1"},
	})
	s.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	return s
}

func (s *mcpSession) send(msg map[string]any) {
	s.t.Helper()
	line, err := json.Marshal(msg)
	require.NoError(s.t, err)
	_, err = s.stdin.Write(append(line, '\n'))
	require.NoError(s.t, err)
}

// request sends a JSON-RPC request and returns the result of its response.
func (s *mcpSession) request(method string, params map[string]any) json.RawMessage {
	s.t.Helper()
	result, rpcErr := s.exchange(method, params)
	require.Empty(s.t, rpcErr, "JSON-RPC error for %s", method)
	return result
}

// exchange sends a JSON-RPC request and returns its response's result and error.
func (s *mcpSession) exchange(method string, params map[string]any) (json.RawMessage, json.RawMessage) {
	s.t.Helper()
	id := s.nextID
	s.nextID++
	s.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})

	for s.stdout.Scan() {
		var resp struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		require.NoError(s.t, json.Unmarshal(s.stdout.Bytes(), &resp),
			"protocol stdout must carry only JSON-RPC: %s", s.stdout.Text())
		if resp.ID == nil || *resp.ID != id {
			continue
		}
		return resp.Result, resp.Error
	}
	require.FailNow(s.t, "MCP server closed stdout", "method %s; err %v; stderr:\n%s",
		method, s.stdout.Err(), s.stderr.String())
	return nil, nil
}

func (s *mcpSession) toolNames() []string {
	s.t.Helper()
	var list struct {
		Tools []struct {
			Name        string `json:"name"`
			InputSchema struct {
				Properties map[string]any `json:"properties"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	require.NoError(s.t, json.Unmarshal(s.request("tools/list", map[string]any{}), &list))
	names := make([]string, 0, len(list.Tools))
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
		assert.NotContains(s.t, tool.InputSchema.Properties, "mcp",
			"--mcp is root-local and must not be an argument of %s", tool.Name)
	}
	sort.Strings(names)
	return names
}

type mcpToolResult struct {
	text    string
	isError bool
}

func (s *mcpSession) callTool(name string, args map[string]any) mcpToolResult {
	s.t.Helper()
	var result struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	raw := s.request("tools/call", map[string]any{"name": name, "arguments": args})
	require.NoError(s.t, json.Unmarshal(raw, &result))
	require.NotEmpty(s.t, result.Content, "tool %s returned no content", name)
	return mcpToolResult{text: result.Content[0].Text, isError: result.IsError}
}

// close ends the session by closing stdin and returns the server's stderr.
func (s *mcpSession) close() string {
	s.t.Helper()
	require.NoError(s.t, s.stdin.Close())
	require.NoError(s.t, s.cmd.Wait(), "MCP server exit; stderr:\n%s", s.stderr.String())
	return s.stderr.String()
}

func readToolsGolden(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(mcpToolsGolden)
	require.NoError(t, err)
	return strings.Fields(string(data))
}

// runningProcessesFor lists live processes whose command line references path.
// It is Linux-only (it reads /proc) and returns nil elsewhere.
func runningProcessesFor(t *testing.T, path string) []string {
	t.Helper()
	if runtime.GOOS != "linux" {
		return nil
	}
	entries, err := os.ReadDir("/proc")
	require.NoError(t, err)
	var matches []string
	for _, entry := range entries {
		cmdline, readErr := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if readErr != nil || !bytes.Contains(cmdline, []byte(path)) {
			continue
		}
		stat, statErr := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if statErr == nil && bytes.Contains(stat, []byte(") Z ")) {
			continue
		}
		matches = append(matches,
			fmt.Sprintf("pid %s: %s", entry.Name(), bytes.ReplaceAll(cmdline, []byte{0}, []byte{' '})))
	}
	return matches
}

// TestMCPServer_ToolListMatchesGoldenForBothEntryPoints verifies that
// `finfocus mcp-server` and `finfocus --mcp` serve the identical tool list,
// which is also the static `__schema --as=mcp` list pinned by the unit golden.
func TestMCPServer_ToolListMatchesGoldenForBothEntryPoints(t *testing.T) {
	env := newMCPTestEnv(t)
	golden := readToolsGolden(t)

	server := startMCPSession(t, env, "mcp-server")
	serverTools := server.toolNames()
	server.close()

	alias := startMCPSession(t, env, "--mcp")
	aliasTools := alias.toolNames()
	alias.close()

	assert.Equal(t, golden, serverTools)
	assert.Equal(t, serverTools, aliasTools)
	for _, excluded := range []string{"finfocus-analyzer-serve", "finfocus-setup", "finfocus-plugin-init"} {
		assert.NotContains(t, serverTools, excluded)
	}

	schema := exec.Command(env.binary, "__schema", "--as=mcp")
	schema.Env = env.environ()
	out, err := schema.Output()
	require.NoError(t, err)
	var static struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(out, &static))
	staticNames := make([]string, 0, len(static.Tools))
	for _, tool := range static.Tools {
		staticNames = append(staticNames, tool.Name)
	}
	sort.Strings(staticNames)
	assert.Equal(t, serverTools, staticNames)
}

// TestMCPServer_SequentialCallsInOneSession drives one long-lived server through
// a mixed sequence of tools/call requests. Every default payload must be JSON,
// an explicit output=table must still return a table (and must not stick to the
// next call), the logging lifecycle must stay clean, and no plugin subprocess
// may outlive the call that launched it.
func TestMCPServer_SequentialCallsInOneSession(t *testing.T) {
	env := newMCPTestEnv(t)
	plan, err := filepath.Abs(mcpSimplePlan)
	require.NoError(t, err)

	session := startMCPSession(t, env, "mcp-server")
	require.Contains(t, session.toolNames(), "finfocus-cost-projected")

	jsonCalls := []struct {
		name string
		args map[string]any
	}{
		{"finfocus-cost-projected", map[string]any{"pulumi-json": plan}},
		{"finfocus-plugin-list", map[string]any{}},
		{"finfocus-config-list", map[string]any{}},
		{"finfocus-config-validate", map[string]any{}},
		{"finfocus-config-routes-list", map[string]any{}},
		{"finfocus-cost-recommendations", map[string]any{"pulumi-json": plan}},
		{"finfocus-plugin-validate", map[string]any{}},
		{"finfocus-cost-actual", map[string]any{"pulumi-json": plan, "from": "2025-01-01", "to": "2025-01-31"}},
		{"finfocus-cost-projected", map[string]any{"pulumi-json": plan, "output": "json"}},
		{"finfocus-cost-projected", map[string]any{"pulumi-json": plan}},
	}
	payloads := map[string]string{}
	for i, call := range jsonCalls {
		result := session.callTool(call.name, call.args)
		require.False(t, result.isError, "call %d %s failed: %s", i, call.name, result.text)
		assert.True(t, json.Valid([]byte(result.text)), "call %d %s payload is not JSON: %s", i, call.name, result.text)
		payloads[call.name] = result.text
		assert.Empty(t, runningProcessesFor(t, env.recorder),
			"plugin subprocess outlived call %d (%s)", i, call.name)
	}

	table := session.callTool("finfocus-cost-projected", map[string]any{"pulumi-json": plan, "output": "table"})
	require.False(t, table.isError, table.text)
	assert.False(t, json.Valid([]byte(table.text)), "explicit output=table must return a table")
	assert.Contains(t, table.text, "COST SUMMARY")

	afterTable := session.callTool("finfocus-cost-projected", map[string]any{"pulumi-json": plan})
	require.False(t, afterTable.isError, afterTable.text)
	assert.True(t, json.Valid([]byte(afterTable.text)), "--output from the previous call must not stick")

	stderr := session.close()
	assert.NotContains(t, stderr, "internal_error")
	assert.NotContains(t, stderr, "already closed")
	assert.LessOrEqual(t, strings.Count(stderr, "Logging to:"), 1, "logging must be opened once per server")
	assert.Empty(t, runningProcessesFor(t, env.recorder), "no plugin subprocess may outlive the server")

	recorded, err := os.ReadDir(env.recordedDir)
	require.NoError(t, err)
	assert.NotEmpty(t, recorded, "cost calls must have reached the recorder plugin subprocess")

	var plugins []map[string]any
	require.NoError(t, json.Unmarshal([]byte(payloads["finfocus-plugin-list"]), &plugins))
	require.Len(t, plugins, 1)
	assert.Equal(t, "recorder", plugins[0]["name"])

	single := exec.Command(env.binary, "config", "list", "--format", "json")
	single.Env = env.environ()
	single.Dir = env.workDir
	out, err := single.Output()
	require.NoError(t, err)
	assert.JSONEq(t, string(out), payloads["finfocus-config-list"],
		"a tools/call must match the equivalent single-shot CLI invocation")
}

// TestMCPServer_MutatingToolHonorsDryRun verifies a mutating tool called with
// dry-run reports what it would do as JSON without touching the filesystem.
//
// No tool on the current surface is confirmation-gated: every command that uses
// ax.Confirm (recommendations dismiss/snooze/undismiss) takes a positional
// recommendation ID, which the ax-go live server cannot pass, so those
// commands are not tools. This test asserts they stay off the tool list.
func TestMCPServer_MutatingToolHonorsDryRun(t *testing.T) {
	env := newMCPTestEnv(t)
	session := startMCPSession(t, env, "mcp-server")

	tools := session.toolNames()
	for _, gated := range []string{
		"finfocus-cost-recommendations-dismiss",
		"finfocus-cost-recommendations-snooze",
		"finfocus-cost-recommendations-undismiss",
	} {
		assert.NotContains(t, tools, gated)
	}

	configPath := filepath.Join(env.home, "config.hujson")
	require.NoFileExists(t, configPath)

	result := session.callTool("finfocus-config-init", map[string]any{"global": true, "dry-run": true})
	require.False(t, result.isError, result.text)
	var action struct {
		Action string `json:"action"`
		DryRun bool   `json:"dry_run"`
		Path   string `json:"path"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.text), &action))
	assert.Equal(t, "would_create", action.Action)
	assert.True(t, action.DryRun)
	assert.Equal(t, configPath, action.Path)
	assert.NoFileExists(t, configPath, "dry-run must not write the config file")

	_, rootErr := session.exchange("tools/call",
		map[string]any{"name": "finfocus", "arguments": map[string]any{"mcp": true}})
	require.NotEmpty(t, rootErr, "the excluded root must not be callable, so --mcp cannot recurse")
	assert.Contains(t, string(rootErr), `unknown tool \"finfocus\"`)

	session.close()
}
