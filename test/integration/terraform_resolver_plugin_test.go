package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/test/integration/helpers"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// These tests launch a real plugin process through the registry: the plugin is
// this test binary re-executed into TestTerraformResolverPluginProcess, which
// serves the mock plugin on the --port the plugin host assigns. They pin the
// behavior `cost projected --terraform-state` must have once a published
// plugin implements ResolveResourceTypes (finfocus-spec >= v0.6.1, #1506).
const (
	envResolverMode   = "FINFOCUS_TEST_TFRESOLVER_MODE"
	envResolverEvents = "FINFOCUS_TEST_TFRESOLVER_EVENTS"

	resolverModeFull    = "full"
	resolverModePartial = "partial"
	resolverModeError   = "error"

	realTerraformState = "../fixtures/terraform/aws-realistic/terraform.tfstate"
)

const (
	instanceMonthly = 75.92
	volumeMonthly   = 8.00
	databaseMonthly = 12.41
)

func resolverPrices() map[string]float64 {
	return map[string]float64{
		"aws:ec2/instance:Instance": instanceMonthly,
		"aws:ebs/volume:Volume":     volumeMonthly,
		"aws:rds/instance:Instance": databaseMonthly,
	}
}

// TestTerraformResolverPluginProcess is the plugin entry point, not a test. It
// only runs when re-executed by a plugin launched from installResolverPlugin.
func TestTerraformResolverPluginProcess(t *testing.T) {
	mode := os.Getenv(envResolverMode)
	if mode == "" {
		t.Skip("helper process for the terraform resolver plugin tests")
	}

	mock := plugin.NewMockPlugin()
	mock.Configure(plugin.MockConfig{PluginSpecVersion: pluginsdk.SpecVersion, SupportsAll: true})
	for resourceType, monthly := range resolverPrices() {
		mock.SetProjectedCostResponse(resourceType, plugin.QuickResponse("USD", monthly, monthly/730))
	}

	mappings := plugin.AWSTerraformTypeMappings()
	switch mode {
	case resolverModePartial:
		mappings = map[string]*pbc.ResourceTypeMapping{"aws_instance": mappings["aws_instance"]}
	case resolverModeError:
		mock.SetError("ResolveResourceTypes", plugin.ErrorUnavailable)
	}
	mock.ConfigureTerraformResolver(mappings)

	eventsPath := os.Getenv(envResolverEvents)
	mock.SetResolveResourceTypesHook(func(req *pbc.ResolveResourceTypesRequest) {
		f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = fmt.Fprintln(f, strings.Join(req.GetSourceTypes(), ","))
	})

	server, err := plugin.StartMockServerTCPAt(mock, "127.0.0.1:"+os.Getenv(pluginsdk.EnvPort))
	require.NoError(t, err)
	defer server.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

// installResolverPlugin isolates FINFOCUS_HOME and installs a plugin wrapper
// that re-executes this test binary as the mock plugin in the given mode. It
// returns the path of the file recording each ResolveResourceTypes call.
func installResolverPlugin(t *testing.T, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("plugin wrapper is a POSIX shell script")
	}

	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	eventsPath := filepath.Join(t.TempDir(), "resolve-calls.log")
	t.Setenv(envResolverMode, mode)
	t.Setenv(envResolverEvents, eventsPath)

	exe, err := os.Executable()
	require.NoError(t, err)
	require.NotContains(t, exe, "'")

	pluginDir := filepath.Join(home, "plugins", "tfresolver", "0.1.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	script := fmt.Sprintf("#!/bin/sh\nexec '%s' -test.run='^TestTerraformResolverPluginProcess$' -- \"$@\"\n", exe)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "finfocus-plugin-tfresolver"), []byte(script), 0o755))

	return eventsPath
}

func resolveCalls(t *testing.T, eventsPath string) []string {
	t.Helper()
	data, err := os.ReadFile(eventsPath)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

type projectedRow struct {
	ResourceType string  `json:"resourceType"`
	ResourceID   string  `json:"resourceId"`
	Adapter      string  `json:"adapter"`
	Monthly      float64 `json:"monthly"`
}

type projectedOutput struct {
	Finfocus struct {
		Summary struct {
			TotalMonthly float64            `json:"totalMonthly"`
			ByProvider   map[string]float64 `json:"byProvider"`
		} `json:"summary"`
		Resources []projectedRow `json:"resources"`
	} `json:"finfocus"`
}

func runProjectedOnRealState(t *testing.T) (projectedOutput, map[string]projectedRow) {
	t.Helper()
	statePath, err := filepath.Abs(realTerraformState)
	require.NoError(t, err)

	h := helpers.NewCLIHelper(t)
	stdout := h.ExecuteOrFail("cost", "projected", "--terraform-state", statePath, "--output", "json")

	var out projectedOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	require.Len(t, out.Finfocus.Resources, 14)

	byID := make(map[string]projectedRow, len(out.Finfocus.Resources))
	for _, r := range out.Finfocus.Resources {
		byID[r.ResourceID] = r
	}
	return out, byID
}

func realStateInstanceIDs() []string {
	return []string{
		"aws_instance.web",
		"module.app.aws_instance.app[0]",
		"module.app.aws_instance.app[1]",
		`aws_instance.worker["small"]`,
		`aws_instance.worker["large"]`,
	}
}

func TestTerraformState_ResolverPlugin_ResolvesAndPrices(t *testing.T) {
	eventsPath := installResolverPlugin(t, resolverModeFull)

	out, byID := runProjectedOnRealState(t)

	for _, r := range out.Finfocus.Resources {
		assert.Contains(t, r.ResourceType, ":", "%s left unresolved", r.ResourceID)
	}
	for _, id := range realStateInstanceIDs() {
		assert.Equal(t, "aws:ec2/instance:Instance", byID[id].ResourceType, id)
		assert.InDelta(t, instanceMonthly, byID[id].Monthly, 0.001, id)
	}
	assert.Equal(t, "aws:ebs/volume:Volume", byID["aws_ebs_volume.data"].ResourceType)
	assert.Positive(t, byID["aws_ebs_volume.data"].Monthly)
	assert.Equal(t, "aws:rds/instance:Instance", byID["aws_db_instance.main"].ResourceType)
	assert.Positive(t, byID["aws_db_instance.main"].Monthly)

	wantTotal := 5*instanceMonthly + volumeMonthly + databaseMonthly
	assert.InDelta(t, wantTotal, out.Finfocus.Summary.TotalMonthly, 0.01)
	assert.Equal(t, map[string]float64{"aws": out.Finfocus.Summary.TotalMonthly}, out.Finfocus.Summary.ByProvider)

	calls := resolveCalls(t, eventsPath)
	require.Len(t, calls, 1, "one ResolveResourceTypes call for the aws provider")
	assert.ElementsMatch(t, []string{
		"aws_db_instance", "aws_dynamodb_table", "aws_ebs_volume", "aws_iam_role", "aws_instance",
		"aws_s3_bucket", "aws_security_group", "aws_subnet", "aws_volume_attachment", "aws_vpc",
	}, strings.Split(calls[0], ","))

	t.Run("second run hits resolve_types cache", func(t *testing.T) {
		out2, byID2 := runProjectedOnRealState(t)
		assert.Len(t, resolveCalls(t, eventsPath), 1, "resolution must come from the cache")
		assert.Equal(t, "aws:ec2/instance:Instance", byID2["aws_instance.web"].ResourceType)
		assert.InDelta(t, out.Finfocus.Summary.TotalMonthly, out2.Finfocus.Summary.TotalMonthly, 0.001)
	})
}

func TestTerraformState_ResolverPlugin_PartialResolution(t *testing.T) {
	eventsPath := installResolverPlugin(t, resolverModePartial)

	out, byID := runProjectedOnRealState(t)

	for _, id := range realStateInstanceIDs() {
		assert.Equal(t, "aws:ec2/instance:Instance", byID[id].ResourceType, id)
		assert.Positive(t, byID[id].Monthly, id)
	}
	for _, id := range []string{"aws_ebs_volume.data", "aws_db_instance.main", "aws_s3_bucket.assets"} {
		assert.NotContains(t, byID[id].ResourceType, ":", "%s should keep its raw type", id)
		assert.Zero(t, byID[id].Monthly, id)
	}
	assert.InDelta(t, 5*instanceMonthly, out.Finfocus.Summary.TotalMonthly, 0.01)
	assert.Positive(t, out.Finfocus.Summary.ByProvider["aws"])
	assert.Len(t, resolveCalls(t, eventsPath), 1)
}

func TestTerraformState_ResolverPlugin_RPCErrorFallsBack(t *testing.T) {
	eventsPath := installResolverPlugin(t, resolverModeError)

	out, _ := runProjectedOnRealState(t)

	for _, r := range out.Finfocus.Resources {
		assert.NotContains(t, r.ResourceType, ":", "%s should keep its raw type", r.ResourceID)
		assert.Zero(t, r.Monthly, r.ResourceID)
	}
	assert.Zero(t, out.Finfocus.Summary.TotalMonthly)
	assert.Len(t, resolveCalls(t, eventsPath), 1, "the failing RPC is still attempted once")
}
