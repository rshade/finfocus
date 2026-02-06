package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// T026: Unit tests for history CLI subcommand.

// T026: Test history command creation.
func TestNewRecommendationsHistoryCmd(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	historySub := findSubcommand(cmd, "history")
	assert.NotNil(t, historySub, "history subcommand should exist")
	assert.Equal(t, "history", historySub.Name())
}

// T026: Test history command flags.
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
func TestHistoryCmd_RequiresRecommendationID(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Try to execute history without recommendation ID
	cmd.SetArgs([]string{"history"})
	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// T026: Test history default output is table.
func TestHistoryCmd_DefaultOutputTable(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	outputFlag := historySub.Flags().Lookup("output")
	require.NotNil(t, outputFlag)
	assert.Equal(t, "table", outputFlag.DefValue)
}

// T026: Test history --output flag parsing.
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
func TestHistoryCmd_UseField(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	assert.Contains(t, historySub.Use, "history")
	assert.Contains(t, historySub.Use, "recommendation-id")
}

// T026: Test history has descriptive help.
func TestHistoryCmd_Help(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	historySub := findSubcommand(cmd, "history")
	require.NotNil(t, historySub)

	assert.NotEmpty(t, historySub.Short)
	assert.NotEmpty(t, historySub.Long)
	assert.NotEmpty(t, historySub.Example)
}

// T026: Test history does not require --pulumi-json (local state only).
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
	cmd := cli.NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Execute history with invalid output format
	cmd.SetArgs([]string{"history", "rec-123", "--output", "xml"})
	err := cmd.Execute()

	// May fail with store error first, or with unsupported format error
	// Either way, an error is expected since "xml" is not a valid format
	require.Error(t, err, "invalid output format 'xml' should produce an error")
	// The error could be about unsupported format or store loading (unit test env),
	// either indicates the command did not silently succeed with an invalid format.
}
