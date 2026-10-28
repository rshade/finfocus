package cli

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/cloudhistory"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

func resolvedHistoryRetention(ctx context.Context, cfg *config.Config) int {
	days := cfg.Cost.History.GetRetentionDays()
	if raw := os.Getenv(config.HistoryEnvRetentionDays); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			return parsed
		}
		logging.FromContext(ctx).
			Warn().
			Str("component", "history").
			Str("operation", "resolve_retention").
			Msg("invalid history retention days env var, ignoring")
	}
	return days
}

func importActualCloudHistory(
	cmd *cobra.Command,
	store history.Store,
	from, to time.Time,
	storeErrors ...error,
) (cloudhistory.Result, error) {
	cfg := config.GetGlobalConfig()
	if cfg.Cost.History.PulumiCloud.Enabled == config.Off {
		return cloudhistory.Result{}, nil
	}
	deps, req, err := cloudHistoryRequest(cmd, cfg, store, from, to)
	if len(storeErrors) > 0 {
		deps.StoreError = storeErrors[0]
	}
	if err != nil {
		return cloudhistory.Result{}, err
	}
	return cloudhistory.Import(cmd.Context(), deps, req)
}

func cloudHistoryRequest(
	cmd *cobra.Command,
	cfg *config.Config,
	store history.Store,
	from, to time.Time,
) (cloudhistory.Deps, cloudhistory.Request, error) {
	ctx := cmd.Context()
	deps := cloudhistory.Deps{Store: store, Logger: *logging.FromContext(ctx)}
	req := cloudhistory.Request{
		Mode:          cfg.Cost.History.PulumiCloud.Enabled,
		From:          from,
		To:            to,
		RetentionDays: resolvedHistoryRetention(ctx, cfg),
	}
	if store == nil || !store.IsEnabled() {
		return deps, req, nil
	}
	cache, ok := store.(history.CloudCache)
	if !ok {
		return deps, req, errors.New("history store does not support Cloud cache")
	}
	deps.Cache = cache
	// --project-dir resolves configuration only. Enrichment detects the project
	// from the working directory, so the import must record under the same one.
	project, err := pulumi.FindProject(".")
	if err != nil {
		//nolint:nilerr // Import detection classifies unavailable project context according to the selected mode.
		return deps, req, nil
	}
	stackFlag, _ := cmd.Flags().GetString("stack")
	if stackFlag == "" {
		stackFlag, _ = pulumi.GetCurrentStack(ctx, project)
	}
	stack, err := pulumi.LoadStackContext(project, stackFlag)
	if err != nil {
		//nolint:nilerr // Import detection classifies unavailable stack context according to the selected mode.
		return deps, req, nil
	}
	req.StackContext = history.StackContext{Project: stack.ProjectName, Stack: stack.Stack}
	session, fail := esc.DiscoverSession(os.Getenv, stack.BackendURL, stack.Stack)
	if fail != nil {
		// Keep transport classification for an explicitly selected backend even
		// when shared discovery rejects its scheme. No saved token is copied.
		if backend := os.Getenv("PULUMI_BACKEND_URL"); backend != "" {
			req.Session = esc.Session{Backend: backend, Token: os.Getenv("PULUMI_ACCESS_TOKEN")}
		}
		return deps, req, nil
	}
	req.Session = session
	stackName := stack.Stack
	if parts := strings.Split(stackName, "/"); len(parts) > 1 {
		stackName = parts[len(parts)-1]
	}
	req.Stack = pulumicloud.StackRef{Org: session.Org, Project: stack.ProjectName, Stack: stackName}
	return deps, req, nil
}

func reportActualCloudHistory(
	cmd *cobra.Command,
	mode config.PulumiCloudMode,
	result cloudhistory.Result,
	verbose bool,
) error {
	if mode == config.Off {
		return nil
	}
	if result.Clamped {
		cmd.PrintErrln("Note: --from was moved to the history retention cutoff.")
	}
	if result.Reason == "history_locked" {
		cmd.PrintErrln("Warning: Cloud history import skipped because history.db is locked.")
	}
	if result.Reason != "" {
		return nil
	}
	if verbose {
		return printCloudHistorySummary(cmd.ErrOrStderr(), result)
	}
	logging.FromContext(cmd.Context()).Debug().
		Str("component", "cloudhistory").Str("operation", "import").
		Int("listed", result.Listed).Int("skipped", result.Skipped).
		Int("from_cache", result.FromCache).Int("fetched", result.Fetched).
		Int("recorded", result.Recorded).Int("unreadable", result.Unreadable).
		Msg("Cloud history import complete")
	return nil
}

// inputStackMismatch reports whether the loaded resources name a different
// stack than the one Cloud and local history resolve. An input with no
// parseable URN, or no resolved stack, gives nothing to compare.
func inputStackMismatch(resources []engine.ResourceDescriptor, stack history.StackContext) bool {
	want := stack.Stack[strings.LastIndex(stack.Stack, "/")+1:]
	if want == "" {
		return false
	}
	for _, resource := range resources {
		urn, _ := resource.Properties[engine.PropertyPulumiURN].(string)
		if urn == "" {
			urn = resource.ID
		}
		rest, ok := strings.CutPrefix(urn, "urn:pulumi:")
		if !ok {
			continue
		}
		if got, _, found := strings.Cut(rest, "::"); found {
			return got != want
		}
	}
	return false
}

type historyStackContextKey struct{}

// actualHistoryContext pins an explicit stack for recording and enrichment.
func actualHistoryContext(cmd *cobra.Command) context.Context {
	ctx := cmd.Context()
	stack, _ := cmd.Flags().GetString("stack")
	if stack == "" {
		return ctx
	}
	project, err := pulumi.FindProject(".")
	if err != nil {
		return ctx
	}
	name, err := pulumi.GetProjectName(project)
	if err != nil {
		return ctx
	}
	ctx = context.WithValue(ctx, historyStackContextKey{}, history.StackContext{Project: name, Stack: stack})
	cmd.SetContext(ctx)
	return ctx
}

func runActualCloudHistory(
	cmd *cobra.Command,
	store history.Store,
	from, to time.Time,
	storeErr error,
	verbose bool,
) (cloudhistory.Result, error) {
	result, err := importActualCloudHistory(cmd, store, from, to, storeErr)
	if err != nil {
		var required *cloudhistory.RequiredError
		if errors.As(err, &required) && required.Validation {
			return result, toValidationError(cmd.Context(), err)
		}
		return result, err
	}
	err = reportActualCloudHistory(cmd, config.GetGlobalConfig().Cost.History.PulumiCloud.Enabled, result, verbose)
	return result, err
}

func newActualEngineWithHistory(
	ctx context.Context,
	cmd *cobra.Command,
	clients []*pluginhost.Client,
) (*engine.Engine, history.Store, cache.Cache, func(), error) {
	cfg := config.GetGlobalConfig()
	eng, cacheStore, cacheCleanup := newEngineWithCache(ctx, cmd, clients, nil, cfg)
	store, historyCleanup, err := initHistoryFromConfigWithError(ctx, cfg, config.GetResolvedProjectDir())
	return eng, store, cacheStore, func() { historyCleanup(); cacheCleanup() }, err
}

func markHistoryImportStatus(
	cmd *cobra.Command,
	resultWithErrors *engine.CostResultWithErrors,
	historyImport cloudhistory.Result,
) {
	if historyImport.Incomplete() {
		cmd.PrintErrf(
			"Warning: Cloud history import has %d unreadable updates; totals may be incomplete.\n",
			historyImport.Unreadable,
		)
		status := &engine.HistoryImportStatus{Incomplete: true, UnreadableUpdates: historyImport.Unreadable}
		resultWithErrors.HistoryImport = status
		for i := range resultWithErrors.Results {
			resultWithErrors.Results[i].HistoryImport = status
		}
	}
}
