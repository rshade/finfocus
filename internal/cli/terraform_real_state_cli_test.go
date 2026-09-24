package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

const realTerraformStateGolden = "testdata/terraform/projected-no-plugins.golden.json"

// normalizeProjectedOutput strips time-dependent fields and returns
// summary.byProvider separately so its provider-prefix keying is asserted
// explicitly rather than only through the golden.
func normalizeProjectedOutput(t *testing.T, raw []byte) (map[string]interface{}, map[string]interface{}) {
	t.Helper()
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc), "stdout is not JSON: %s", raw)

	root, ok := doc["finfocus"].(map[string]interface{})
	require.True(t, ok, "missing finfocus envelope")
	summary, ok := root["summary"].(map[string]interface{})
	require.True(t, ok, "missing summary")
	byProvider, ok := summary["byProvider"].(map[string]interface{})
	require.True(t, ok, "missing summary.byProvider")
	delete(summary, "byProvider")

	for _, list := range []interface{}{root["resources"], summary["resources"]} {
		rows, isList := list.([]interface{})
		require.True(t, isList)
		for _, row := range rows {
			r, isMap := row.(map[string]interface{})
			require.True(t, isMap)
			for _, k := range []string{"startDate", "endDate"} {
				if _, exists := r[k]; exists {
					r[k] = "<normalized>"
				}
			}
		}
	}
	return doc, byProvider
}

func TestCostProjected_RealTerraformState_NoPluginsGolden(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)
	t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())

	statePath, err := filepath.Abs("../../test/fixtures/terraform/aws-realistic/terraform.tfstate")
	require.NoError(t, err)
	goldenPath, err := filepath.Abs(realTerraformStateGolden)
	require.NoError(t, err)
	isolateFromPulumiProject(t)

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{
		"cost", "projected", "--terraform-state", statePath, "--output", "json",
	})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)

	doc, byProvider := normalizeProjectedOutput(t, result.Stdout)

	root := doc["finfocus"].(map[string]interface{})
	summary := root["summary"].(map[string]interface{})
	rows := root["resources"].([]interface{})
	assert.Len(t, rows, 14, "14 managed instances; data sources excluded")
	assert.InDelta(t, 0, summary["totalMonthly"], 0)
	assert.Equal(t, map[string]interface{}{"aws": 0.0}, byProvider,
		"unresolved terraform types must group under their provider prefix")

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		r := row.(map[string]interface{})
		ids = append(ids, r["resourceId"].(string))
		assert.InDelta(t, 0, r["monthly"], 0, r["resourceId"])
		errObj, hasErr := r["error"].(map[string]interface{})
		require.True(t, hasErr, "fallback rows carry a NO_COST_DATA error: %v", r["resourceId"])
		assert.Equal(t, "NO_COST_DATA", errObj["code"])
	}
	assert.Contains(t, ids, "module.app.aws_instance.app[0]")
	assert.Contains(t, ids, "module.app.aws_instance.app[1]")
	assert.Contains(t, ids, `aws_instance.worker["small"]`)
	assert.Contains(t, ids, `aws_instance.worker["large"]`)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(doc))
	actual := buf.Bytes()
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, actual, 0o644))
		return
	}
	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "golden file %s missing; run UPDATE_GOLDEN=1", goldenPath)
	assert.JSONEq(t, string(expected), string(actual),
		"golden mismatch for %s; run UPDATE_GOLDEN=1 to update", goldenPath)
}
