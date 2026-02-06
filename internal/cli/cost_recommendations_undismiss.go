package cli

import (
	"fmt"

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

	// Confirmation prompt
	if !force {
		cmd.PrintErrf("Undismiss recommendation %s?\n", recommendationID)
		cmd.PrintErrln()
		if !confirmPrompt(cmd, "Continue? [y/N]: ") {
			cmd.PrintErrln("Undismiss cancelled.")
			return nil
		}
	}

	// Load dismissal store
	store, err := loadDismissalStore()
	if err != nil {
		return fmt.Errorf("failed to load dismissal store: %w", err)
	}

	// Create engine (no plugins needed for undismiss)
	eng := engine.New(nil, nil)

	// Execute undismiss
	result, err := eng.UndismissRecommendation(ctx, store, recommendationID)
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
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "undismiss").
		Str("recommendation_id", recommendationID).
		Bool("was_dismissed", result.WasDismissed).
		Msg("undismiss complete")

	return nil
}
