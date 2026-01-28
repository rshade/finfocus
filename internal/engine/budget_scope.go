package engine

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

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
	ScopeType ScopeType

	// ScopeKey is the identifier within the scope type.
	// For provider: "aws", "gcp", etc.
	// For tag: "team:platform", "env:prod", etc.
	// For type: "aws:ec2/instance", etc.
	// For global: empty string.
	ScopeKey string

	// Budget is the configured budget for this scope.
	Budget config.ScopedBudget

	// CurrentSpend is the total cost allocated to this scope.
	CurrentSpend float64

	// Percentage is CurrentSpend / Budget.Amount * 100.
	Percentage float64

	// ForecastedSpend is the projected end-of-period spend.
	ForecastedSpend float64

	// ForecastPercentage is ForecastedSpend / Budget.Amount * 100.
	ForecastPercentage float64

	// Health is the overall health status (OK, WARNING, CRITICAL, EXCEEDED).
	Health pbc.BudgetHealthStatus

	// Alerts is the list of evaluated threshold statuses.
	Alerts []ThresholdStatus

	// MatchedResources is the count of resources allocated to this scope.
	MatchedResources int

	// Currency is the budget currency for display.
	Currency string
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
	ResourceID string

	// ResourceType is the full type string (e.g., "aws:ec2/instance").
	ResourceType string

	// Provider is the extracted provider from the resource type.
	Provider string

	// Cost is the resource's cost that was allocated.
	Cost float64

	// AllocatedScopes lists all scopes that received this resource's cost.
	// Format: "global", "provider:aws", "tag:team:platform", "type:aws:ec2/instance"
	AllocatedScopes []string

	// MatchedTags lists all tags that matched tag budgets for this resource.
	// If multiple matched, only the highest priority receives cost.
	MatchedTags []string

	// SelectedTagBudget is the tag budget that received the cost allocation.
	// Empty if no tag budget matched or no tag budgets configured.
	SelectedTagBudget string

	// Warnings contains any warnings generated during allocation.
	// e.g., "overlapping tag budgets without priority"
	Warnings []string
}

// ScopedBudgetResult contains all evaluated scoped budgets and summaries.
type ScopedBudgetResult struct {
	// Global is the global budget status (always present if configured).
	Global *ScopedBudgetStatus

	// ByProvider maps provider names to their budget statuses.
	ByProvider map[string]*ScopedBudgetStatus

	// ByTag contains tag budget statuses in priority order.
	ByTag []*ScopedBudgetStatus

	// ByType maps resource types to their budget statuses.
	ByType map[string]*ScopedBudgetStatus

	// OverallHealth is the worst health status across all scopes.
	OverallHealth pbc.BudgetHealthStatus

	// CriticalScopes lists scope identifiers with CRITICAL or EXCEEDED status.
	CriticalScopes []string

	// Allocations contains per-resource allocation details (debug mode only).
	Allocations []BudgetAllocation

	// Warnings contains all warnings generated during evaluation.
	Warnings []string
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

// ScopedBudgetEvaluator provides methods for evaluating scoped budgets.
type ScopedBudgetEvaluator struct {
	// config is the budgets configuration to evaluate against.
	config *config.BudgetsConfig

	// providerIndex maps lowercase provider names to their budgets.
	providerIndex map[string]*config.ScopedBudget

	// tagBudgets is sorted by priority (descending).
	tagBudgets []config.TagBudget

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

	// Build type index
	typeIndex := make(map[string]*config.ScopedBudget, len(cfg.Types))
	maps.Copy(typeIndex, cfg.Types)

	return &ScopedBudgetEvaluator{
		config:        cfg,
		providerIndex: providerIndex,
		tagBudgets:    tagBudgets,
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
func (e *ScopedBudgetEvaluator) MatchTagBudgets(ctx context.Context, tags map[string]string) []config.TagBudget {
	if len(tags) == 0 || len(e.tagBudgets) == 0 {
		return nil
	}

	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "MatchTagBudgets").
		Logger()

	var matches []config.TagBudget
	for _, tag := range e.tagBudgets {
		parsed, err := config.ParseTagSelector(tag.Selector)
		if err != nil {
			logger.Warn().
				Str("selector", tag.Selector).
				Err(err).
				Msg("invalid tag selector in configuration")
			continue
		}

		if parsed.Matches(tags) {
			matches = append(matches, tag)
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

	return &ScopedBudgetStatus{
		ScopeType:    ScopeTypeProvider,
		ScopeKey:     provider,
		Budget:       *budget,
		CurrentSpend: currentSpend,
		Percentage:   percentage,
		Health:       health,
		Currency:     budget.Currency,
	}
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

	return &ScopedBudgetStatus{
		ScopeType:    ScopeTypeTag,
		ScopeKey:     tagBudget.Selector,
		Budget:       tagBudget.ScopedBudget,
		CurrentSpend: currentSpend,
		Percentage:   percentage,
		Health:       health,
		Currency:     tagBudget.Currency,
	}
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

	return &ScopedBudgetStatus{
		ScopeType:    ScopeTypeType,
		ScopeKey:     resourceType,
		Budget:       *budget,
		CurrentSpend: currentSpend,
		Percentage:   percentage,
		Health:       health,
		Currency:     budget.Currency,
	}
}

// AllocateCosts allocates a resource's cost to all applicable budget scopes.
// This is the main entry point for multi-scope cost allocation.
// Returns a BudgetAllocation with all scopes that received the cost.
// Returns nil if the context is cancelled.
func (e *ScopedBudgetEvaluator) AllocateCosts(
	ctx context.Context,
	resourceType string,
	tags map[string]string,
	cost float64,
) *BudgetAllocation {
	// Check for context cancellation early to support graceful shutdown
	select {
	case <-ctx.Done():
		return nil
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
func IdentifyCriticalScopes(result *ScopedBudgetResult) []string {
	if result == nil {
		return nil
	}

	var criticalScopes []string

	// Check global
	if result.Global != nil && isCriticalOrExceeded(result.Global.Health) {
		criticalScopes = append(criticalScopes, result.Global.ScopeIdentifier())
	}

	// Check providers
	for _, status := range result.ByProvider {
		if isCriticalOrExceeded(status.Health) {
			criticalScopes = append(criticalScopes, status.ScopeIdentifier())
		}
	}

	// Check tags
	for _, status := range result.ByTag {
		if isCriticalOrExceeded(status.Health) {
			criticalScopes = append(criticalScopes, status.ScopeIdentifier())
		}
	}

	// Check types
	for _, status := range result.ByType {
		if isCriticalOrExceeded(status.Health) {
			criticalScopes = append(criticalScopes, status.ScopeIdentifier())
		}
	}

	return criticalScopes
}

// isCriticalOrExceeded returns true if health status is CRITICAL or EXCEEDED.
func isCriticalOrExceeded(health pbc.BudgetHealthStatus) bool {
	return health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL ||
		health == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
}
