package cli

import (
	"context"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectOutputFormatFromArgs(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "long flag with equals",
			argv: []string{"finfocus", "overview", "--output=json"},
			want: "json",
		},
		{
			name: "long flag with separate value",
			argv: []string{"finfocus", "cost", "projected", "--output", "ndjson"},
			want: "ndjson",
		},
		{
			name: "short flag with separate value",
			argv: []string{"finfocus", "plugin", "certify", "-o", "json"},
			want: "json",
		},
		{
			name: "short flag with equals",
			argv: []string{"finfocus", "plugin", "certify", "-o=json"},
			want: "json",
		},
		{
			name: "no output flag",
			argv: []string{"finfocus", "overview"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectOutputFormatFromArgs(tt.argv)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldSuppressAuxiliaryOutput(t *testing.T) {
	assert.True(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview", "--output", "json"}))
	assert.True(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview", "--output=ndjson"}))
	assert.False(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview", "--output", "table"}))
	assert.False(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview"}))
}

func TestSuppressAuxOutputContextHelpers(t *testing.T) {
	base := context.Background()
	assert.False(t, suppressAuxOutputFromContext(base))

	withSuppress := contextWithSuppressAuxOutput(base, true)
	assert.True(t, suppressAuxOutputFromContext(withSuppress))

	withNoSuppress := contextWithSuppressAuxOutput(base, false)
	assert.False(t, suppressAuxOutputFromContext(withNoSuppress))
}

// newResolverTestCmd builds a root carrying ax's persistent --format flag and a
// child with --output (default def) and a boolean --json flag.
func newResolverTestCmd(t *testing.T, def string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "finfocus"}
	root.PersistentFlags().String(axFormatFlag, "", "output format: json or human")
	child := &cobra.Command{Use: "child"}
	child.Flags().String("output", def, "output format")
	child.Flags().Bool("json", false, "json output")
	root.AddCommand(child)
	child.SetContext(context.Background())
	return child
}

func TestResolveOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		defaultVal string
		output     string // explicit --output; empty means not set
		format     string // explicit --format; empty means not set
		agentMode  string // AGENT_MODE value; empty means unset
		ctxMode    ax.Mode
		want       string
	}{
		{name: "piped call with no signals stays table", defaultVal: "table", want: "table"},
		{name: "config default kept", defaultVal: "ndjson", want: "ndjson"},
		{name: "format json selects json", defaultVal: "table", format: "json", want: "json"},
		{name: "format agent alias selects json", defaultVal: "table", format: "agent", want: "json"},
		{name: "format human keeps default", defaultVal: "table", format: "human", want: "table"},
		{name: "invalid format keeps default", defaultVal: "table", format: "bogus", want: "table"},
		{name: "AGENT_MODE=1 selects json", defaultVal: "table", agentMode: "1", want: "json"},
		{name: "AGENT_MODE=json selects json", defaultVal: "table", agentMode: "json", want: "json"},
		{name: "AGENT_MODE=0 keeps default", defaultVal: "table", agentMode: "0", want: "table"},
		{name: "invalid AGENT_MODE keeps default", defaultVal: "table", agentMode: "maybe", want: "table"},
		{name: "format human wins over AGENT_MODE", defaultVal: "table", format: "human", agentMode: "1",
			want: "table"},
		{name: "format json wins over AGENT_MODE=0", defaultVal: "table", format: "json", agentMode: "0",
			want: "json"},
		{name: "explicit output wins over format json", defaultVal: "table", output: "table", format: "json",
			want: "table"},
		{name: "explicit output wins over AGENT_MODE", defaultVal: "table", output: "table", agentMode: "1",
			want: "table"},
		{name: "explicit ndjson wins over format json", defaultVal: "table", output: "ndjson", format: "json",
			want: "ndjson"},
		{name: "explicit invalid output reaches validation", defaultVal: "table", output: "xml", format: "json",
			want: "xml"},
		{name: "TTY-derived json mode alone does not switch", defaultVal: "table", ctxMode: ax.ModeJSON,
			want: "table"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(agentModeEnv, tc.agentMode)
			cmd := newResolverTestCmd(t, tc.defaultVal)
			if tc.output != "" {
				require.NoError(t, cmd.Flags().Set("output", tc.output))
			}
			if tc.format != "" {
				require.NoError(t, cmd.Root().PersistentFlags().Set(axFormatFlag, tc.format))
			}
			if tc.ctxMode != "" {
				cmd.SetContext(ax.WithMode(cmd.Context(), tc.ctxMode))
			}
			current, err := cmd.Flags().GetString("output")
			require.NoError(t, err)

			assert.Equal(t, tc.want, resolveOutputFormat(cmd, "output", current))
		})
	}
}

func TestResolveJSONFlag(t *testing.T) {
	t.Setenv(agentModeEnv, "")
	tests := []struct {
		name   string
		json   string // explicit --json; empty means not set
		format string
		want   bool
	}{
		{name: "default is false", want: false},
		{name: "format json enables json", format: "json", want: true},
		{name: "explicit json true", json: "true", want: true},
		{name: "explicit json false wins over format json", json: "false", format: "json", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newResolverTestCmd(t, "table")
			if tc.json != "" {
				require.NoError(t, cmd.Flags().Set("json", tc.json))
			}
			if tc.format != "" {
				require.NoError(t, cmd.Root().PersistentFlags().Set(axFormatFlag, tc.format))
			}
			current, err := cmd.Flags().GetBool("json")
			require.NoError(t, err)

			assert.Equal(t, tc.want, resolveJSONFlag(cmd, "json", current))
		})
	}
}

func TestMachineOutputRequestedWithoutFormatFlag(t *testing.T) {
	t.Setenv(agentModeEnv, "")
	assert.False(t, machineOutputRequested(&cobra.Command{Use: "bare"}))

	t.Setenv(agentModeEnv, "1")
	assert.True(t, machineOutputRequested(&cobra.Command{Use: "bare"}))
}
