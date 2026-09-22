package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
)

// newRecommendationsUndismissCmd creates the "undismiss" subcommand for re-enabling
// a previously dismissed or snoozed recommendation.
func newRecommendationsUndismissCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "undismiss <recommendation-id>",
		Short: "Re-enable a dismissed or snoozed recommendation",
		Long: `Remove a dismissal or snooze from a recommendation so it reappears
in the default recommendation listing.

This operates on local state only and does not require plugin connections.`,
		Example: `  # Re-enable a dismissed recommendation
  finfocus cost recommendations undismiss rec-123abc

  # Skip confirmation prompt
  finfocus cost recommendations undismiss rec-123abc --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeUndismiss(cmd, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// executeUndismiss handles the undismiss subcommand logic.
func executeUndismiss(cmd *cobra.Command, recommendationID string, force bool) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	// Unify --force and global --yes
	ctx = ax.WithApproval(ctx, force || ax.ApprovalFromContext(ctx))

	// Get confirmation status
	confirmSubject := fmt.Sprintf("undismiss recommendation %s", recommendationID)
	outcome, confirmErr := ax.Confirm(ctx, confirmSubject)
	if confirmErr != nil {
		return confirmErr
	}

	// Handle confirmation outcomes
	switch outcome {
	case ax.ConfirmationBlocked:
		// ax.Confirm pairs Blocked with a non-nil error, which the check above
		// already returned, so this branch is unreachable today. Fail closed if
		// that contract ever changes: returning confirmErr here would be nil and
		// would skip the mutation while exiting 0.
		return errors.New("confirmation required: re-run with --yes")
	case ax.ConfirmationPromptRequired:
		// Need to do interactive prompt
		cmd.PrintErrf("Undismiss recommendation %s?\n", recommendationID)
		cmd.PrintErrln()
		if !confirmWithReader(cmd, "Continue? [y/N]: ") {
			cmd.PrintErrln("Undismiss cancelled.")
			return nil
		}
	case ax.ConfirmationApproved:
		// Already approved, proceed
	}

	// Load dismissal store
	store, err := loadDismissalStore()
	if err != nil {
		return fmt.Errorf("failed to load dismissal store: %w", err)
	}

	// Create engine (no plugins needed for undismiss)
	eng := engine.New(nil, nil)

	// Commit: execute the undismiss (wrapping in ax.Perform for dry-run support)
	commit := func(ctx2 context.Context) error {
		// Execute undismiss
		result, err := eng.UndismissRecommendation(ctx2, store, recommendationID)
		if err != nil {
			return fmt.Errorf("undismissing recommendation: %w", err)
		}

		// Render result
		if result.WasDismissed {
			cmd.Printf("Recommendation %s has been undismissed.\n", result.RecommendationID)
		} else {
			cmd.Printf("Recommendation %s was not dismissed.\n", result.RecommendationID)
		}

		log.Info().
			Ctx(ctx2).
			Str("component", "cli").
			Str("operation", "undismiss").
			Str("recommendation_id", recommendationID).
			Bool("was_dismissed", result.WasDismissed).
			Msg("undismiss complete")

		return nil
	}

	rehearse := func(_ context.Context) error {
		cmd.Printf("Would %s\n", confirmSubject)
		return nil
	}

	return ax.Perform(ctx, rehearse, commit)
}
