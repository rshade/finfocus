package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// T026: Unit tests for history CLI subcommand.

// T026: Test history command creation.
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestNewRecommendationsHistoryCmd(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	historySub := findSubcommand(cmd, "history")
	assert.NotNil(t, historySub, "history subcommand should exist")
	assert.Equal(t, "history", historySub.Name())
}

// T026: Test history command flags.
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestHistoryCmd_Flags(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	// Check --output flag
	outputFlag := historySub.Flags().Lookup("output")
	require.NotNil(t, outputFlag, "output flag should exist")
	assert.Equal(t, "table", outputFlag.DefValue)
}

// T026: Test history requires recommendation-id positional arg.
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestHistoryCmd_RequiresRecommendationID(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Try to execute history without recommendation ID
	cmd.SetArgs([]string{"history"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// T026: Test history default output is table.
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestHistoryCmd_DefaultOutputTable(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	outputFlag := historySub.Flags().Lookup("output")
	require.NotNil(t, outputFlag)
	assert.Equal(t, "table", outputFlag.DefValue)
}

// T026: Test history --output flag parsing.
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestHistoryCmd_OutputFlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"table output", "table", "table"},
		{"json output", "json", "json"},
		{"ndjson output", "ndjson", "ndjson"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewCostRecommendationsCmd()
			historySub := findSubcommand(cmd, "history")
			require.NotNil(t, historySub)

			err := historySub.Flags().Parse([]string{"--output", tt.value})
			require.NoError(t, err)

			val, err := historySub.Flags().GetString("output")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, val)
		})
	}
}

// T026: Test history command Use field contains recommendation-id.
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestHistoryCmd_UseField(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	assert.Contains(t, historySub.Use, "history")
	assert.Contains(t, historySub.Use, "recommendation-id")
}

// T026: Test history has descriptive help.
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestHistoryCmd_Help(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	assert.NotEmpty(t, historySub.Short)
	assert.NotEmpty(t, historySub.Long)
	assert.NotEmpty(t, historySub.Example)
	assert.Contains(t, historySub.Long, "installed plugins")
	assert.NotContains(t, historySub.Long, "local state only")
}

// T026: Test history does not require --pulumi-json (local state only).
//
//nolint:paralleltest // opens the default BoltDB file under ~/.finfocus, which is locked per process and shared by every test (via NewCostRecommendationsCmd)
func TestHistoryCmd_NoPluginConnectionRequired(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	// History should NOT have pulumi-json flag (operates locally)
	pulumiFlag := historySub.Flags().Lookup("pulumi-json")
	assert.Nil(t, pulumiFlag, "history should not require pulumi-json flag")

	// History should NOT have adapter flag
	adapterFlag := historySub.Flags().Lookup("adapter")
	assert.Nil(t, adapterFlag, "history should not require adapter flag")
}

// T026: Test history invalid output format.
func TestHistoryCmd_InvalidOutputFormat(t *testing.T) {
	// Isolate the dismissal store so the result never depends on state left in
	// the real ~/.finfocus by other tests or by the developer's own runs.
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Execute history with invalid output format
	cmd.SetArgs([]string{"history", "rec-123", "--output", "xml"})
	err := cmd.Execute()

	// The format is rejected before any state is loaded, so this holds even
	// though rec-123 has no recorded history in the isolated store.
	require.Error(t, err, "invalid output format 'xml' should produce an error")
	assert.Contains(t, err.Error(), "unsupported output format: xml")
}

type historyCommandPlugin struct {
	proto.CostSourceClient

	events []proto.RecommendationLifecycleEvent
}

func (p *historyCommandPlugin) GetRecommendationHistory(
	context.Context,
	*proto.GetRecommendationHistoryRequest,
	...grpc.CallOption,
) (*proto.GetRecommendationHistoryResponse, error) {
	return &proto.GetRecommendationHistoryResponse{Events: p.events}, nil
}

// isolateHistoryHome points dismissal storage and plugin discovery at a temp dir.
func isolateHistoryHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_PLUGIN_DIR", filepath.Join(home, "plugins"))
	prev := config.GetResolvedProjectDir()
	config.SetResolvedProjectDir("")
	t.Cleanup(func() { config.SetResolvedProjectDir(prev) })
	return home
}

func seedLocalHistory(t *testing.T, home, id string, when time.Time) []byte {
	t.Helper()
	store, err := config.NewDismissalStore(filepath.Join(home, "dismissed.json"))
	require.NoError(t, err)
	require.NoError(t, store.Load())
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: id,
		Status:           config.StatusDismissed,
		Reason:           "business-constraint",
		DismissedAt:      when,
		History: []config.LifecycleEvent{{
			Action:    config.ActionDismissed,
			Reason:    "business-constraint",
			Timestamp: when,
		}},
	}))
	require.NoError(t, store.Save())
	body, err := os.ReadFile(store.FilePath())
	require.NoError(t, err)
	return body
}

//nolint:paralleltest // sets FINFOCUS_HOME and the resolved project directory
func TestHistoryCmd_LocalHistoryWithoutPlugins(t *testing.T) {
	home := isolateHistoryHome(t)
	when := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	before := seedLocalHistory(t, home, "rec-local", when)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"history", "rec-local"})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, outBuf.String(), "ACTOR")
	assert.Contains(t, outBuf.String(), "dismissed")
	assert.Contains(t, outBuf.String(), "business-constraint")
	after, err := os.ReadFile(filepath.Join(home, "dismissed.json"))
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

//nolint:paralleltest // sets FINFOCUS_HOME, the resolved project directory, and the history plugin opener
func TestHistoryCmd_MergesPluginActorAndCloses(t *testing.T) {
	home := isolateHistoryHome(t)
	when := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	before := seedLocalHistory(t, home, "rec-merge", when)
	closed := false
	cli.SetHistoryPluginOpenerForTest(t, func(context.Context) ([]*pluginhost.Client, func(), error) {
		return []*pluginhost.Client{{
			Name: "aws",
			API: &historyCommandPlugin{events: []proto.RecommendationLifecycleEvent{{
				Action:    "acknowledged",
				Reason:    "reviewed",
				Actor:     "ada@example.com",
				Timestamp: when.Add(time.Hour),
			}}},
		}}, func() { closed = true }, nil
	})

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"history", "rec-merge", "--output", "json"})
	require.NoError(t, cmd.Execute())

	assert.True(t, closed)
	assert.Contains(t, outBuf.String(), "ada@example.com")
	assert.Contains(t, outBuf.String(), "dismissed")
	assert.Contains(t, outBuf.String(), "acknowledged")
	after, err := os.ReadFile(filepath.Join(home, "dismissed.json"))
	require.NoError(t, err)
	assert.Equal(t, before, after)

	ndjson := cli.NewCostRecommendationsCmd()
	var ndBuf bytes.Buffer
	ndjson.SetOut(&ndBuf)
	ndjson.SetErr(&bytes.Buffer{})
	ndjson.SetArgs([]string{"history", "rec-merge", "--output", "ndjson"})
	require.NoError(t, ndjson.Execute())
	assert.Contains(t, ndBuf.String(), `"actor":"ada@example.com"`)
	assert.Contains(t, ndBuf.String(), `"recommendation_id":"rec-merge"`)
}

//nolint:paralleltest // sets FINFOCUS_HOME, the resolved project directory, and the history plugin opener
func TestHistoryCmd_PluginOpenFailureKeepsLocalHistory(t *testing.T) {
	home := isolateHistoryHome(t)
	when := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	seedLocalHistory(t, home, "rec-open", when)
	cli.SetHistoryPluginOpenerForTest(t, func(context.Context) ([]*pluginhost.Client, func(), error) {
		return nil, nil, errors.New("plugin directory unavailable")
	})

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"history", "rec-open"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, outBuf.String(), "dismissed")
	assert.Contains(t, outBuf.String(), "business-constraint")
}

//nolint:paralleltest // builds a command that touches process-wide config paths
func TestHistoryCmd_EmptyRecommendationID(t *testing.T) {
	isolateHistoryHome(t)
	cmd := cli.NewCostRecommendationsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"history", ""})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recommendation ID is required")
}
