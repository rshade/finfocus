package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/test/integration/helpers"
)

// recorderAltInstall is the install directory the recorder is copied into, so
// the name it reports ("recorder") differs from its install name.
const recorderAltInstall = "recorder-alt"

const testDeploymentPlan = `{
  "steps": [
    {
      "op": "create",
      "urn": "urn:pulumi:dev::demo::test:apps/v1:Deployment::web",
      "newState": {
        "type": "test:apps/v1:Deployment",
        "inputs": {"replicas": 1}
      }
    }
  ]
}`

// recorderAsked reports whether the recorder received any request: through
// the engine it is only ever asked Supports for a test: resource.
func recorderAsked(t *testing.T, dir string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, readErr)
		var file struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.Unmarshal(raw, &file))
		if file.Method == "Supports" {
			return true
		}
	}
	return false
}

// TestRouting_InstallNameAndReportedNameAgree runs the real CLI against the
// real recorder installed under a name it does not report (#2278, #2206).
// Feature and pattern rules for either spelling are covered by the router
// tests; this checks the wiring a unit test cannot: the install name reaching
// the client from the registry, config validate, the run-time warning for an
// entry that matches nothing, and --adapter.
//
//nolint:paralleltest // sets FINFOCUS_HOME and executes a copied binary
func TestRouting_InstallNameAndReportedNameAgree(t *testing.T) {
	binPath := recorderBinary(t)
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skipf("Plugin binary not found at %s. Run 'make build-recorder' first.", binPath)
	}

	home := t.TempDir()
	installRecorderAs(t, home, binPath, recorderAltInstall)
	project := t.TempDir()
	plan := filepath.Join(project, "plan.json")
	require.NoError(t, os.WriteFile(plan, []byte(testDeploymentPlan), 0o600))

	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_PLUGIN_DIR", "")
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_LOG_LEVEL", "warn")
	t.Chdir(project)

	writeRouting := func(t *testing.T, name string) {
		t.Helper()
		body := `{}`
		if name != "" {
			body = `{"routing":{"plugins":[{"name":"` + name + `","features":["ProjectedCosts"]}]}}`
		}
		require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), []byte(body), 0o600))
		config.ResetGlobalConfigForTest()
		t.Cleanup(config.ResetGlobalConfigForTest)
	}
	logFile := filepath.Join(home, "logs", "finfocus.log")
	// runProjected reports whether the recorder was asked, and the log lines
	// this run wrote (the CLI logs to a file under FINFOCUS_HOME).
	runProjected := func(t *testing.T, args ...string) (bool, string) {
		t.Helper()
		require.NoError(t, os.RemoveAll(logFile))
		recordDir := t.TempDir()
		t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", recordDir)
		helper := helpers.NewCLIHelper(t)
		cmd := append([]string{"cost", "projected", "--pulumi-json", plan, "--output", "json"}, args...)
		_, err := helper.Execute(cmd...)
		require.NoError(t, err, "stderr: %s", helper.Stderr())
		logs, readErr := os.ReadFile(logFile)
		if !os.IsNotExist(readErr) {
			require.NoError(t, readErr)
		}
		return recorderAsked(t, recordDir), string(logs) + helper.Stderr()
	}

	for _, spelling := range []string{recorderAltInstall, "recorder"} {
		t.Run("routing by "+spelling, func(t *testing.T) {
			writeRouting(t, spelling)
			helper := helpers.NewCLIHelper(t)
			out, err := helper.Execute("config", "validate")
			require.NoError(t, err, "config validate rejected %q: %s %s", spelling, out, helper.Stderr())

			asked, stderr := runProjected(t)
			assert.True(t, asked)
			assert.NotContains(t, stderr, "routing entry matches no loaded plugin")
		})
	}
	t.Run("routing by a name no plugin has", func(t *testing.T) {
		writeRouting(t, "recorder-typo")
		helper := helpers.NewCLIHelper(t)
		_, err := helper.Execute("config", "validate")
		require.Error(t, err)

		_, stderr := runProjected(t)
		assert.Contains(t, stderr, "routing entry matches no loaded plugin")
		assert.Contains(t, stderr, "recorder-typo")
	})

	for _, spelling := range []string{recorderAltInstall, "recorder"} {
		t.Run("--adapter "+spelling, func(t *testing.T) {
			writeRouting(t, "")
			asked, stderr := runProjected(t, "--adapter", spelling)
			assert.True(t, asked)
			assert.NotContains(t, stderr, "no loaded plugin answers")
		})
	}
	t.Run("--adapter naming no plugin", func(t *testing.T) {
		writeRouting(t, "")
		asked, stderr := runProjected(t, "--adapter", "recorder-typo")
		assert.False(t, asked, "no plugin answers to the name, so none is asked")
		assert.Contains(t, stderr, "no loaded plugin answers to the --adapter value")
	})
}
