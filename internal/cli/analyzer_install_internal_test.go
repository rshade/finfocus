package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/analyzer"
)

func TestReportAnalyzerInstallPrintsOneVPrefix(t *testing.T) {
	t.Setenv(agentModeEnv, "") // human output, whatever the caller's environment

	tests := []struct {
		name   string
		result analyzer.InstallResult
		want   []string
	}{
		{
			name:   "installed from a make build",
			result: analyzer.InstallResult{Action: analyzer.ActionInstalled, Version: "v0.5.4"},
			want:   []string{"  Version: v0.5.4\n"},
		},
		{
			name:   "installed from a release build",
			result: analyzer.InstallResult{Action: analyzer.ActionInstalled, Version: "0.5.4"},
			want:   []string{"  Version: v0.5.4\n"},
		},
		{
			name: "update available",
			result: analyzer.InstallResult{
				Action: analyzer.ActionUpdateAvailable, Version: "0.5.3", CurrentVersion: "v0.5.4",
			},
			want: []string{"already installed at v0.5.3\n", "  Current finfocus version: v0.5.4\n"},
		},
		{
			name:   "already current",
			result: analyzer.InstallResult{Action: analyzer.ActionAlreadyCurrent, Version: "0.5.4"},
			want:   []string{"already installed at v0.5.4\n"},
		},
	}
	for _, tc := range tests { //nolint:paralleltest // the parent sets AGENT_MODE with t.Setenv
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)

			require.NoError(t, reportAnalyzerInstall(cmd, &tc.result))

			for _, want := range tc.want {
				assert.Contains(t, out.String(), want)
			}
			assert.NotContains(t, out.String(), "vv")
		})
	}
}
