package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/cloudhistory"
	"github.com/rshade/finfocus/internal/config"
)

const (
	importCloudHistoryFlag   = "import-cloud-history"
	importHistoryFromFlag    = "from"
	importHistoryAdapterFlag = "adapter"
)

func registerImportCloudHistoryFlags(cmd *cobra.Command) {
	cmd.Flags().
		Bool(importCloudHistoryFlag, false, "import Pulumi Cloud historical resource identities without calculating costs")
	for _, name := range []string{importHistoryFromFlag, "to", "stack"} {
		flag := cmd.Flags().Lookup(name)
		if flag != nil {
			flag.Usage = strings.Replace(
				flag.Usage,
				"only valid with --web",
				"only valid with --web or --import-cloud-history",
				1,
			)
		}
	}
}

func validateImportCloudHistoryFlags(cmd *cobra.Command) error {
	enabled, _ := cmd.Flags().GetBool(importCloudHistoryFlag)
	if !enabled {
		return nil
	}
	for _, name := range []string{"web", "mcp", "port", "no-browser", "pulumi-json", "pulumi-state", importHistoryAdapterFlag, "filter"} {
		if cmd.Flags().Changed(name) {
			return fmt.Errorf("--%s cannot be combined with --%s", importCloudHistoryFlag, name)
		}
	}
	return nil
}

func runImportCloudHistory(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if len(args) > 0 {
		return toValidationError(
			ctx,
			errors.New("--import-cloud-history cannot be combined with a subcommand"),
		)
	}
	cfg := config.GetGlobalConfig()
	if cfg.Cost.History.PulumiCloud.Enabled == config.Off {
		return toValidationError(
			ctx,
			//nolint:staticcheck // Contract C1 requires this exact user-facing message.
			errors.New(
				"Pulumi Cloud history is disabled; set cost.history.pulumi_cloud.enabled to auto or true",
			),
		)
	}
	format, _ := cmd.Flags().GetString("format")
	if format != "" && format != "json" && format != "human" {
		return toValidationError(ctx, errors.New("invalid format: use human or json"))
	}
	now := time.Now()
	from := now.AddDate(0, 0, -resolvedHistoryRetention(ctx, cfg))
	to := now
	for _, bound := range []struct {
		name   string
		target *time.Time
	}{{importHistoryFromFlag, &from}, {"to", &to}} {
		raw, _ := cmd.Flags().GetString(bound.name)
		if raw == "" {
			continue
		}
		parsed, err := ParseTime(raw)
		if err != nil {
			return toValidationError(ctx, fmt.Errorf("invalid --%s date: %w", bound.name, err))
		}
		*bound.target = parsed
	}
	if !to.After(from) {
		return toValidationError(ctx, errors.New("'to' date must be after 'from' date"))
	}
	store, cleanup, storeErr := initHistoryFromConfigWithError(
		ctx,
		cfg,
		config.GetResolvedProjectDir(),
	)
	defer cleanup()
	deps, req, err := cloudHistoryRequest(cmd, cfg, store, from, to)
	if err != nil {
		return err
	}
	deps.StoreError = storeErr
	req.Mode = config.Required
	// Use the same instant for the default range and retention clamp.
	deps.Now = func() time.Time { return now }
	result, importErr := cloudhistory.Import(ctx, deps, req)
	if result.Clamped {
		cmd.PrintErrln("Note: --from was moved to the history retention cutoff.")
	}
	if format == "json" {
		if err = json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
			return fmt.Errorf("writing Cloud history summary: %w", err)
		}
	} else if err = printCloudHistorySummary(cmd.OutOrStdout(), result); err != nil {
		return err
	}
	if importErr != nil {
		var required *cloudhistory.RequiredError
		if errors.As(importErr, &required) && required.Validation &&
			required.Reason == "history_disabled" {
			return toValidationError(ctx, importErr)
		}
		return importErr
	}
	return nil
}

func printCloudHistorySummary(w io.Writer, result cloudhistory.Result) error {
	_, err := fmt.Fprintf(
		w,
		"Updates listed: %d\nSkipped (not state changing): %d\nServed from cache: %d\nFetched: %d\nIdentities recorded: %d\nUnreadable: %d\n",
		result.Listed,
		result.Skipped,
		result.FromCache,
		result.Fetched,
		result.Recorded,
		result.Unreadable,
	)
	if err != nil {
		return fmt.Errorf("writing Cloud history summary: %w", err)
	}
	return nil
}
