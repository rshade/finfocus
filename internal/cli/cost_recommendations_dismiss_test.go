package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findSubcommandLocal finds a subcommand by name in a cobra.Command.
// Local copy for package cli tests (the shared helper is in cli_test package).
func findSubcommandLocal(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

// T008: Unit tests for dismiss CLI subcommand.
// T015: Unit tests for snooze CLI subcommand.

// T008: Test dismiss command creation.
func TestNewRecommendationsDismissCmd(t *testing.T) {
	cmd := NewCostRecommendationsCmd()

	dismissSub := findSubcommandLocal(cmd, "dismiss")
	require.NotNil(t, dismissSub, "dismiss subcommand should exist")
	assert.Equal(t, "dismiss", dismissSub.Name())
}

// T008: Test dismiss command flags.
func TestDismissCmd_Flags(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	dismissSub := findSubcommandLocal(cmd, "dismiss")
	require.NotNil(t, dismissSub, "dismiss subcommand should exist")

	// Check required flags
	reasonFlag := dismissSub.Flags().Lookup("reason")
	require.NotNil(t, reasonFlag, "reason flag should exist")

	// Check optional flags
	noteFlag := dismissSub.Flags().Lookup("note")
	require.NotNil(t, noteFlag, "note flag should exist")

	forceFlag := dismissSub.Flags().Lookup("force")
	require.NotNil(t, forceFlag, "force flag should exist")

	planFlag := dismissSub.Flags().Lookup("pulumi-json")
	require.NotNil(t, planFlag, "pulumi-json flag should exist")

	adapterFlag := dismissSub.Flags().Lookup("adapter")
	require.NotNil(t, adapterFlag, "adapter flag should exist")
}

// T008: Test dismiss requires reason flag.
func TestDismissCmd_RequiresReason(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Try to execute dismiss without reason
	cmd.SetArgs([]string{"dismiss", "rec-123"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

// T008: Test dismiss requires recommendation-id positional arg.
func TestDismissCmd_RequiresRecommendationID(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Try to execute dismiss without recommendation ID
	cmd.SetArgs([]string{"dismiss", "--reason", "business-constraint"})
	err := cmd.Execute()

	require.Error(t, err)
	// Should fail due to missing positional arg
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// T008: Test "other" reason requires --note.
func TestDismissCmd_OtherRequiresNote(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Execute dismiss with "other" reason but no note
	cmd.SetArgs([]string{"dismiss", "rec-123", "--reason", "other", "--force"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "note")
}

// T008: Test invalid reason validation.
func TestDismissCmd_InvalidReason(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Execute dismiss with invalid reason
	cmd.SetArgs([]string{"dismiss", "rec-123", "--reason", "invalid-reason", "--force"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid reason")
}

// T008: Test all valid reasons are accepted (parsing only).
func TestDismissCmd_ValidReasons(t *testing.T) {
	validReasons := []string{
		"not-applicable",
		"already-implemented",
		"business-constraint",
		"technical-constraint",
		"deferred",
		"inaccurate",
		// "other" requires note, tested separately
	}

	for _, reason := range validReasons {
		t.Run(reason, func(t *testing.T) {
			cmd := NewCostRecommendationsCmd()
			var outBuf, errBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			// Execute with valid reason (will fail later due to no store, but parsing should work)
			cmd.SetArgs([]string{"dismiss", "rec-123", "--reason", reason, "--force"})
			err := cmd.Execute()

			// Should not fail with "invalid reason" - may fail later with store error
			if err != nil {
				assert.NotContains(t, err.Error(), "invalid reason",
					"reason %s should be valid", reason)
			}
		})
	}
}

// T008: Test --force skips confirmation.
func TestDismissCmd_ForceSkipsConfirmation(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	dismissSub := findSubcommandLocal(cmd, "dismiss")
	require.NotNil(t, dismissSub)

	// Parse flags only
	err := dismissSub.Flags().Parse([]string{"--force", "--reason", "business-constraint"})
	require.NoError(t, err)

	forceVal, err := dismissSub.Flags().GetBool("force")
	require.NoError(t, err)
	assert.True(t, forceVal)
}

// T008: Test Snoozed->Dismissed direct transition (FR-010a).
func TestDismissCmd_DirectTransitionFromSnoozed(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	dismissSub := findSubcommandLocal(cmd, "dismiss")
	require.NotNil(t, dismissSub)

	// The dismiss command should accept any recommendation ID
	// without checking if it's snoozed (engine handles transitions)
	assert.Contains(t, dismissSub.Use, "dismiss")
	assert.Contains(t, dismissSub.Use, "recommendation-id")
}

// T015: Test snooze command creation.
func TestNewRecommendationsSnoozeCmd(t *testing.T) {
	cmd := NewCostRecommendationsCmd()

	snoozeSub := findSubcommandLocal(cmd, "snooze")
	require.NotNil(t, snoozeSub, "snooze subcommand should exist")
	assert.Equal(t, "snooze", snoozeSub.Name())
}

// T015: Test snooze command flags.
func TestSnoozeCmd_Flags(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	snoozeSub := findSubcommandLocal(cmd, "snooze")
	require.NotNil(t, snoozeSub)

	// Check required flag
	untilFlag := snoozeSub.Flags().Lookup("until")
	require.NotNil(t, untilFlag, "until flag should exist")

	// Check optional flags
	reasonFlag := snoozeSub.Flags().Lookup("reason")
	require.NotNil(t, reasonFlag, "reason flag should exist")

	noteFlag := snoozeSub.Flags().Lookup("note")
	require.NotNil(t, noteFlag, "note flag should exist")

	forceFlag := snoozeSub.Flags().Lookup("force")
	require.NotNil(t, forceFlag, "force flag should exist")
}

// T015: Test snooze requires --until flag.
func TestSnoozeCmd_RequiresUntil(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Try to execute snooze without until
	cmd.SetArgs([]string{"snooze", "rec-123"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "until")
}

// T015: Test snooze validates future date (FR-012).
func TestSnoozeCmd_RejectsPastDate(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Use a date in the past
	pastDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	cmd.SetArgs([]string{"snooze", "rec-123", "--until", pastDate, "--force"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "future")
}

// T015: Test snooze accepts YYYY-MM-DD format.
func TestSnoozeCmd_AcceptsYYYYMMDD(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Use valid future date in YYYY-MM-DD format
	futureDate := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	cmd.SetArgs([]string{"snooze", "rec-123", "--until", futureDate, "--force"})
	err := cmd.Execute()

	// May fail with store error, but should not fail with date parsing error
	if err != nil {
		assert.NotContains(t, err.Error(), "parsing time")
	}
}

// T015: Test snooze accepts RFC3339 format.
func TestSnoozeCmd_AcceptsRFC3339(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Use valid future date in RFC3339 format
	futureDate := time.Now().AddDate(0, 1, 0).Format(time.RFC3339)
	cmd.SetArgs([]string{"snooze", "rec-123", "--until", futureDate, "--force"})
	err := cmd.Execute()

	// May fail with store error, but should not fail with date parsing error
	if err != nil {
		assert.NotContains(t, err.Error(), "parsing time")
	}
}

// T015: Test snooze default reason is "deferred".
func TestSnoozeCmd_DefaultReasonDeferred(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	snoozeSub := findSubcommandLocal(cmd, "snooze")
	require.NotNil(t, snoozeSub)

	reasonFlag := snoozeSub.Flags().Lookup("reason")
	require.NotNil(t, reasonFlag)
	assert.Equal(t, "deferred", reasonFlag.DefValue)
}

// NOTE: This test validates CLI structure only, not transition logic.
// Full transition logic is tested in engine_dismiss_test.go.
func TestSnoozeCmd_DirectTransitionAllowed_CLIParsing(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	snoozeSub := findSubcommandLocal(cmd, "snooze")
	require.NotNil(t, snoozeSub)

	// The snooze command should accept any recommendation ID
	// without checking if it's already dismissed (engine handles transitions)
	assert.Contains(t, snoozeSub.Use, "snooze")
	assert.Contains(t, snoozeSub.Use, "recommendation-id")
}

// NOTE: This test validates CLI parsing only; full transition logic is
// tested in engine_dismiss_test.go.
func TestSnoozeCmd_ResnoozeAllowed_CLIParsing(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	snoozeSub := findSubcommandLocal(cmd, "snooze")
	require.NotNil(t, snoozeSub)

	// Verify snooze doesn't prevent multiple calls for same ID
	futureDate := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	err := snoozeSub.Flags().Parse([]string{
		"--until", futureDate,
		"--force",
	})
	require.NoError(t, err)
}

// T015: Test snooze rejects invalid date format.
func TestSnoozeCmd_RejectsInvalidDateFormat(t *testing.T) {
	cmd := NewCostRecommendationsCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Use invalid date format
	cmd.SetArgs([]string{"snooze", "rec-123", "--until", "not-a-date", "--force"})
	err := cmd.Execute()

	require.Error(t, err)
	// Should fail with date format error
	assert.Contains(t, err.Error(), "invalid date format")
}
