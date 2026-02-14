package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
)

// dismissParams holds the parameters for the dismiss subcommand.
type dismissParams struct {
	reason   string
	note     string
	force    bool
	planPath string
	adapter  string
}

// snoozeParams holds the parameters for the snooze subcommand.
type snoozeParams struct {
	until    string
	reason   string
	note     string
	force    bool
	planPath string
	adapter  string
}

// newRecommendationsDismissCmd creates the "dismiss" subcommand for permanently dismissing
// a recommendation with a reason.
func newRecommendationsDismissCmd() *cobra.Command {
	var params dismissParams

	cmd := &cobra.Command{
		Use:   "dismiss <recommendation-id>",
		Short: "Dismiss a cost optimization recommendation",
		Long: `Permanently dismiss a recommendation by ID with a reason.

When the connected plugin supports dismissal (DISMISS_RECOMMENDATIONS capability),
the DismissRecommendation RPC is called. The dismissal is always persisted locally
for client-side filtering.

Valid reasons:
  not-applicable        Recommendation doesn't apply to your situation
  already-implemented   You've already acted on this recommendation
  business-constraint   Business requirements prevent action
  technical-constraint  Technical limitations prevent action
  deferred              Will address later (use with snooze for auto-expiry)
  inaccurate            Recommendation data or savings estimate is wrong
  other                 Custom reason (requires --note)`,
		Example: `  # Dismiss with a reason
  finfocus cost recommendations dismiss rec-123abc --reason business-constraint --pulumi-json plan.json

  # Dismiss with a custom note
  finfocus cost recommendations dismiss rec-123abc --reason other --note "Intentional oversizing" --pulumi-json plan.json

  # Skip confirmation prompt
  finfocus cost recommendations dismiss rec-123abc --reason not-applicable --force --pulumi-json plan.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeDismiss(cmd, args[0], params)
		},
	}

	cmd.Flags().StringVarP(&params.reason, "reason", "r", "", "Dismissal reason (required)")
	cmd.Flags().StringVarP(&params.note, "note", "n", "", "Free-text explanation (required for 'other' reason)")
	cmd.Flags().BoolVarP(&params.force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&params.planPath, "pulumi-json", "", "Path to Pulumi preview JSON (for plugin communication)")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "Use specific adapter plugin")

	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

// newRecommendationsSnoozeCmd creates the "snooze" subcommand for temporarily dismissing
// a recommendation until a future date.
func newRecommendationsSnoozeCmd() *cobra.Command {
	var params snoozeParams

	cmd := &cobra.Command{
		Use:   "snooze <recommendation-id>",
		Short: "Snooze a recommendation until a future date",
		Long: `Temporarily dismiss a recommendation until a specific date.
The recommendation automatically reappears when the snooze date passes.

Supports direct transitions: a dismissed recommendation can be snoozed directly,
and a snoozed recommendation can have its expiry updated by re-snoozing.`,
		Example: `  # Snooze until a specific date
  finfocus cost recommendations snooze rec-456def --until 2026-04-01 --pulumi-json plan.json

  # Snooze with reason and note
  finfocus cost recommendations snooze rec-456def --until 2026-07-01 --reason deferred \
    --note "Scheduled for Q2 review" --pulumi-json plan.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSnooze(cmd, args[0], params)
		},
	}

	cmd.Flags().StringVar(&params.until, "until", "", "Snooze until date (required, YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVarP(&params.reason, "reason", "r", "deferred", "Dismissal reason")
	cmd.Flags().StringVarP(&params.note, "note", "n", "", "Free-text explanation")
	cmd.Flags().BoolVarP(&params.force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&params.planPath, "pulumi-json", "", "Path to Pulumi preview JSON (for plugin communication)")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "Use specific adapter plugin")

	_ = cmd.MarkFlagRequired("until")

	return cmd
}

// executeDismiss handles the dismiss subcommand logic.
func executeDismiss(cmd *cobra.Command, recommendationID string, params dismissParams) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	// Validate reason
	if _, err := proto.ParseDismissalReason(params.reason); err != nil {
		return fmt.Errorf("invalid reason: %w", err)
	}

	// Validate "other" requires --note
	if strings.EqualFold(params.reason, "other") && params.note == "" {
		return errors.New("--note is required when reason is 'other'")
	}

	// Confirmation prompt
	if !params.force {
		reasonLabel := proto.DismissalReasonLabel(mustParseDismissalReason(params.reason))
		cmd.PrintErrf("Dismiss recommendation %s?\n", recommendationID)
		cmd.PrintErrf("  Reason: %s\n", reasonLabel)
		if params.note != "" {
			cmd.PrintErrf("  Note: %s\n", params.note)
		}
		cmd.PrintErrln()
		if !confirmPrompt(cmd, "Continue? [y/N]: ") {
			cmd.PrintErrln("Dismissal cancelled.")
			return nil
		}
	}

	// Build dismiss request
	req := engine.DismissRequest{
		RecommendationID: recommendationID,
		Reason:           params.reason,
		CustomReason:     params.note,
	}

	// Load dismissal store
	store, err := loadDismissalStore()
	if err != nil {
		return err
	}

	// Create engine with or without plugins
	eng, cleanup, engineErr := createDismissEngine(ctx, params.planPath, params.adapter)
	if engineErr != nil {
		return engineErr
	}
	defer cleanup()

	// Execute dismissal
	result, err := eng.DismissRecommendation(ctx, store, req)
	if err != nil {
		return fmt.Errorf("dismissing recommendation: %w", err)
	}

	// Render result
	renderDismissResult(cmd, result)

	log.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "dismiss").
		Str("recommendation_id", recommendationID).
		Bool("plugin_dismissed", result.PluginDismissed).
		Bool("local_persisted", result.LocalPersisted).
		Msg("recommendation dismissed")

	return nil
}

// executeSnooze handles the snooze subcommand logic.
func executeSnooze(cmd *cobra.Command, recommendationID string, params snoozeParams) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	// Parse the --until date
	expiresAt, err := parseSnoozeDate(params.until)
	if err != nil {
		return err
	}

	// Validate future date
	if !expiresAt.After(time.Now()) {
		return fmt.Errorf("snooze date must be in the future: %s", params.until)
	}

	// Validate reason
	if _, reasonErr := proto.ParseDismissalReason(params.reason); reasonErr != nil {
		return fmt.Errorf("invalid reason: %w", reasonErr)
	}

	// Confirmation prompt
	if !params.force {
		reasonLabel := proto.DismissalReasonLabel(mustParseDismissalReason(params.reason))
		cmd.PrintErrf("Snooze recommendation %s?\n", recommendationID)
		cmd.PrintErrf("  Until: %s\n", expiresAt.Format("2006-01-02"))
		cmd.PrintErrf("  Reason: %s\n", reasonLabel)
		if params.note != "" {
			cmd.PrintErrf("  Note: %s\n", params.note)
		}
		cmd.PrintErrln()
		if !confirmPrompt(cmd, "Continue? [y/N]: ") {
			cmd.PrintErrln("Snooze cancelled.")
			return nil
		}
	}

	// Build dismiss request with expiry
	req := engine.DismissRequest{
		RecommendationID: recommendationID,
		Reason:           params.reason,
		CustomReason:     params.note,
		ExpiresAt:        &expiresAt,
	}

	// Load dismissal store
	store, err := loadDismissalStore()
	if err != nil {
		return err
	}

	// Create engine with or without plugins
	eng, cleanup, engineErr := createDismissEngine(ctx, params.planPath, params.adapter)
	if engineErr != nil {
		return engineErr
	}
	defer cleanup()

	// Execute snooze (dismiss with expiry)
	result, err := eng.DismissRecommendation(ctx, store, req)
	if err != nil {
		return fmt.Errorf("snoozing recommendation: %w", err)
	}

	// Render result
	renderSnoozeResult(cmd, result, expiresAt)

	log.Info().
		Ctx(ctx).
		Str("component", "cli").
		Str("operation", "snooze").
		Str("recommendation_id", recommendationID).
		Str("until", expiresAt.Format(time.RFC3339)).
		Bool("plugin_dismissed", result.PluginDismissed).
		Msg("recommendation snoozed")

	return nil
}

// loadDismissalStore creates and loads the dismissal store.
func loadDismissalStore() (*config.DismissalStore, error) {
	store, err := config.NewDismissalStore("")
	if err != nil {
		return nil, fmt.Errorf("creating dismissal store: %w", err)
	}

	if loadErr := store.Load(); loadErr != nil {
		if errors.Is(loadErr, config.ErrStoreCorrupted) {
			return nil, fmt.Errorf("dismissal state file is corrupted; "+
				"remove or fix %s to continue: %w", store.FilePath(), loadErr)
		}
		// Other load errors (e.g., permissions) are fatal
		return nil, fmt.Errorf("loading dismissal store: %w", loadErr)
	}

	return store, nil
}

// createDismissEngine creates an engine instance, optionally with plugin connections.
// a non-nil error if plugin initialization fails.
func createDismissEngine(
	ctx context.Context,
	planPath string,
	adapter string,
) (*engine.Engine, func(), error) {
	if planPath == "" {
		// No plugins needed - local-only dismissal
		return engine.New(nil, nil), func() {}, nil
	}

	// Open plugin connections using the shared helper
	audit := newAuditContext(ctx, "dismiss recommendation", map[string]string{
		"pulumi_json": planPath,
	})

	clients, cleanup, err := openPlugins(ctx, adapter, audit)
	if err != nil {
		return nil, nil, fmt.Errorf("opening plugins: %w", err)
	}

	return engine.New(clients, nil).
		WithRouter(createRouterForEngine(ctx, clients)), cleanup, nil
}

// parseSnoozeDate parses a date string supporting both YYYY-MM-DD and RFC3339 formats.
// Date-only inputs are parsed in the local timezone and set to end-of-day (23:59:59).
func parseSnoozeDate(s string) (time.Time, error) {
	// Try YYYY-MM-DD first (parse in local timezone, set to end-of-day)
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t.Add(23*time.Hour + 59*time.Minute + 59*time.Second), nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid date format %q (expected YYYY-MM-DD or RFC3339)", s)
}

// confirmPrompt displays a prompt and reads user confirmation.
func confirmPrompt(cmd *cobra.Command, prompt string) bool {
	cmd.PrintErr(prompt)

	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return false
	}

	response := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return response == "y" || response == "yes"
}

// mustParseDismissalReason parses a reason string, panicking on error.
// Only use when the reason has already been validated.
func mustParseDismissalReason(reason string) pbc.DismissalReason {
	r, err := proto.ParseDismissalReason(reason)
	if err != nil {
		panic(fmt.Sprintf("mustParseDismissalReason: invalid reason %q: %v", reason, err))
	}
	return r
}

// renderDismissResult outputs the dismiss result to the command.
func renderDismissResult(cmd *cobra.Command, result *engine.DismissResult) {
	cmd.Printf("Recommendation %s dismissed.\n", result.RecommendationID)

	if result.PluginDismissed {
		cmd.Printf("  Plugin: %s accepted the dismissal", result.PluginName)
		if result.PluginMessage != "" {
			cmd.Printf(" (%s)", result.PluginMessage)
		}
		cmd.Println()
	}

	if result.LocalPersisted {
		cmd.Println("  Local state updated.")
	}

	if result.Warning != "" {
		cmd.PrintErrf("  Warning: %s\n", result.Warning)
	}
}

// renderSnoozeResult outputs the snooze result to the command.
func renderSnoozeResult(cmd *cobra.Command, result *engine.DismissResult, until time.Time) {
	cmd.Printf("Recommendation %s snoozed until %s.\n",
		result.RecommendationID, until.Format("2006-01-02"))

	if result.PluginDismissed {
		cmd.Printf("  Plugin: %s accepted the snooze", result.PluginName)
		if result.PluginMessage != "" {
			cmd.Printf(" (%s)", result.PluginMessage)
		}
		cmd.Println()
	}

	if result.LocalPersisted {
		cmd.Println("  Local state updated.")
	}

	if result.Warning != "" {
		cmd.PrintErrf("  Warning: %s\n", result.Warning)
	}
}