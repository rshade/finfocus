package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/constants"
	"github.com/rshade/finfocus/internal/weblog"
)

const (
	tierAbsent = "absent"
	tierTrue   = "true"
	tierFalse  = "false"
)

func writeWebLogsTier(t *testing.T, dir, setting string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o700))
	var body string
	switch setting {
	case tierTrue, tierFalse:
		body = `{"web":{"logs":{"enabled":` + setting + `}}}`
	default:
		body = `{"output":{"precision":2}}`
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.hujson"), []byte(body), 0o600))
}

// Not parallel: t.Setenv and the process-wide config.
func TestWebLogsEnableMatrix(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		global  string
		project string
		want    bool
		invalid bool
	}{
		{name: "env 1 beats config false", env: "1", global: tierFalse, want: true},
		{name: "env 1 with config absent", env: "1", global: tierAbsent, want: true},
		{name: "env 0 beats config true", env: "0", global: tierTrue, want: false},
		{name: "env 0 with config absent", env: "0", global: tierAbsent, want: false},
		{name: "env maybe is a validation error", env: "maybe", global: tierTrue, invalid: true},
		{name: "unset env, config true", env: "", global: tierTrue, want: true},
		{name: "unset env, config false", env: "", global: tierFalse, want: false},
		{name: "unset env, config absent", env: "", global: tierAbsent, want: false},
		{name: "project true over absent global", env: "", global: tierAbsent, project: tierTrue, want: true},
		{name: "project true over global false", env: "", global: tierFalse, project: tierTrue, want: true},
		{name: "project false over global true", env: "", global: tierTrue, project: tierFalse, want: false},
		{name: "env 0 beats project true", env: "0", global: tierAbsent, project: tierTrue, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			project := filepath.Join(t.TempDir(), ".finfocus")
			t.Setenv("FINFOCUS_HOME", home)
			t.Setenv("FINFOCUS_PROJECT_DIR", project)
			t.Setenv(constants.EnvAnalyzerMode, "")
			t.Setenv(config.EnvWebLogs, tc.env)

			writeWebLogsTier(t, home, tc.global)
			projectDir := ""
			if tc.project != "" {
				writeWebLogsTier(t, project, tc.project)
				projectDir = project
			}
			config.ResetGlobalConfigForTest()
			t.Cleanup(config.ResetGlobalConfigForTest)
			cmd := newTestLoggingCmd()
			cmd.Use = "finfocus"
			cmd.Flags().Bool(webFlag, true, "serve the web UI")
			config.InitGlobalConfigWithProject(cmd.Context(), projectDir)

			var lifecycle commandLifecycle
			err := lifecycle.startLogging(cmd)
			if tc.invalid {
				require.Error(t, err)
				assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
				return
			}
			require.NoError(t, err)
			t.Cleanup(func() { _ = lifecycle.stopLogging() })
			assert.Equal(t, tc.want, weblog.HubFromContext(cmd.Context()) != nil)
		})
	}
}
