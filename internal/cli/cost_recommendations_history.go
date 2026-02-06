package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
)

// newRecommendationsHistoryCmd creates the "history" subcommand for viewing
// the lifecycle history of a recommendation.
func newRecommendationsHistoryCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "history <recommendation-id>",
		Short: "View lifecycle history of a recommendation",
		Long: `Display the dismiss/snooze/undismiss history for a specific recommendation.
Shows all lifecycle events in chronological order.

This operates on local state only and does not require plugin connections.`,
		Example: `  # View history in table format
  finfocus cost recommendations history rec-123abc

  # View history as JSON
  finfocus cost recommendations history rec-123abc --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeHistory(cmd, args[0], output)
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "Output format: table, json, ndjson")

	return cmd
}

// executeHistory handles the history subcommand logic.
func executeHistory(cmd *cobra.Command, recommendationID string, output string) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	// Load dismissal store
	store, err := loadDismissalStore()
	if err != nil {
		return fmt.Errorf("load dismissal store: %w", err)
	}

	// Create engine (no plugins needed for history)
	eng := engine.New(nil, nil)

	// Get history
	events, err := eng.GetRecommendationHistory(ctx, store, recommendationID)
	if err != nil {
		return fmt.Errorf("getting recommendation history: %w", err)
	}

	if len(events) == 0 {
		cmd.Printf("No history found for recommendation %s.\n", recommendationID)
		return nil
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "history_retrieved").
		Str("recommendation_id", recommendationID).
		Int("event_count", len(events)).
		Msg("history retrieved")

	// Render based on output format
	switch output {
	case "json":
		return renderHistoryJSON(cmd, recommendationID, events)
	case "ndjson":
		return renderHistoryNDJSON(cmd, recommendationID, events)
	case "table":
		return renderHistoryTable(cmd, recommendationID, events)
	default:
		return fmt.Errorf("unsupported output format: %s", output)
	}
}

// renderHistoryTable renders lifecycle events as a table.
func renderHistoryTable(cmd *cobra.Command, recommendationID string, events []config.LifecycleEvent) error {
	cmd.Printf("History for recommendation %s:\n\n", recommendationID)

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)

	fmt.Fprintln(tw, "TIMESTAMP\tACTION\tREASON\tNOTE\tEXPIRES")
	fmt.Fprintln(tw, "---------\t------\t------\t----\t-------")

	for _, event := range events {
		expiresStr := ""
		if event.ExpiresAt != nil {
			expiresStr = event.ExpiresAt.Format("2006-01-02")
		}

		noteStr := event.CustomReason
		const maxNoteLen = 30
		if len(noteStr) > maxNoteLen {
			noteStr = noteStr[:maxNoteLen-3] + "..."
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			event.Timestamp.Format("2006-01-02 15:04:05"),
			string(event.Action),
			event.Reason,
			noteStr,
			expiresStr,
		)
	}

	return tw.Flush()
}

// historyJSONOutput represents the JSON output for history.
type historyJSONOutput struct {
	RecommendationID string                  `json:"recommendation_id"`
	Events           []config.LifecycleEvent `json:"events"`
	EventCount       int                     `json:"event_count"`
}

// historyNDJSONEvent wraps a lifecycle event with the recommendation ID for NDJSON output.
type historyNDJSONEvent struct {
	RecommendationID string                `json:"recommendation_id"`
	Event            config.LifecycleEvent `json:"event"`
}

// renderHistoryNDJSON renders lifecycle events as newline-delimited JSON.
func renderHistoryNDJSON(cmd *cobra.Command, recommendationID string, events []config.LifecycleEvent) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	for _, event := range events {
		if err := encoder.Encode(historyNDJSONEvent{
			RecommendationID: recommendationID,
			Event:            event,
		}); err != nil {
			return fmt.Errorf("encoding history NDJSON: %w", err)
		}
	}
	return nil
}

// renderHistoryJSON renders lifecycle events as JSON.
func renderHistoryJSON(cmd *cobra.Command, recommendationID string, events []config.LifecycleEvent) error {
	output := historyJSONOutput{
		RecommendationID: recommendationID,
		Events:           events,
		EventCount:       len(events),
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encoding history JSON: %w", err)
	}
	return nil
}
