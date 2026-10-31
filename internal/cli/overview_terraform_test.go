package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	mockplugin "github.com/rshade/finfocus/test/mocks/plugin"
)

//nolint:paralleltest,nolintlint // Writes and executes a fake Terraform binary and changes the working directory.
func TestOverviewTerraformDirectoryAlwaysPlans(t *testing.T) {
	requirePOSIXShell(t)
	fixture, err := filepath.Abs("../ingest/testdata/terraform/plan/show.json")
	require.NoError(t, err)
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), []byte("# Terraform root\n"), 0o600))
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "plans")
	t.Setenv("OVERVIEW_TF_FIXTURE", fixture)
	t.Setenv("OVERVIEW_TF_PLANS", marker)
	script := `#!/bin/sh
case "$1" in
version) echo '{"terraform_version":"1.6.0"}';;
plan)
 echo plan >> "$OVERVIEW_TF_PLANS"
 for arg in "$@"; do case "$arg" in -out=*) : > "${arg#-out=}";; esac; done
 ;;
show) /bin/cat "$OVERVIEW_TF_FIXTURE";;
*) exit 1;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(script), 0o700))
	t.Setenv("PATH", bin)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Chdir(project)
	for range 2 {
		cmd := &cobra.Command{}
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetIn(strings.NewReader("n\n"))
		_, steps, _, stateOnly, loadErr := testLoadPlainOverviewData(context.Background(), cmd, overviewParams{})
		require.NoError(t, loadErr)
		assert.False(t, stateOnly)
		assert.NotEmpty(t, steps)
		assert.NotContains(t, output.String(), "preview")
	}
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	pipe := newOverviewPipeline(overviewPipelineConfig{cmd: cmd, params: overviewParams{cfg: config.New()}})
	data, err := tuiOverviewLoader(context.Background(), pipe)
	require.NoError(t, err)
	assert.False(t, data.IsStateOnly)
	assert.True(t, data.HasChanges)
	assert.NotEmpty(t, data.Rows)
	webData, webErr := autoDetectOverviewLoader(context.Background(), pipe)
	require.NoError(t, webErr)
	assert.NotEmpty(t, webData.Rows)
	plans, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, 4, strings.Count(string(plans), "plan\n"))
}

func TestOverviewTerraformSavedPlan(t *testing.T) {
	fixture, err := filepath.Abs("../ingest/testdata/terraform/plan/show.json")
	require.NoError(t, err)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // Any attempted infrastructure subprocess must fail.
	t.Chdir(t.TempDir())
	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"overview", "--terraform-plan", fixture, "--output", "json"},
	)
	assert.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "aws_s3_bucket")
	var output engine.OverviewJSONOutput
	require.NoError(t, json.Unmarshal(result.Stdout, &output))
	statuses := map[engine.ResourceStatus]int{
		engine.StatusCreating:  0,
		engine.StatusUpdating:  0,
		engine.StatusDeleting:  0,
		engine.StatusReplacing: 0,
		engine.StatusActive:    0,
	}
	for _, row := range output.Resources {
		statuses[row.Status]++
	}
	assert.Positive(t, statuses[engine.StatusCreating])
	assert.Positive(t, statuses[engine.StatusUpdating])
	assert.Positive(t, statuses[engine.StatusDeleting])
	assert.Positive(t, statuses[engine.StatusReplacing])
	assert.Positive(t, statuses[engine.StatusActive])
	for _, flag := range []string{"pulumi-json", "pulumi-state", "terraform-state"} {
		result = axtest.Run(
			context.Background(),
			t,
			NewRootCmd("test"),
			[]string{"overview", "--terraform-plan", fixture, "--" + flag, fixture},
		)
		assert.Equal(t, 2, result.ExitCode, string(result.Stderr))
		assert.Contains(t, string(result.Stderr), "mutually exclusive")
	}
	missing := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"overview", "--terraform-plan", "missing.json"},
	)
	assert.Equal(t, 2, missing.ExitCode)
	bad := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte(`{"format_version":"2.0"}`), 0o600))
	result = axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"overview", "--terraform-plan", bad})
	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "format_version")
}

func TestOverviewTerraformStateDoesNotPlan(t *testing.T) {
	fixture, err := filepath.Abs(realTerraformStatePath)
	require.NoError(t, err)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"overview", "--terraform-state", fixture, "--output", "json"},
	)
	assert.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "aws_instance")
}

func TestOverviewNoProjectIsValidationError(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	result := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"overview", "--plain"})
	assert.Equal(t, 2, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stderr), "Pulumi")
	assert.Contains(t, string(result.Stderr), "Terraform")
}

func TestOverviewTerraformNoOpPlanIsZero(t *testing.T) {
	raw, err := os.ReadFile("../ingest/testdata/terraform/plan/show.json")
	require.NoError(t, err)
	var document map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &document))
	var changes []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(document["resource_changes"], &changes))
	var unchanged []map[string]json.RawMessage
	for _, change := range changes {
		var action struct {
			Actions []string `json:"actions"`
		}
		require.NoError(t, json.Unmarshal(change["change"], &action))
		if len(action.Actions) == 1 && action.Actions[0] == "no-op" {
			unchanged = append(unchanged, change)
		}
	}
	require.NotEmpty(t, unchanged)
	document["resource_changes"], err = json.Marshal(unchanged)
	require.NoError(t, err)
	raw, err = json.Marshal(document)
	require.NoError(t, err)
	plan := filepath.Join(t.TempDir(), "noop.json")
	require.NoError(t, os.WriteFile(plan, raw, 0o600))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"overview", "--terraform-plan", plan, "--plain"},
	)
	assert.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "No changes. Projected monthly cost is zero.")
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	pipe := newOverviewPipeline(
		overviewPipelineConfig{cmd: cmd, params: overviewParams{terraformPlan: plan, cfg: config.New()}},
	)
	data, err := tuiOverviewLoader(context.Background(), pipe)
	require.NoError(t, err)
	assert.Empty(t, data.Rows, "no-op Terraform plans must not price existing resources")
}

//nolint:paralleltest // Uses a local gRPC mock plugin and pipeline configuration.
func TestOverviewTerraformResolvesTypesBeforeEnrichment(t *testing.T) {
	client, _ := newMockResolverClient(t, func(mock *mockplugin.MockPlugin) {
		mock.ConfigureTerraformResolver(mockplugin.AWSTerraformTypeMappings())
	})
	rows := []engine.OverviewRow{{URN: "aws_ebs_volume.disk", Type: "aws_ebs_volume", Status: engine.StatusUpdating,
		Properties: map[string]any{"type": "gp2"}, ProjectedProperties: map[string]any{"type": "gp3"}}}
	cmd := NewOverviewCmd()
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: cmd, params: overviewParams{terraformPlan: "loaded.json", cfg: config.New()},
		loader: fakePipelineLoader(rows), newEngine: fakeNewEngine, enrichRows: passThroughEnrichment,
		openPlugins: func(context.Context, string, *auditContext) ([]*pluginhost.Client, func(), error) {
			return []*pluginhost.Client{client}, func() {}, nil
		},
	})
	require.NoError(t, runOverviewPipeline(context.Background(), pipe))
	got := pipe.Result().Rows
	require.Len(t, got, 1)
	assert.Equal(t, "aws:ebs/volume:Volume", got[0].Type)
	assert.Equal(t, "gp2", got[0].Properties["volumeType"])
	assert.Equal(t, "gp3", got[0].ProjectedProperties["volumeType"])
	assert.Equal(t, engine.StatusUpdating, got[0].Status)
}

type overviewProviderRecorder struct {
	proto.CostSourceClient

	providers chan string
}

func (recorder *overviewProviderRecorder) GetProjectedCost(
	_ context.Context, request *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	response := &proto.GetProjectedCostResponse{}
	for _, resource := range request.Resources {
		recorder.providers <- resource.Provider
		response.Results = append(
			response.Results,
			&proto.CostResult{MonthlyCost: 1, Currency: "USD"},
		)
	}
	return response, nil
}

//nolint:paralleltest // Uses a local gRPC mock plugin.
func TestOverviewTerraformPreservesCustomProvider(t *testing.T) {
	client, _ := newMockResolverClient(t, func(mock *mockplugin.MockPlugin) {
		mock.Configure(
			mockplugin.MockConfig{
				SupportsAll:       true,
				Providers:         []string{"custom"},
				PluginSpecVersion: pluginsdk.SpecVersion,
			},
		)
	})
	recorder := &overviewProviderRecorder{CostSourceClient: client.API, providers: make(chan string, 4)}
	client.API = recorder
	cmd := NewOverviewCmd()
	state, steps, _, _, err := testLoadPlainOverviewData(
		context.Background(),
		cmd,
		overviewParams{terraformPlan: "../ingest/testdata/terraform/plan/show.json"},
	)
	require.NoError(t, err)
	rows, err := engine.MergeResourcesForOverview(context.Background(), state, steps)
	require.NoError(t, err)
	var row *engine.OverviewRow
	for i := range rows {
		if rows[i].URN == "cloudflare_zone.site" {
			row = &rows[i]
			break
		}
	}
	require.NotNil(t, row)
	eng := engine.New([]*pluginhost.Client{client}, nil)
	engine.EnrichOverviewRow(
		context.Background(),
		row,
		eng,
		engine.DateRange{Start: time.Now().Add(-time.Hour), End: time.Now()},
	)
	select {
	case provider := <-recorder.providers:
		assert.Equal(t, "custom", provider)
	default:
		require.Fail(t, "custom provider resource never reached the pricing plugin")
	}
}

func TestOverviewTerraformStateKeepsCloudIDAndAddress(t *testing.T) {
	t.Parallel()
	cmd := NewOverviewCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	plan := "../ingest/testdata/terraform/plan/show.json"
	resources, _, err := loadTerraformPlanFile(context.Background(), cmd, plan)
	require.NoError(t, err)
	var expectedID string
	for _, resource := range resources {
		if resource.ID == "terraform_data.holder" {
			expectedID, _ = resource.OldProperties[ingest.PropertyTerraformCloudID].(string)
		}
	}
	require.NotEmpty(t, expectedID)
	state, steps, _, _, err := testLoadPlainOverviewData(context.Background(), cmd, overviewParams{terraformPlan: plan})
	require.NoError(t, err)
	rows, err := engine.MergeResourcesForOverview(context.Background(), state, steps)
	require.NoError(t, err)
	var matched bool
	for _, row := range rows {
		if row.URN != "terraform_data.holder" {
			continue
		}
		matched = true
		assert.Equal(t, expectedID, row.ResourceID, "actual cost lookups use the cloud ID")
		assert.Equal(t, "terraform_data.holder", row.URN, "selectors retain the Terraform address")
	}
	assert.True(t, matched)
}
