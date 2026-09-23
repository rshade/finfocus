package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

// extractProviderFromType extracts the provider prefix from a Pulumi type token.
func extractProviderFromType(typeToken string) string {
	if idx := strings.Index(typeToken, ":"); idx > 0 {
		return typeToken[:idx]
	}
	return ""
}

// convertEngineStateToHistoryState converts engine.StateResource to
// history.StateResource for recording to the history store.
func convertEngineStateToHistoryState(resources []engine.StateResource) []history.StateResource {
	result := make([]history.StateResource, 0, len(resources))
	for _, r := range resources {
		if r.ID == "" {
			continue
		}
		result = append(result, history.StateResource{
			URN:      r.URN,
			CloudID:  r.ID,
			Type:     r.Type,
			Provider: extractProviderFromType(r.Type),
			Tags:     history.ExtractTagsFromProperties(r.Properties),
		})
	}
	return result
}

// recordHistorySnapshot is a fire-and-forget helper that records state resources
// to the history store. Errors are logged at WARN level but never returned.
func recordHistorySnapshot(ctx context.Context, store history.Store, resources []engine.StateResource) {
	if store == nil || !store.IsEnabled() {
		return
	}
	stackCtx, ok := detectHistoryStackContext(ctx)
	if !ok {
		return
	}
	log := logging.FromContext(ctx)
	writer := history.NewWriter(store, *log)
	stateResources := convertEngineStateToHistoryState(resources)
	writer.RecordStateSnapshot(stackCtx, stateResources)
}

// recordHistoryPlanLineage is a fire-and-forget helper that records plan lineage
// (replace/delete cloud IDs) to the history store. Only steps with non-empty
// cloud IDs are recorded.
func recordHistoryPlanLineage(ctx context.Context, store history.Store, planSteps []engine.PlanStep) {
	if store == nil || !store.IsEnabled() {
		return
	}
	stackCtx, ok := detectHistoryStackContext(ctx)
	if !ok {
		return
	}
	log := logging.FromContext(ctx)
	writer := history.NewWriter(store, *log)
	historySteps := convertEnginePlanStepsToHistoryPlanSteps(planSteps)
	if len(historySteps) > 0 {
		writer.RecordPlanLineage(stackCtx, historySteps)
	}
}

// convertEnginePlanStepsToHistoryPlanSteps converts engine.PlanStep to
// history.PlanStep for recording to the history store. Only steps with
// non-empty cloud IDs are included.
func convertEnginePlanStepsToHistoryPlanSteps(steps []engine.PlanStep) []history.PlanStep {
	result := make([]history.PlanStep, 0, len(steps))
	for _, s := range steps {
		if s.OldCloudID == "" && s.NewCloudID == "" {
			continue
		}
		result = append(result, history.PlanStep{
			Op:         s.Op,
			URN:        s.URN,
			Type:       s.Type,
			Provider:   extractProviderFromType(s.Type),
			OldCloudID: s.OldCloudID,
			NewCloudID: s.NewCloudID,
		})
	}
	return result
}

// detectHistoryStackContext attempts to detect the Pulumi project and stack
// for history scoping. Returns false if either project name or stack cannot
// be fully resolved, so callers can skip history operations rather than
// recording under a meaningless zero-value hash.
func detectHistoryStackContext(ctx context.Context) (history.StackContext, bool) {
	log := logging.FromContext(ctx)

	projectDir, err := pulumidetect.FindProject(".")
	if err != nil {
		log.Debug().Ctx(ctx).
			Str("component", "history").
			Msg("no Pulumi project detected, using empty stack context for history")
		return history.StackContext{}, false
	}

	projectName, nameErr := pulumidetect.GetProjectName(projectDir)
	if nameErr != nil {
		log.Debug().Ctx(ctx).Err(nameErr).
			Str("component", "history").
			Msg("could not read Pulumi project name, using empty stack context")
		return history.StackContext{}, false
	}

	stack, stackErr := pulumidetect.GetCurrentStack(ctx, projectDir)
	if stackErr != nil || stack == "" {
		log.Debug().Ctx(ctx).Err(stackErr).
			Str("component", "history").
			Msg("could not detect current stack, using empty stack context")
		return history.StackContext{}, false
	}

	return history.StackContext{
		Project: projectName,
		Stack:   stack,
	}, true
}

// initHistoryFromConfig initializes a history store using values from the
// provided configuration and environment variables.
//
// Resolution precedence for each setting:
//   - enabled: env var FINFOCUS_HISTORY_ENABLED > config > default (true)
//   - retention_days: env var FINFOCUS_HISTORY_RETENTION_DAYS > config > default (90)
//   - directory: env var FINFOCUS_HISTORY_DIR > config > ~/.finfocus/history
//
// Returns nil when history is disabled or initialization fails (logged at WARN).
// The caller should defer the returned cleanup function.
func initHistoryFromConfig(ctx context.Context, cfg *config.Config) (history.Store, func()) {
	log := logging.FromContext(ctx)
	noopCleanup := func() {}

	retentionDays := cfg.Cost.History.GetRetentionDays()
	// Use IsEnabled() which handles nil (omitted) vs explicit false correctly.
	enabled := cfg.Cost.History.IsEnabled()

	// Override with env vars
	if envEnabled := os.Getenv(config.HistoryEnvEnabled); envEnabled != "" {
		if parsed, err := strconv.ParseBool(envEnabled); err == nil {
			enabled = parsed
		} else {
			log.Warn().Ctx(ctx).
				Str("component", "history").
				Str("env_var", config.HistoryEnvEnabled).
				Str("value", envEnabled).
				Msg("invalid history enabled env var, ignoring")
		}
	}

	if envRetention := os.Getenv(config.HistoryEnvRetentionDays); envRetention != "" {
		if days, err := strconv.Atoi(envRetention); err == nil && days > 0 {
			retentionDays = days
		} else {
			log.Warn().Ctx(ctx).
				Str("component", "history").
				Str("env_var", config.HistoryEnvRetentionDays).
				Str("value", envRetention).
				Msg("invalid history retention days env var, ignoring")
		}
	}

	if !enabled {
		return nil, noopCleanup
	}

	historyDir := resolveHistoryDir(ctx, cfg)

	store, err := history.NewBoltStore(ctx, historyDir, true, retentionDays)
	if err != nil {
		if errors.Is(err, history.ErrHistoryLocked) {
			log.Warn().Ctx(ctx).
				Str("component", "history").
				Msg("history database locked, proceeding without history")
		} else {
			log.Warn().Ctx(ctx).Err(err).
				Str("component", "history").
				Msg("history initialization failed, proceeding without history")
		}
		return nil, noopCleanup
	}

	log.Debug().Ctx(ctx).
		Str("component", "history").
		Bool("enabled", true).
		Int("retention_days", retentionDays).
		Str("history_dir", historyDir).
		Msg("history store initialized")

	cleanup := func() {
		if closeErr := store.Close(); closeErr != nil {
			log.Warn().Ctx(ctx).Err(closeErr).
				Str("component", "history").
				Msg("failed to close history store")
		}
	}

	return store, cleanup
}

// resolveHistoryDir determines the history directory using the resolution chain:
// env var (FINFOCUS_HISTORY_DIR) → config setting → ~/.finfocus/history.
func resolveHistoryDir(ctx context.Context, cfg *config.Config) string {
	log := logging.FromContext(ctx)
	if dir := os.Getenv(config.HistoryEnvDir); dir != "" {
		return dir
	}
	if cfg.Cost.History.Directory != "" {
		return cfg.Cost.History.Directory
	}
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		abs, absErr := filepath.Abs(filepath.Join(".finfocus", "history"))
		if absErr != nil {
			abs = filepath.Join(os.TempDir(), "finfocus-history")
		}
		log.Warn().Ctx(ctx).Err(homeErr).
			Str("component", "history").
			Str("fallback_path", abs).
			Msg("failed to determine home directory, using absolute fallback history path")
		return abs
	}
	return filepath.Join(homeDir, ".finfocus", "history")
}
