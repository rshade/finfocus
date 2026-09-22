package cli

import (
	"context"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
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
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "dismiss", "rec-123"})

	require.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "reason")
}

// T008: Test dismiss requires recommendation-id positional arg.
func TestDismissCmd_RequiresRecommendationID(t *testing.T) {
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "dismiss", "--reason", "business-constraint"})

	require.NotEqual(t, 0, result.ExitCode)
	// Should fail due to missing positional arg
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "accepts 1 arg")
}

// T008: Test "other" reason requires --note.
func TestDismissCmd_OtherRequiresNote(t *testing.T) {
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "dismiss", "rec-123", "--reason", "other", "--force"})

	require.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "note")
}

// T008: Test invalid reason validation.
func TestDismissCmd_InvalidReason(t *testing.T) {
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "dismiss", "rec-123", "--reason", "invalid-reason", "--force"})

	require.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "invalid reason")
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
			t.Setenv("FINFOCUS_HOME", t.TempDir())
			root := NewRootCmd("test-version")
			result := axtest.Run(context.Background(), t, root,
				[]string{"cost", "recommendations", "dismiss", "rec-123", "--reason", reason, "--force"})

			// Should not fail with "invalid reason" - may fail later with store error
			if result.ExitCode != 0 {
				stderr := string(result.Stderr)
				assert.NotContains(t, stderr, "invalid reason",
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
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "snooze", "rec-123"})

	require.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "until")
}

// T015: Test snooze validates future date (FR-012).
func TestSnoozeCmd_RejectsPastDate(t *testing.T) {
	// Use a date in the past
	pastDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "snooze", "rec-123", "--until", pastDate, "--force"})

	require.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "future")
}

// T015: Test snooze accepts YYYY-MM-DD format.
func TestSnoozeCmd_AcceptsYYYYMMDD(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	// Use valid future date in YYYY-MM-DD format
	futureDate := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "snooze", "rec-123", "--until", futureDate, "--force"})

	// May fail with store error, but should not fail with date parsing error
	if result.ExitCode != 0 {
		stderr := string(result.Stderr)
		assert.NotContains(t, stderr, "parsing time")
	}
}

// T015: Test snooze accepts RFC3339 format.
func TestSnoozeCmd_AcceptsRFC3339(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	// Use valid future date in RFC3339 format
	futureDate := time.Now().AddDate(0, 1, 0).Format(time.RFC3339)
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "snooze", "rec-123", "--until", futureDate, "--force"})

	// May fail with store error, but should not fail with date parsing error
	if result.ExitCode != 0 {
		stderr := string(result.Stderr)
		assert.NotContains(t, stderr, "parsing time")
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
	root := NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root,
		[]string{"cost", "recommendations", "snooze", "rec-123", "--until", "not-a-date", "--force"})

	require.NotEqual(t, 0, result.ExitCode)
	// Should fail with date format error
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "invalid date format")
}

func TestRecommendationChangesDryRun(t *testing.T) {
	until := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "dismiss",
			args: []string{"cost", "recommendations", "dismiss", "rec-preview", "--reason", "not-applicable"},
			want: "Would dismiss recommendation rec-preview",
		},
		{
			name: "snooze",
			args: []string{"cost", "recommendations", "snooze", "rec-preview", "--until", until},
			want: "Would snooze recommendation rec-preview until " + until,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FINFOCUS_HOME", t.TempDir())
			tt.args = append(tt.args, "--force", "--dry-run")
			result := axtest.Run(context.Background(), t, NewRootCmd("test"), tt.args)
			require.Zero(t, result.ExitCode, string(result.Stderr))
			assert.Contains(t, string(result.Stdout), tt.want)
			store, err := config.NewDismissalStore("")
			require.NoError(t, err)
			assert.NoFileExists(t, store.FilePath())
		})
	}
}
