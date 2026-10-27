package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/webui"
	mockplugin "github.com/rshade/finfocus/test/mocks/plugin"
)

//nolint:paralleltest,nolintlint // Changes cwd/environment and executes a fake tofu binary for a real web session.
func TestWebTerraformPlansAndServesPricedRows(t *testing.T) {
	unsetPassphraseEnv(t)
	fixture, err := filepath.Abs("../ingest/testdata/terraform/plan/show.json")
	require.NoError(t, err)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	parent := t.TempDir()
	writePulumiProject(t, parent, true)
	project := filepath.Join(parent, "terraform")
	require.NoError(t, os.Mkdir(project, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), []byte("# Terraform root\n"), 0o600))
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "commands")
	t.Setenv("WEB_TF_PLAN", fixture)
	t.Setenv("WEB_TF_MARKER", marker)
	script := `#!/bin/sh
case "$1" in
version) echo '{"terraform_version":"1.6.0"}';;
plan)
 echo plan >> "$WEB_TF_MARKER"
 for arg in "$@"; do case "$arg" in -out=*) : > "${arg#-out=}";; esac; done
 ;;
show) /bin/cat "$WEB_TF_PLAN";;
*) exit 1;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(script), 0o700))
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(bin, "pulumi"),
			[]byte("#!/bin/sh\necho pulumi >> \"$WEB_TF_MARKER\"\nexit 1\n"),
			0o700,
		),
	)
	t.Setenv("PATH", bin)
	t.Chdir(project)
	client, mock := newMockResolverClient(
		t,
		func(m *mockplugin.MockPlugin) { m.ConfigureTerraformResolver(mockplugin.AWSTerraformTypeMappings()) },
	)
	mock.SetProjectedCostResponse("aws:s3/bucket:Bucket", &proto.CostResult{MonthlyCost: 12, Currency: "USD"})
	ready := make(chan OverviewPipelineResult, 1)
	webPipelineHook = func(pipe *OverviewPipeline) {
		pipe.cfg.openPlugins = func(context.Context, string, *auditContext) ([]*pluginhost.Client, func(), error) {
			return []*pluginhost.Client{client}, func() {}, nil
		}
		onReady := pipe.OnReady
		pipe.OnReady = func(result OverviewPipelineResult) { onReady(result); ready <- result }
	}
	t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })
	out, cancel, wait := runWebCommand(t, []string{"--web", "--no-browser"})
	defer func() { cancel(); require.NoError(t, wait()) }()
	origin, cookie := webHTTP(t, awaitURL(t, out))
	select {
	case result := <-ready:
		require.NotEmpty(t, result.Rows)
		var found bool
		for _, row := range result.Rows {
			if row.URN == "aws_s3_bucket.site" {
				found = true
				assert.Equal(t, "aws:s3/bucket:Bucket", row.Type)
				require.NotNil(t, row.ProjectedCost)
				assert.InDelta(t, 12.0, row.ProjectedCost.MonthlyCost, 0.001)
			}
		}
		assert.True(t, found)
	case <-time.After(15 * time.Second):
		require.FailNow(t, "Terraform web pipeline did not reach ready")
	}
	response := webAPI(t, origin, cookie, "/api/overview/query", `{}`)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var page webui.OverviewPage
	require.NoError(t, json.NewDecoder(response.Body).Decode(&page))
	assert.NotEmpty(t, page.Rows)
	assertTerraformWebQueries(t, origin, cookie)
	commands, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(commands), "plan\n"))
	assert.NotContains(t, string(commands), "pulumi")
}

func assertTerraformWebQueries(t *testing.T, origin string, cookie *http.Cookie) {
	t.Helper()
	for _, request := range []struct{ path, body, contains string }{
		{"/api/estimate/resources", "", "aws:s3/bucket:Bucket"},
		{"/api/estimate/baseline?urn=aws_s3_bucket.site", "", "aws_s3_bucket.site"},
		{"/api/cost/actual/query", `{}`, ""},
		{"/api/recommendations/query", `{}`, ""},
	} {
		response := webAPI(t, origin, cookie, request.path, request.body)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode, "%s: %s", request.path, body)
		if request.contains != "" {
			assert.Contains(t, string(body), request.contains)
		}
		if strings.HasPrefix(request.path, "/api/estimate/baseline") {
			var baseline struct {
				Result *engine.EstimateResult `json:"result"`
			}
			require.NoError(t, json.Unmarshal(body, &baseline))
			require.NotNil(t, baseline.Result, "%s", body)
			require.NotNil(t, baseline.Result.Baseline)
			assert.InDelta(t, 12, baseline.Result.Baseline.Monthly, 0.001)
		}
	}
}

func TestWebTerraformCapturesSourceMetadata(t *testing.T) {
	t.Parallel()
	cmd := NewOverviewCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	data := newWebData(cmd, overviewParams{cfg: &config.Config{}}, engine.DateRange{}, nil)
	params := data.params
	params.terraformPlan = "../ingest/testdata/terraform/plan/show.json"
	snapshot, resources, _, err := loadPlainOverviewSnapshot(context.Background(), cmd, params, nil)
	require.NoError(t, err)
	require.NoError(
		t,
		data.setPipelineSource(
			context.Background(),
			OverviewPipelineResult{Snapshot: snapshot, TerraformResources: resources},
		),
	)
	resources, err = data.estimateResources(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, resources)
	byID := resourcesByID(resources)
	holder := byID["terraform_data.holder"]
	assert.Equal(t, "update", holder.Operation)
	assert.NotEmpty(t, holder.OldProperties)
	assert.Contains(t, holder.Properties, ingest.PropertyTerraformUnknown)
	assert.Equal(t, "custom", byID["cloudflare_zone.site"].Provider)
	deployed, err := data.loadResources(context.Background())
	require.NoError(t, err)
	deployedByID := resourcesByID(deployed)
	assert.NotContains(t, deployedByID, "aws_s3_bucket.site", "planned creates are not actual-cost resources")
	assert.Equal(t, holder.OldProperties, deployedByID[holder.ID].Properties)
}
