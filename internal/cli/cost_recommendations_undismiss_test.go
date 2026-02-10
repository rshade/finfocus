package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// T022: Unit tests for undismiss CLI subcommand.

// T022: Test undismiss command creation.
func TestNewRecommendationsUndismissCmd(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	undismissSub := findSubcommand(cmd, "undismiss")
	assert.NotNil(t, undismissSub, "undismiss subcommand should exist")
	assert.Equal(t, "undismiss", undismissSub.Name())
}

// T022: Test undismiss command flags.
func TestUndismissCmd_Flags(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	undismissSub := findSubcommand(cmd, "undismiss")
	require.NotNil(t, undismissSub)

	// Check --force flag
	forceFlag := undismissSub.Flags().Lookup("force")
	require.NotNil(t, forceFlag, "force flag should exist")
	assert.Equal(t, "false", forceFlag.DefValue)
}

// T022: Test undismiss requires recommendation-id positional arg.
func TestUndismissCmd_RequiresRecommendationID(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Try to execute undismiss without recommendation ID
	cmd.SetArgs([]string{"undismiss"})
	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// T022: Test --force flag parsing.
func TestUndismissCmd_ForceFlag(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	undismissSub := findSubcommand(cmd, "undismiss")
	require.NotNil(t, undismissSub)

	// Parse flags
	err := undismissSub.Flags().Parse([]string{"--force"})
	require.NoError(t, err)

	forceVal, err := undismissSub.Flags().GetBool("force")
	require.NoError(t, err)
	assert.True(t, forceVal)
}

// T022: Test undismiss command Use field contains recommendation-id.
func TestUndismissCmd_UseField(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	undismissSub := findSubcommand(cmd, "undismiss")
	require.NotNil(t, undismissSub)

	assert.Contains(t, undismissSub.Use, "undismiss")
	assert.Contains(t, undismissSub.Use, "recommendation-id")
}

// T022: Test undismiss has descriptive help.
func TestUndismissCmd_Help(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	undismissSub := findSubcommand(cmd, "undismiss")
	require.NotNil(t, undismissSub)

	assert.NotEmpty(t, undismissSub.Short)
	assert.NotEmpty(t, undismissSub.Long)
	assert.NotEmpty(t, undismissSub.Example)
}

// T022: Test undismiss does not require --pulumi-json (local state only).
func TestUndismissCmd_NoPluginConnectionRequired(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	undismissSub := findSubcommand(cmd, "undismiss")
	require.NotNil(t, undismissSub)

	// Undismiss should NOT have pulumi-json flag (operates locally)
	pulumiFlag := undismissSub.Flags().Lookup("pulumi-json")
	assert.Nil(t, pulumiFlag, "undismiss should not require pulumi-json flag")

	// Undismiss should NOT have adapter flag
	adapterFlag := undismissSub.Flags().Lookup("adapter")
	assert.Nil(t, adapterFlag, "undismiss should not require adapter flag")
}
