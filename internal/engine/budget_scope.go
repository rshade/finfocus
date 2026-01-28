package engine

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
)

// ScopeType identifies the category of a budget scope.
type ScopeType string

// Budget scope type constants.
const (
	// ScopeTypeGlobal represents the global budget that all resources count toward.
	ScopeTypeGlobal ScopeType = "global"

	// ScopeTypeProvider represents a per-cloud-provider budget (aws, gcp, azure).
	ScopeTypeProvider ScopeType = "provider"

	// ScopeTypeTag represents a tag-based budget with priority ordering.
	ScopeTypeTag ScopeType = "tag"

	// ScopeTypeType represents a per-resource-type budget (e.g., aws:ec2/instance).
	ScopeTypeType ScopeType = "type"
)

// percentageMultiplier converts decimal ratios to percentage values.
const percentageMultiplier = 100

// Default alert thresholds used when no thresholds are configured.
const (
	defaultThresholdInfo     = 50
	defaultThresholdWarning  = 80
	defaultThresholdCritical = 100
)

// String returns the string representation of the scope type.
func (s ScopeType) String() string {
	return string(s)
}

// IsValid returns true if the scope type is a recognized value.
func (s ScopeType) IsValid() bool {
	switch s {
	case ScopeTypeGlobal, ScopeTypeProvider, ScopeTypeTag, ScopeTypeType:
		return true
	default:
		return false
	}
}

// ScopedBudgetStatus represents the evaluated state of a scoped budget.
type ScopedBudgetStatus struct {
	// ScopeType identifies the budget scope category.
	ScopeType ScopeType `json:"scope_type"`

	// ScopeKey is the identifier within the scope type.
	// For provider: "aws", "gcp", etc.
	// For tag: "team:platform", "env:prod", etc.
	// For type: "aws:ec2/instance", etc.
	// For global: empty string.
	ScopeKey string `json:"scope_key,omitempty"`

	// Budget is the configured budget for this scope.
	Budget config.ScopedBudget `json:"budget"`

	// CurrentSpend is the total cost allocated to this scope.
	CurrentSpend float64 `json:"current_spend"`

	// Percentage is CurrentSpend / Budget.Amount * 100.
	Percentage float64 `json:"percentage"`

	// ForecastedSpend is the projected end-of-period spend.
	ForecastedSpend float64 `json:"forecasted_spend,omitempty"`

	// ForecastPercentage is ForecastedSpend / Budget.Amount * 100.
	ForecastPercentage float64 `json:"forecast_percentage,omitempty"`

	// Health is the overall health status (OK, WARNING, CRITICAL, EXCEEDED).
	Health pbc.BudgetHealthStatus `json:"health"`

	// Alerts is the list of evaluated threshold statuses.
	Alerts []ThresholdStatus `json:"alerts,omitempty"`

	// MatchedResources is the count of resources allocated to this scope.
	MatchedResources int `json:"matched_resources,omitempty"`

	// Currency is the budget currency for display.
	Currency string `json:"currency,omitempty"`
}

// IsOverBudget returns true if current spend exceeds the budget amount.
func (s *ScopedBudgetStatus) IsOverBudget() bool {
	return s.Percentage >= HealthThresholdExceeded
}

// HasExceededAlerts returns true if any alert has EXCEEDED status.
func (s *ScopedBudgetStatus) HasExceededAlerts() bool {
	for _, alert := range s.Alerts {
		if alert.Status == ThresholdStatusExceeded {
			return true
		}
	}
	return false
}

// ScopeIdentifier returns a string identifier for this scope in format "type:key".
func (s *ScopedBudgetStatus) ScopeIdentifier() string {
	if s.ScopeType == ScopeTypeGlobal {
		return "global"
	}
	return fmt.Sprintf("%s:%s", s.ScopeType, s.ScopeKey)
}

// BudgetAllocation tracks cost allocation for a single resource.
type BudgetAllocation struct {
	// ResourceID is the unique identifier of the resource.
	ResourceID string `json:"resource_id,omitempty"`

	// ResourceType is the full type string (e.g., "aws:ec2/instance").
	ResourceType string `json:"resource_type"`

	// Provider is the extracted provider from the resource type.
	Provider string `json:"provider,omitempty"`

	// Cost is the resource's cost that was allocated.
	Cost float64 `json:"cost"`

	// AllocatedScopes lists all scopes that received this resource's cost.
	// Format: "global", "provider:aws", "tag:team:platform", "type:aws:ec2/instance"
	AllocatedScopes []string `json:"allocated_scopes,omitempty"`

	// MatchedTags lists all tags that matched tag budgets for this resource.
	// If multiple matched, only the highest priority receives cost.
	MatchedTags []string `json:"matched_tags,omitempty"`

	// SelectedTagBudget is the tag budget that received the cost allocation.
	// Empty if no tag budget matched or no tag budgets configured.
	SelectedTagBudget string `json:"selected_tag_budget,omitempty"`

	// Warnings contains any warnings generated during allocation.
	// e.g., "overlapping tag budgets without priority"
	Warnings []string `json:"warnings,omitempty"`
}

// ScopedBudgetResult contains all evaluated scoped budgets and summaries.
type ScopedBudgetResult struct {
	// Global is the global budget status (always present if configured).
	Global *ScopedBudgetStatus `json:"global,omitempty"`

	// ByProvider maps provider names to their budget statuses.
	ByProvider map[string]*ScopedBudgetStatus `json:"by_provider,omitempty"`

	// ByTag contains tag budget statuses in priority order.
	ByTag []*ScopedBudgetStatus `json:"by_tag,omitempty"`

	// ByType maps resource types to their budget statuses.
	ByType map[string]*ScopedBudgetStatus `json:"by_type,omitempty"`

	// OverallHealth is the worst health status across all scopes.
	OverallHealth pbc.BudgetHealthStatus `json:"overall_health"`

	// CriticalScopes lists scope identifiers with CRITICAL or EXCEEDED status.
	CriticalScopes []string `json:"critical_scopes,omitempty"`

	// Allocations contains per-resource allocation details (debug mode only).
	Allocations []BudgetAllocation `json:"allocations,omitempty"`

	// Warnings contains all warnings generated during evaluation.
	Warnings []string `json:"warnings,omitempty"`
}

// HasExceededBudgets returns true if any budget has EXCEEDED health status.
func (r *ScopedBudgetResult) HasExceededBudgets() bool {
	return r.OverallHealth == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
}

// HasCriticalBudgets returns true if any budget has CRITICAL or EXCEEDED health status.
func (r *ScopedBudgetResult) HasCriticalBudgets() bool {
	return r.OverallHealth == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL ||
		r.OverallHealth == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
}

// AllScopes returns all scoped budget statuses in a flat list.
//
// WARNING: The returned slice contains pointers to internal state.
// Callers MUST NOT modify the returned ScopedBudgetStatus objects.
// This design is intentional for performance in rendering scenarios.
// If modification is needed, callers should create their own copies.
func (r *ScopedBudgetResult) AllScopes() []*ScopedBudgetStatus {
	var scopes []*ScopedBudgetStatus

	if r.Global != nil {
		scopes = append(scopes, r.Global)
	}

	// Add provider scopes (sorted by key for deterministic output)
	providerKeys := make([]string, 0, len(r.ByProvider))
	for key := range r.ByProvider {
		providerKeys = append(providerKeys, key)
	}
	sort.Strings(providerKeys)
	for _, key := range providerKeys {
		scopes = append(scopes, r.ByProvider[key])
	}

	// Add tag scopes (already in priority order)
	scopes = append(scopes, r.ByTag...)

	// Add type scopes (sorted by key for deterministic output)
	typeKeys := make([]string, 0, len(r.ByType))
	for key := range r.ByType {
		typeKeys = append(typeKeys, key)
	}
	sort.Strings(typeKeys)
	for _, key := range typeKeys {
		scopes = append(scopes, r.ByType[key])
	}

	return scopes
}

// ExtractProvider extracts the provider name from a resource type string.
// Examples:
//   - "aws:ec2/instance" -> "aws"
//   - "gcp:compute/instance" -> "gcp"
//   - "azure:compute/virtualMachine" -> "azure"
//   - "unknown" -> "unknown"
//   - ":ec2/instance" -> "" (colon at start, no provider)
func ExtractProvider(resourceType string) string {
	idx := strings.Index(resourceType, ":")
	if idx > 0 {
		return strings.ToLower(resourceType[:idx])
	}
	if idx == 0 {
		// Colon at start means no valid provider
		return ""
	}
	return strings.ToLower(resourceType)
}

// CalculateHealthFromPercentage calculates health status from a raw utilization percentage.
// This is a convenience wrapper around CalculateBudgetHealthFromPercentage for scoped budgets.
func CalculateHealthFromPercentage(percentage float64) pbc.BudgetHealthStatus {
	return CalculateBudgetHealthFromPercentage(percentage)
}

// AggregateHealthStatuses returns the worst-case health status from a list of statuses.
// If statuses is empty, returns UNSPECIFIED.
func AggregateHealthStatuses(statuses []pbc.BudgetHealthStatus) pbc.BudgetHealthStatus {
	if len(statuses) == 0 {
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
	}

	worst := pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
	for _, status := range statuses {
		if healthSeverity(status) > healthSeverity(worst) {
			worst = status
		}
	}

	return worst
}

// parsedTagEntry holds a tag budget alongside its pre-parsed selector
// to avoid repeated parsing during resource matching.
type parsedTagEntry struct {
	budget   config.TagBudget
	selector *config.ParsedTagSelector
}

// ScopedBudgetEvaluator provides methods for evaluating scoped budgets.
type ScopedBudgetEvaluator struct {
	// config is the budgets configuration to evaluate against.
	config *config.BudgetsConfig

	// providerIndex maps lowercase provider names to their budgets.
	providerIndex map[string]*config.ScopedBudget

	// tagBudgets is sorted by priority (descending).
	tagBudgets []config.TagBudget

	// parsedTags holds pre-parsed tag selectors (sorted by priority descending).
	// Avoids re-parsing selectors on every MatchTagBudgets call.
	parsedTags []parsedTagEntry

	// typeIndex maps resource types to their budgets.
	typeIndex map[string]*config.ScopedBudget
}

// NewScopedBudgetEvaluator creates a new evaluator for the given configuration.
func NewScopedBudgetEvaluator(cfg *config.BudgetsConfig) *ScopedBudgetEvaluator {
	if cfg == nil {
		return &ScopedBudgetEvaluator{
			providerIndex: make(map[string]*config.ScopedBudget),
			tagBudgets:    nil,
			typeIndex:     make(map[string]*config.ScopedBudget),
		}
	}

	// Build provider index (case-insensitive)
	providerIndex := make(map[string]*config.ScopedBudget, len(cfg.Providers))
	for name, budget := range cfg.Providers {
		providerIndex[strings.ToLower(name)] = budget
	}

	// Sort tag budgets by priority (descending)
	tagBudgets := make([]config.TagBudget, len(cfg.Tags))
	copy(tagBudgets, cfg.Tags)
	sort.Slice(tagBudgets, func(i, j int) bool {
		return tagBudgets[i].Priority > tagBudgets[j].Priority
	})

	// Pre-parse tag selectors to avoid repeated parsing per resource
	parsedTags := make([]parsedTagEntry, 0, len(tagBudgets))
	logger := logging.FromContext(context.Background())
	for _, tb := range tagBudgets {
		parsed, err := config.ParseTagSelector(tb.Selector)
		if err != nil {
			// Log invalid selector at warn level so users see configuration issues
			logger.Warn().
				Str("selector", tb.Selector).
				Err(err).
				Msg("skipping invalid tag selector - this budget will not be applied")
			continue
		}
		parsedTags = append(parsedTags, parsedTagEntry{
			budget:   tb,
			selector: parsed,
		})
	}

	// Build type index
	typeIndex := make(map[string]*config.ScopedBudget, len(cfg.Types))
	maps.Copy(typeIndex, cfg.Types)

	return &ScopedBudgetEvaluator{
		config:        cfg,
		providerIndex: providerIndex,
		tagBudgets:    tagBudgets,
		parsedTags:    parsedTags,
		typeIndex:     typeIndex,
	}
}

// GetProviderBudget returns the budget for a provider, or nil if not configured.
func (e *ScopedBudgetEvaluator) GetProviderBudget(provider string) *config.ScopedBudget {
	return e.providerIndex[strings.ToLower(provider)]
}

// GetTypeBudget returns the budget for a resource type, or nil if not configured.
func (e *ScopedBudgetEvaluator) GetTypeBudget(resourceType string) *config.ScopedBudget {
	return e.typeIndex[resourceType]
}

// MatchTagBudgets returns all tag budgets that match the given tags.
// Results are returned in priority order (highest first).
// Uses pre-parsed selectors for efficiency (parsed once in NewScopedBudgetEvaluator).
func (e *ScopedBudgetEvaluator) MatchTagBudgets(_ context.Context, tags map[string]string) []config.TagBudget {
	if len(tags) == 0 || len(e.parsedTags) == 0 {
		return nil
	}

	var matches []config.TagBudget
	for _, entry := range e.parsedTags {
		if entry.selector.Matches(tags) {
			matches = append(matches, entry.budget)
		}
	}

	return matches
}

// SelectHighestPriorityTagBudget selects the tag budget with highest priority from matches.
// Returns nil if no matches provided.
// Emits a warning if multiple budgets have the same highest priority.
func (e *ScopedBudgetEvaluator) SelectHighestPriorityTagBudget(
	ctx context.Context,
	matches []config.TagBudget,
) (*config.TagBudget, []string) {
	if len(matches) == 0 {
		return nil, nil
	}

	// First match is highest priority (already sorted).
	selected := &matches[0]

	// Check for priority ties and handle if found.
	samePriority := collectSamePrioritySelectors(matches, selected.Priority)
	if len(samePriority) <= 1 {
		return selected, nil
	}

	// Handle priority tie: sort alphabetically and select first.
	selected, warning := handlePriorityTie(ctx, matches, samePriority)
	return selected, []string{warning}
}

// collectSamePrioritySelectors returns all selectors with the same priority as the first match.
func collectSamePrioritySelectors(matches []config.TagBudget, priority int) []string {
	samePriority := []string{matches[0].Selector}
	for i := 1; i < len(matches); i++ {
		if matches[i].Priority == priority {
			samePriority = append(samePriority, matches[i].Selector)
		} else {
			break // No more same-priority budgets (sorted by priority).
		}
	}
	return samePriority
}

// handlePriorityTie resolves a priority tie by selecting the first alphabetically.
func handlePriorityTie(
	ctx context.Context,
	matches []config.TagBudget,
	samePriority []string,
) (*config.TagBudget, string) {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "SelectHighestPriorityTagBudget").
		Logger()

	// Sort alphabetically and select first.
	sort.Strings(samePriority)
	var selected *config.TagBudget
	for i := range matches {
		if matches[i].Selector == samePriority[0] {
			selected = &matches[i]
			break
		}
	}

	warning := fmt.Sprintf("overlapping tag budgets with same priority %d: %v - selected %q",
		selected.Priority, samePriority, selected.Selector)

	logger.Warn().
		Int("priority", selected.Priority).
		Strs("selectors", samePriority).
		Str("selected", selected.Selector).
		Msg("overlapping tag budgets without unique priority")

	return selected, warning
}

// AllocateCostToProvider allocates a resource's cost to its provider budget.
// Returns a BudgetAllocation with provider scope if a matching budget exists.
func (e *ScopedBudgetEvaluator) AllocateCostToProvider(
	ctx context.Context,
	resourceType string,
	cost float64,
) *BudgetAllocation {
	provider := ExtractProvider(resourceType)

	allocation := &BudgetAllocation{
		ResourceType:    resourceType,
		Provider:        provider,
		Cost:            cost,
		AllocatedScopes: []string{},
	}

	// Check if provider budget exists
	if provider != "" {
		if budget := e.GetProviderBudget(provider); budget != nil {
			allocation.AllocatedScopes = append(allocation.AllocatedScopes, fmt.Sprintf("provider:%s", provider))

			logger := logging.FromContext(ctx).With().
				Str("component", "engine").
				Str("operation", "AllocateCostToProvider").
				Logger()

			logger.Debug().
				Str("resource_type", resourceType).
				Str("provider", provider).
				Float64("cost", cost).
				Msg("allocated cost to provider budget")
		}
	}

	return allocation
}

// CalculateProviderBudgetStatus calculates the budget status for a provider scope.
func CalculateProviderBudgetStatus(
	provider string,
	budget *config.ScopedBudget,
	currentSpend float64,
) *ScopedBudgetStatus {
	var percentage float64
	if budget.Amount > 0 {
		percentage = (currentSpend / budget.Amount) * percentageMultiplier
	}

	health := CalculateHealthFromPercentage(percentage)

	status := &ScopedBudgetStatus{
		ScopeType:    ScopeTypeProvider,
		ScopeKey:     provider,
		Budget:       *budget,
		CurrentSpend: currentSpend,
		Percentage:   percentage,
		Health:       health,
		Currency:     budget.Currency,
	}

	enrichScopedBudgetStatus(status, budget)
	return status
}

// AllocateCostToTag allocates a resource's cost to the highest-priority matching tag budget.
// Returns a BudgetAllocation with tag scope if a matching budget exists.
// If multiple tag budgets match with the same priority, warnings are emitted.
func (e *ScopedBudgetEvaluator) AllocateCostToTag(
	ctx context.Context,
	resourceType string,
	tags map[string]string,
	cost float64,
) *BudgetAllocation {
	allocation := &BudgetAllocation{
		ResourceType:    resourceType,
		Provider:        ExtractProvider(resourceType),
		Cost:            cost,
		AllocatedScopes: []string{},
		MatchedTags:     []string{},
		Warnings:        []string{},
	}

	// Find all matching tag budgets
	matches := e.MatchTagBudgets(ctx, tags)
	if len(matches) == 0 {
		return allocation
	}

	// Record all matched selectors
	for _, match := range matches {
		allocation.MatchedTags = append(allocation.MatchedTags, match.Selector)
	}

	// Select highest priority tag budget
	selected, warnings := e.SelectHighestPriorityTagBudget(ctx, matches)
	if selected == nil {
		return allocation
	}

	allocation.SelectedTagBudget = selected.Selector
	allocation.AllocatedScopes = append(allocation.AllocatedScopes, fmt.Sprintf("tag:%s", selected.Selector))
	allocation.Warnings = warnings

	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "AllocateCostToTag").
		Logger()

	logger.Debug().
		Str("resource_type", resourceType).
		Str("selected_tag", selected.Selector).
		Int("priority", selected.Priority).
		Strs("matched_tags", allocation.MatchedTags).
		Float64("cost", cost).
		Msg("allocated cost to tag budget")

	return allocation
}

// CalculateTagBudgetStatus calculates the budget status for a tag scope.
func CalculateTagBudgetStatus(
	tagBudget *config.TagBudget,
	currentSpend float64,
) *ScopedBudgetStatus {
	var percentage float64
	if tagBudget.Amount > 0 {
		percentage = (currentSpend / tagBudget.Amount) * percentageMultiplier
	}

	health := CalculateHealthFromPercentage(percentage)

	status := &ScopedBudgetStatus{
		ScopeType:    ScopeTypeTag,
		ScopeKey:     tagBudget.Selector,
		Budget:       tagBudget.ScopedBudget,
		CurrentSpend: currentSpend,
		Percentage:   percentage,
		Health:       health,
		Currency:     tagBudget.Currency,
	}

	enrichScopedBudgetStatus(status, &tagBudget.ScopedBudget)
	return status
}

// AllocateCostToType allocates a resource's cost to its resource type budget.
// Returns a BudgetAllocation with type scope if a matching budget exists.
func (e *ScopedBudgetEvaluator) AllocateCostToType(
	ctx context.Context,
	resourceType string,
	cost float64,
) *BudgetAllocation {
	allocation := &BudgetAllocation{
		ResourceType:    resourceType,
		Provider:        ExtractProvider(resourceType),
		Cost:            cost,
		AllocatedScopes: []string{},
	}

	// Check if type budget exists (exact match, case-sensitive)
	if budget := e.GetTypeBudget(resourceType); budget != nil {
		allocation.AllocatedScopes = append(allocation.AllocatedScopes, fmt.Sprintf("type:%s", resourceType))

		logger := logging.FromContext(ctx).With().
			Str("component", "engine").
			Str("operation", "AllocateCostToType").
			Logger()

		logger.Debug().
			Str("resource_type", resourceType).
			Float64("cost", cost).
			Msg("allocated cost to type budget")
	}

	return allocation
}

// CalculateTypeBudgetStatus calculates the budget status for a resource type scope.
func CalculateTypeBudgetStatus(
	resourceType string,
	budget *config.ScopedBudget,
	currentSpend float64,
) *ScopedBudgetStatus {
	var percentage float64
	if budget.Amount > 0 {
		percentage = (currentSpend / budget.Amount) * percentageMultiplier
	}

	health := CalculateHealthFromPercentage(percentage)

	status := &ScopedBudgetStatus{
		ScopeType:    ScopeTypeType,
		ScopeKey:     resourceType,
		Budget:       *budget,
		CurrentSpend: currentSpend,
		Percentage:   percentage,
		Health:       health,
		Currency:     budget.Currency,
	}

	enrichScopedBudgetStatus(status, budget)
	return status
}

// AllocateCosts allocates a resource's cost to all applicable budget scopes.
// This is the main entry point for multi-scope cost allocation.
// Returns a BudgetAllocation with all scopes that received the cost.
// Returns an empty allocation (no scopes) if the context is cancelled.
func (e *ScopedBudgetEvaluator) AllocateCosts(
	ctx context.Context,
	resourceType string,
	tags map[string]string,
	cost float64,
) *BudgetAllocation {
	// Check for context cancellation early to support graceful shutdown.
	// Return an empty allocation rather than nil to avoid nil pointer panics in callers.
	select {
	case <-ctx.Done():
		return &BudgetAllocation{
			ResourceType:    resourceType,
			Provider:        ExtractProvider(resourceType),
			Cost:            cost,
			AllocatedScopes: []string{},
			MatchedTags:     []string{},
			Warnings:        []string{},
		}
	default:
	}

	allocation := &BudgetAllocation{
		ResourceType:    resourceType,
		Provider:        ExtractProvider(resourceType),
		Cost:            cost,
		AllocatedScopes: []string{},
		MatchedTags:     []string{},
		Warnings:        []string{},
	}

	// Allocate to global budget if configured
	if e.config != nil && e.config.Global != nil && e.config.Global.IsEnabled() {
		allocation.AllocatedScopes = append(allocation.AllocatedScopes, "global")
	}

	// Allocate to provider budget if configured
	provider := allocation.Provider
	if provider != "" && e.GetProviderBudget(provider) != nil {
		allocation.AllocatedScopes = append(allocation.AllocatedScopes, fmt.Sprintf("provider:%s", provider))
	}

	// Allocate to tag budget if configured and matching
	if len(tags) > 0 && len(e.tagBudgets) > 0 {
		tagAllocation := e.AllocateCostToTag(ctx, resourceType, tags, cost)
		if tagAllocation.SelectedTagBudget != "" {
			allocation.AllocatedScopes = append(allocation.AllocatedScopes,
				fmt.Sprintf("tag:%s", tagAllocation.SelectedTagBudget))
			allocation.MatchedTags = tagAllocation.MatchedTags
			allocation.SelectedTagBudget = tagAllocation.SelectedTagBudget
			allocation.Warnings = append(allocation.Warnings, tagAllocation.Warnings...)
		}
	}

	// Allocate to type budget if configured
	if e.GetTypeBudget(resourceType) != nil {
		allocation.AllocatedScopes = append(allocation.AllocatedScopes,
			fmt.Sprintf("type:%s", resourceType))
	}

	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "AllocateCosts").
		Logger()

	logger.Debug().
		Str("resource_type", resourceType).
		Strs("allocated_scopes", allocation.AllocatedScopes).
		Float64("cost", cost).
		Msg("allocated cost to all applicable scopes")

	return allocation
}

// enrichScopedBudgetStatus populates ForecastedSpend, ForecastPercentage,
// and Alerts on a ScopedBudgetStatus using the budget's alert configuration
// and linear extrapolation forecasting.
func enrichScopedBudgetStatus(status *ScopedBudgetStatus, budget *config.ScopedBudget) {
	if status == nil || budget == nil || budget.Amount <= 0 {
		return
	}

	// Forecast: linear extrapolation from current spend
	now := time.Now()
	currentDay := now.Day()
	totalDays := daysInMonth(now)
	if currentDay == 0 {
		currentDay = 1
	}
	dailyRate := status.CurrentSpend / float64(currentDay)
	status.ForecastedSpend = dailyRate * float64(totalDays)
	status.ForecastPercentage = (status.ForecastedSpend / budget.Amount) * percentageMultiplier

	// Evaluate alerts from budget config
	alerts := budget.Alerts
	if len(alerts) == 0 {
		// Use default thresholds if none configured
		alerts = []config.AlertConfig{
			{Threshold: defaultThresholdInfo, Type: config.AlertTypeActual},
			{Threshold: defaultThresholdWarning, Type: config.AlertTypeActual},
			{Threshold: defaultThresholdCritical, Type: config.AlertTypeActual},
		}
	}

	status.Alerts = make([]ThresholdStatus, 0, len(alerts))
	for _, alert := range alerts {
		var pct float64
		if alert.Type == config.AlertTypeActual {
			pct = status.Percentage
		} else {
			pct = status.ForecastPercentage
		}
		status.Alerts = append(status.Alerts, ThresholdStatus{
			Threshold: alert.Threshold,
			Type:      alert.Type,
			Status:    evaluateThreshold(alert.Threshold, pct),
		})
	}
}

// CalculateOverallHealth calculates the worst-case health status across all scopes.
// Uses "worst wins" aggregation: EXCEEDED > CRITICAL > WARNING > OK > UNSPECIFIED.
func CalculateOverallHealth(result *ScopedBudgetResult) pbc.BudgetHealthStatus {
	if result == nil {
		return pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
	}

	var statuses []pbc.BudgetHealthStatus

	// Collect health from global
	if result.Global != nil {
		statuses = append(statuses, result.Global.Health)
	}

	// Collect health from providers
	for _, status := range result.ByProvider {
		statuses = append(statuses, status.Health)
	}

	// Collect health from tags
	for _, status := range result.ByTag {
		statuses = append(statuses, status.Health)
	}

	// Collect health from types
	for _, status := range result.ByType {
		statuses = append(statuses, status.Health)
	}

	return AggregateHealthStatuses(statuses)
}

// IdentifyCriticalScopes returns the scope identifiers that have CRITICAL or EXCEEDED status.
// Results are sorted for deterministic output.
func IdentifyCriticalScopes(result *ScopedBudgetResult) []string {
	if result == nil {
		return nil
	}

	var criticalScopes []string

	// Check global
	if result.Global != nil && isCriticalOrExceeded(result.Global.Health) {
		criticalScopes = append(criticalScopes, result.Global.ScopeIdentifier())
	}

	// Check providers (sorted for deterministic output)
	providerKeys := make([]string, 0, len(result.ByProvider))
	for key := range result.ByProvider {
		providerKeys = append(providerKeys, key)
	}
	sort.Strings(providerKeys)
	for _, key := range providerKeys {
		if isCriticalOrExceeded(result.ByProvider[key].Health) {
			criticalScopes = append(criticalScopes, result.ByProvider[key].ScopeIdentifier())
		}
	}

	// Check tags
	for _, status := range result.ByTag {
		if isCriticalOrExceeded(status.Health) {
			criticalScopes = append(criticalScopes, status.ScopeIdentifier())
		}
	}

	// Check types (sorted for deterministic output)
	typeKeys := make([]string, 0, len(result.ByType))
	for key := range result.ByType {
		typeKeys = append(typeKeys, key)
	}
	sort.Strings(typeKeys)
	for _, key := range typeKeys {
		if isCriticalOrExceeded(result.ByType[key].Health) {
			criticalScopes = append(criticalScopes, result.ByType[key].ScopeIdentifier())
		}
	}

	return criticalScopes
}

// isCriticalOrExceeded returns true if health status is CRITICAL or EXCEEDED.
func isCriticalOrExceeded(health pbc.BudgetHealthStatus) bool {
	return health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL ||
		health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
}
