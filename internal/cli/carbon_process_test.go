package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/registry"
)

// TestCarbonPluginProcess builds the real companion plugin when explicitly
// supplied, using only loopback AWS fixtures and isolated configuration.
//
//nolint:gocognit,paralleltest // Sequential modes exercise one external process fixture.
func TestCarbonPluginProcess(t *testing.T) {
	source := os.Getenv("FINFOCUS_CARBON_PLUGIN_SOURCE")
	if source == "" {
		t.Skip("set FINFOCUS_CARBON_PLUGIN_SOURCE to the historical-carbon aws-ce source checkout")
	}
	if runtime.GOOS == "windows" {
		t.Skip("fixture launcher uses a POSIX shell")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	plugin := filepath.Join(dir, "aws-ce-real")
	core := filepath.Join(dir, "finfocus")
	for _, build := range []struct{ dir, target, out string }{{source, "./cmd/plugin", plugin}, {"../..", "./cmd/finfocus", core}} {
		cmd := exec.CommandContext(ctx, "go", "build", "-o", build.out, build.target)
		cmd.Dir = build.dir
		data, err := cmd.CombinedOutput()
		require.NoError(t, err, "build: %s", data)
	}
	var calls atomic.Int32
	var mode atomic.Value
	mode.Store("pages")
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Contains(t, r.Header.Get("Authorization"), "fixture-key/")
		var query struct {
			NextToken string `json:"NextToken"`
		}
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&query)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch mode.Load().(string) {
		case "empty":
			_, _ = fmt.Fprint(w, `{"Results":[]}`)
			return
		case "error":
			if query.NextToken != "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprint(w, `{"__type":"ValidationException","message":"later fixture failure"}`)
				return
			}
		}
		if mode.Load() == "zero" {
			_, _ = fmt.Fprintf(
				w,
				`{"Results":[%s]}`,
				carbonAWSRow("111", "AmazonEC2", "2026-01-01", "2026-02-01", "v1", 0),
			)
			return
		}
		if query.NextToken == "" {
			_, _ = fmt.Fprintf(
				w,
				`{"Results":[%s,%s],"NextToken":"aws-next"}`,
				carbonAWSRow("111", "AmazonEC2", "2026-01-01", "2026-02-01", "v1", 1.25),
				carbonAWSRow("222", "AmazonS3", "2026-01-01", "2026-02-01", "v1", 2.5),
			)
			return
		}
		assert.Equal(t, "aws-next", query.NextToken)
		_, _ = fmt.Fprintf(
			w,
			`{"Results":[%s,%s]}`,
			carbonAWSRow("111", "AmazonS3", "2026-02-01", "2026-03-01", "v2", 0),
			carbonAWSRow("222", "AmazonEC2", "2026-02-01", "2026-03-01", "v2", 0.75),
		)
	}))
	defer fixture.Close()
	costFixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_, _ = fmt.Fprint(
			w,
			`{"ResultsByTime":[{"TimePeriod":{"Start":"2026-01-01","End":"2026-02-01"},"Total":{"UnblendedCost":{"Amount":"1","Unit":"USD"},"AmortizedCost":{"Amount":"1","Unit":"USD"}}}],"ForecastResultsByTime":[{"TimePeriod":{"Start":"2026-01-01","End":"2026-02-01"},"MeanValue":"2"}],"Total":{"Amount":"2","Unit":"USD"}}`,
		)
	}))
	defer costFixture.Close()
	for _, configured := range []bool{false, true} {
		t.Run(fmt.Sprintf("config=%t", configured), func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			binDir := filepath.Join(home, "plugins", "aws-ce", "1.0.0")
			require.NoError(t, os.MkdirAll(binDir, 0755))
			starts := filepath.Join(home, "starts")
			wrapper := "#!/bin/sh\nprintf '%s\\n' \"${FINFOCUS_AWS_CARBON_ENABLED-unset}\" >> '" + starts + "'\nexec '" + plugin + "' \"$@\"\n"
			require.NoError(t, os.WriteFile(filepath.Join(binDir, "finfocus-plugin-aws-ce"), []byte(wrapper), 0755))
			env := carbonProcessEnv(home, project, fixture.URL, costFixture.URL)
			if configured {
				require.NoError(
					t,
					os.WriteFile(
						filepath.Join(home, "config.hujson"),
						[]byte(`{"plugins":{"aws-ce":{"carbon_enabled":true}}}`),
						0600,
					),
				)
			} else {
				env = append(env, "FINFOCUS_AWS_CARBON_ENABLED=true")
			}
			run := func(args ...string) (string, string, error) {
				cmd := exec.CommandContext(ctx, core, args...)
				cmd.Dir = project
				cmd.Env = env
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err := cmd.Run()
				return stdout.String(), stderr.String(), err
			}

			beforeSchema := calls.Load()
			schemaOut, schemaErr, err := run("__schema", "--as=mcp")
			require.NoError(t, err, schemaErr)
			assert.Contains(t, schemaOut, "finfocus-carbon-actual")
			assert.Equal(t, beforeSchema, calls.Load(), "schema discovery must not request AWS data")
			_, statErr := os.Stat(starts)
			assert.True(t, os.IsNotExist(statErr), "schema discovery must not launch the installed plugin")
			args := []string{"carbon", "actual", "--start", "2026-01-01", "--end", "2026-03-01", "--output", "json"}
			mode.Store("pages")
			before := calls.Load()
			stdout, stderr, err := run(args...)
			require.NoError(t, err, "%s", stderr)
			assert.Equal(t, int32(2), calls.Load()-before)
			var report struct {
				TotalGrams *float64         `json:"totalGrams"`
				Records    []map[string]any `json:"records"`
				Request    map[string]any   `json:"request"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &report))
			require.NotNil(t, report.TotalGrams)
			assert.InDelta(t, 4500000.0, *report.TotalGrams, 0)
			require.Len(t, report.Records, 4)
			assert.Equal(t, "v2", report.Records[3]["modelVersion"])
			assert.Equal(t, "aws-sustainability", report.Records[0]["source"])
			assert.Equal(t, "2026-01-01T00:00:00Z", report.Records[0]["start"])
			assert.Zero(t, report.Records[2]["value"])
			assert.Contains(t, stdout, "CARBON_ACCOUNTING_METHOD_MARKET_BASED")
			launches, readErr := os.ReadFile(starts)
			require.NoError(t, readErr)
			if configured {
				assert.Equal(
					t,
					"unset\ntrue\n",
					string(launches),
					"config-only enable must restart after descriptor discovery",
				)
			} else {
				assert.True(t, strings.HasPrefix(string(launches), "true\n"))
				assert.NotContains(t, string(launches), "unset")
			}
			runCarbonMCPReport(ctx, t, core, project, env)
			mode.Store("error")
			stdout, stderr, err = run(args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			assert.Contains(t, stderr, "historical carbon page 2")
			mode.Store("empty")
			stdout, stderr, err = run(args...)
			require.NoError(t, err, "%s", stderr)
			assert.Contains(t, stdout, `"totalGrams":null`)
			assert.Contains(t, stdout, `"records":[]`)
			mode.Store("zero")
			stdout, stderr, err = run(args...)
			require.NoError(t, err, "%s", stderr)
			assert.Contains(t, stdout, `"totalGrams":0`)
			assert.Contains(t, stdout, `"value":0`)
			// Ordinary monetary RPCs must never consult Sustainability with opt-in on.
			before = calls.Load()
			runCarbonMonetaryProbe(ctx, t, env, project)
			assert.Equal(t, before, calls.Load())
			// Disable at both sources and require actionable guidance, without AWS reads.
			env = carbonProcessEnv(home, project, fixture.URL, costFixture.URL)
			require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), []byte(`{}`), 0600))
			before = calls.Load()
			stdout, stderr, err = run(args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			assert.Contains(t, stderr, "FINFOCUS_AWS_CARBON_ENABLED=true")
			assert.Equal(t, before, calls.Load())
			before = calls.Load()
			runCarbonMonetaryProbe(ctx, t, env, project)
			assert.Equal(t, before, calls.Load())
		})
	}
}
func carbonAWSRow(account, service, start, end, model string, value float64) string {
	return fmt.Sprintf(
		`{"DimensionsValues":{"USAGE_ACCOUNT_ID":%q,"REGION":"us-east-1","SERVICE":%q},"EmissionsValues":{"TOTAL_MBM_CARBON_EMISSIONS":{"Unit":"MTCO2e","Value":%g}},"ModelVersion":%q,"TimePeriod":{"Start":%q,"End":%q}}`,
		account,
		service,
		value,
		model,
		start+"T00:00:00Z",
		end+"T00:00:00Z",
	)
}
func carbonProcessEnv(home, project, carbonURL, costURL string) []string {
	env := []string{}
	for _, pair := range os.Environ() {
		key, _, _ := strings.Cut(pair, "=")
		if strings.HasPrefix(key, "AWS_") || strings.HasPrefix(key, "FINFOCUS_") {
			continue
		}
		env = append(env, pair)
	}
	return append(
		env,
		"FINFOCUS_HOME="+home,
		"FINFOCUS_PROJECT_DIR="+project,
		"AWS_REGION=us-east-1",
		"AWS_ACCESS_KEY_ID=fixture-key",
		"AWS_SECRET_ACCESS_KEY=fixture-secret",
		"AWS_EC2_METADATA_DISABLED=true",
		"AWS_ENDPOINT_URL_SUSTAINABILITY="+carbonURL,
		"AWS_ENDPOINT_URL_COST_EXPLORER="+costURL,
	)
}

func runCarbonMonetaryProbe(ctx context.Context, t *testing.T, env []string, project string) {
	t.Helper()
	binary, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestCarbonMonetaryHelper$")
	cmd.Env = append(append([]string(nil), env...), "FINFOCUS_CARBON_MONETARY_HELPER=true")
	cmd.Dir = project
	data, err := cmd.CombinedOutput()
	require.NoError(t, err, "monetary RPCs: %s", data)
}
func TestCarbonMonetaryHelper(t *testing.T) {
	t.Parallel()
	if os.Getenv("FINFOCUS_CARBON_MONETARY_HELPER") != "true" {
		t.Skip("subprocess helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	clients, cleanup, err := registry.NewDefault().Open(ctx, "aws-ce")
	require.NoError(t, err)
	defer cleanup()
	require.Len(t, clients, 1)
	client := pbc.NewCostSourceServiceClient(clients[0].Conn)
	_, err = client.GetActualCost(
		ctx,
		&pbc.GetActualCostRequest{
			ResourceId: "aws-account-total",
			Start:      timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			End:        timestamppb.New(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)),
		},
	)
	require.NoError(t, err)
	_, err = client.GetProjectedCost(
		ctx,
		&pbc.GetProjectedCostRequest{
			Resource: &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:account",
				Sku:    "account",
				Region: "us-east-1", Id: "aws-account-total"},
		},
	)
	require.NoError(t, err)
}

func runCarbonMCPReport(ctx context.Context, t *testing.T, core, project string, env []string) {
	t.Helper()
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, core, "mcp-server")
	cmd.Dir = project
	cmd.Env = env
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, stdin.Close())
		waitErr := cmd.Wait()
		require.NoError(t, waitErr, stderr.String())
	}()
	decoder := json.NewDecoder(stdout)
	_, err = fmt.Fprintln(
		stdin,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"carbon-fixture","version":"1.0"}}}`,
	)
	require.NoError(t, err)
	var initialized json.RawMessage
	require.NoError(t, decoder.Decode(&initialized))
	_, err = fmt.Fprintln(stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	require.NoError(t, err)
	_, err = fmt.Fprintln(
		stdin,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"finfocus-carbon-actual","arguments":{"start":"2026-01-01","end":"2026-03-01"}}}`,
	)
	require.NoError(t, err)
	for {
		var response struct {
			ID     int `json:"id"`
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		require.NoError(t, decoder.Decode(&response))
		if response.ID != 2 {
			continue
		}
		require.False(t, response.Result.IsError)
		require.NotEmpty(t, response.Result.Content)
		text := response.Result.Content[0].Text
		require.True(t, json.Valid([]byte(text)), "MCP report must be JSON without --output: %s", text)
		var report struct {
			TotalGrams float64           `json:"totalGrams"`
			Records    []json.RawMessage `json:"records"`
		}
		require.NoError(t, json.Unmarshal([]byte(text), &report))
		assert.InDelta(t, 4500000.0, report.TotalGrams, 0)
		assert.Len(t, report.Records, 4)
		assert.Contains(t, text, "modelVersion")
		return
	}
}
