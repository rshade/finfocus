package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/engine/batch"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const (
	hoursPerDay                 = 24
	hoursPerMonth               = 730
	daysPerWeek                 = 7     // Days in a week
	daysPerMonth                = 30    // Approximate days per month
	avgDaysPerMonth             = 30.44 // Accurate average days per month for conversions
	minProviderServiceParts     = 2     // For "provider:service"
	minProviderServiceTypeParts = 3     // For "provider:service:type"
	filterKeyValueParts         = 2     // For "key=value"
	defaultDatabaseMonthlyCost  = 50.0  // Default monthly cost for database resources
	defaultStorageMonthlyCost   = 5.0   // Default monthly cost for storage resources
	defaultComputeMonthlyCost   = 20.0  // Default monthly cost for compute resources
	defaultServiceName          = "default"
	defaultCurrency             = "USD" // Default currency for cost calculations
	batchProcessingThreshold    = 100   // Threshold for enabling batch processing
)

const (
	// defaultConcurrencyMultiplier is the default multiplier for NumCPU to determine worker count.
	defaultConcurrencyMultiplier = 2
	// envConcurrencyMultiplier is the environment variable to override the default multiplier.
	envConcurrencyMultiplier = "FINFOCUS_CONCURRENCY_MULTIPLIER"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const (
	// ContextKeyUtilization is the context key for passing utilization rate.
	ContextKeyUtilization ContextKey = "utilization"

	// Timeout constants for engine operations.
	defaultQueryTimeout = 60 * time.Second // Overall query timeout.
	perResourceTimeout  = 5 * time.Second  // Per-resource calculation timeout.
)

var (
	// ErrNoCostData is returned when no cost data is available for a resource.
	ErrNoCostData = errors.New("no cost data available")
	// ErrMixedCurrencies is returned when cross-provider aggregation encounters multiple currencies.
	ErrMixedCurrencies = errors.New("mixed currencies not supported in cross-provider aggregation")
	// ErrInvalidGroupBy is returned when a non-time-based grouping is used for cross-provider aggregation.
	ErrInvalidGroupBy = errors.New("invalid groupBy type for cross-provider aggregation")
	// ErrEmptyResults is returned when aggregation is attempted on empty results.
	ErrEmptyResults = errors.New("empty results provided for aggregation")
	// ErrInvalidDateRange is returned when the end date is before the start date.
	ErrInvalidDateRange = errors.New("invalid date range: end date must be after start date")
)

// SpecLoader is an interface for loading pricing specifications from local YAML files.
type SpecLoader interface {
	LoadSpec(provider, service, sku string) (interface{}, error)
}

// PluginMatch represents a matched plugin from the router.
// This mirrors router.PluginMatch to avoid circular imports.
type PluginMatch struct {
	Client      *pluginhost.Client
	Priority    int
	Fallback    bool
	MatchReason string
	Source      string
}

// Router selects appropriate plugins for resources.
// This interface is implemented by the router package.
type Router interface {
	// SelectPlugins returns plugins that match a resource for a given feature.
	// Results are ordered by priority (highest first).
	SelectPlugins(ctx context.Context, resource ResourceDescriptor, feature string) []PluginMatch

	// ShouldFallback returns true if fallback is enabled for a plugin.
	ShouldFallback(pluginName string) bool
}

// Engine orchestrates cost calculations between plugins and local pricing specifications.
type Engine struct {
	clients []*pluginhost.Client
	loader  SpecLoader
	cache   *cache.FileStore
	router  Router // Optional router for plugin selection; if nil, queries all plugins
}

// New creates a new Engine with the given plugin clients and spec loader.
func New(clients []*pluginhost.Client, loader SpecLoader) *Engine {
	return &Engine{
		clients: clients,
		loader:  loader,
		cache:   nil, // Cache is optional, set via WithCache
	}
}

// WithCache sets the cache store for the engine.
// This is optional - if not set, no caching will be performed.
func (e *Engine) WithCache(cacheStore *cache.FileStore) *Engine {
	e.cache = cacheStore
	return e
}

// WithRouter sets the router for intelligent plugin selection.
// This is optional - if not set, all plugins are queried for each resource.
func (e *Engine) WithRouter(router Router) *Engine {
	e.router = router
	return e
}

func (e *Engine) getConcurrencyMultiplier() int {
	if val := os.Getenv(envConcurrencyMultiplier); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			return i
		}
	}
	return defaultConcurrencyMultiplier
}

// selectPluginMatchesForResource returns the full PluginMatch list for a resource.
// This includes fallback configuration and priority information for each plugin.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - resource: The resource to find plugins for
//   - feature: The feature being requested (e.g., "ProjectedCosts", "ActualCosts")
//
// Returns:
//   - []PluginMatch: Matched plugins with metadata, ordered by priority
func (e *Engine) selectPluginMatchesForResource(
	ctx context.Context,
	resource ResourceDescriptor,
	feature string,
) []PluginMatch {
	log := logging.FromContext(ctx)

	// If no router configured, return all clients as matches with fallback enabled
	if e.router == nil {
		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "select_plugins").
			Str("resource_type", resource.Type).
			Int("client_count", len(e.clients)).
			Msg("no router configured, using all clients")

		matches := make([]PluginMatch, len(e.clients))
		for i, client := range e.clients {
			matches[i] = PluginMatch{
				Client:      client,
				Priority:    0,
				Fallback:    true, // Default to fallback enabled
				MatchReason: "automatic",
				Source:      "automatic",
			}
		}
		return matches
	}

	// Use router for intelligent plugin selection
	matches := e.router.SelectPlugins(ctx, resource, feature)
	if len(matches) == 0 {
		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "select_plugins").
			Str("resource_type", resource.Type).
			Msg("router returned no matches, falling back to all clients")

		// Create matches for all clients with fallback enabled
		fallbackMatches := make([]PluginMatch, len(e.clients))
		for i, client := range e.clients {
			fallbackMatches[i] = PluginMatch{
				Client:      client,
				Priority:    0,
				Fallback:    true,
				MatchReason: "automatic",
				Source:      "automatic",
			}
		}
		return fallbackMatches
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "select_plugins").
		Str("resource_type", resource.Type).
		Int("matched_count", len(matches)).
		Msg("router selected plugins")

	return matches
}

func (e *Engine) getWorkerCount(jobCount int) int {
	if jobCount == 0 {
		return 0
	}
	numWorkers := runtime.NumCPU() * e.getConcurrencyMultiplier()
	if jobCount < numWorkers {
		numWorkers = jobCount
	}
	return numWorkers
}

// GetProjectedCost calculates projected costs for the given resources using plugins or specs.
//
//nolint:gocognit,funlen // Comprehensive cost calculation requires multiple nested conditions and fallbacks.
func (e *Engine) GetProjectedCost(
	ctx context.Context,
	resources []ResourceDescriptor,
) ([]CostResult, error) {
	log := logging.FromContext(ctx)
	start := time.Now()

	// Apply overall query timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultQueryTimeout)
		defer cancel()
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "get_projected_cost").
		Int("resource_count", len(resources)).
		Int("plugin_count", len(e.clients)).
		Msg("starting projected cost calculation")

	// Validate all resources before processing
	for i, resource := range resources {
		if err := resource.Validate(); err != nil {
			return nil, fmt.Errorf("invalid resource at index %d: %w", i, err)
		}
	}

	type job struct {
		index    int
		resource ResourceDescriptor
	}

	type workerResult struct {
		index   int
		results []CostResult
	}

	numWorkers := e.getWorkerCount(len(resources))
	if numWorkers == 0 {
		return []CostResult{}, nil
	}

	jobs := make(chan job, len(resources))
	resultsChan := make(chan workerResult, len(resources))
	var wg sync.WaitGroup

	// Worker function
	worker := func() {
		defer wg.Done()
		for j := range jobs {
			// Check context cancellation
			if ctx.Err() != nil {
				return
			}

			resource := j.resource
			var resourceResults []CostResult

			// Select plugin matches using router (if configured) or all clients
			selectedMatches := e.selectPluginMatchesForResource(ctx, resource, "ProjectedCosts")

			for i, match := range selectedMatches {
				client := match.Client
				log.Debug().
					Ctx(ctx).
					Str("component", "engine").
					Str("resource_type", resource.Type).
					Str("resource_id", resource.ID).
					Str("plugin", client.Name).
					Int("priority", match.Priority).
					Msg("querying plugin for projected cost")

				// Apply per-resource timeout for plugin calls
				resourceCtx, resourceCancel := context.WithTimeout(ctx, perResourceTimeout)
				result, err := e.getProjectedCostFromPlugin(resourceCtx, client, resource)
				resourceCancel()
				if err != nil {
					// Check if fallback is enabled for this plugin
					if !match.Fallback {
						log.Info().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Str("plugin", client.Name).
							Err(err).
							Msg("plugin failed and fallback disabled, stopping fallback chain")
						break // Stop trying more plugins
					}
					// Log fallback event at INFO level per FR-020
					if i < len(selectedMatches)-1 {
						nextPlugin := selectedMatches[i+1].Client.Name
						log.Info().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Str("failed_plugin", client.Name).
							Str("next_plugin", nextPlugin).
							Err(err).
							Msg("plugin failed, falling back to next priority plugin")
					} else {
						log.Debug().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Str("plugin", client.Name).
							Err(err).
							Msg("last plugin failed, no more plugins to try")
					}
					continue
				}
				if result != nil {
					log.Debug().
						Ctx(ctx).
						Str("component", "engine").
						Str("resource_type", resource.Type).
						Str("plugin", client.Name).
						Float64("monthly_cost", result.Monthly).
						Msg("plugin returned cost data")
					resourceResults = append(resourceResults, *result)
				}
			}

			if len(resourceResults) == 0 {
				// Single spec fallback per resource
				if e.loader != nil {
					log.Debug().
						Ctx(ctx).
						Str("component", "engine").
						Str("resource_type", resource.Type).
						Str("resource_id", resource.ID).
						Msg("no plugin data, trying spec fallback")

					if specRes := e.getProjectedCostFromSpec(ctx, resource); specRes != nil {
						log.Debug().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Float64("monthly_cost", specRes.Monthly).
							Msg("spec fallback provided cost data")
						resourceResults = append(resourceResults, *specRes)
					}
				}

				if len(resourceResults) == 0 {
					// Final fallback: no cost data available
					log.Warn().
						Ctx(ctx).
						Str("component", "engine").
						Str("resource_type", resource.Type).
						Str("resource_id", resource.ID).
						Msg("no pricing data available from plugins or specs")

					resourceResults = append(resourceResults, CostResult{
						ResourceType: resource.Type,
						ResourceID:   resource.ID,
						Adapter:      "none",
						Currency:     defaultCurrency,
						Monthly:      0,
						Hourly:       0,
						Notes:        "No pricing information available",
					})
				}
			}

			resultsChan <- workerResult{index: j.index, results: resourceResults}
		}
	}

	// Start workers
	for range numWorkers {
		wg.Add(1)
		go worker()
	}

	// Submit jobs
	for i, resource := range resources {
		jobs <- job{index: i, resource: resource}
	}
	close(jobs)

	// Wait for workers in a goroutine to close resultsChan
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var collectedResults []workerResult
	for res := range resultsChan {
		collectedResults = append(collectedResults, res)
	}

	if ctx.Err() != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "engine").
			Int("processed", len(collectedResults)).
			Int("total", len(resources)).
			Msg("query timeout reached, returning partial results")
	}

	// Sort by index to preserve order
	sort.Slice(collectedResults, func(i, j int) bool {
		return collectedResults[i].index < collectedResults[j].index
	})

	var results []CostResult
	for _, cr := range collectedResults {
		results = append(results, cr.results...)
	}

	if ctx.Err() != nil {
		return results, fmt.Errorf("projected cost calculation cancelled: %w", ctx.Err())
	}

	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "get_projected_cost").
		Int("result_count", len(results)).
		Dur("duration_ms", time.Since(start)).
		Msg("projected cost calculation complete")

	return results, nil
}

// GetProjectedCostWithErrors calculates projected costs with comprehensive error tracking.
//
//nolint:funlen,gocognit // Parallel implementation requires worker setup
func (e *Engine) GetProjectedCostWithErrors(
	ctx context.Context,
	resources []ResourceDescriptor,
) (*CostResultWithErrors, error) {
	type job struct {
		index    int
		resource ResourceDescriptor
	}

	type workerResult struct {
		index   int
		results []CostResult
		errors  []ErrorDetail
	}

	numWorkers := e.getWorkerCount(len(resources))
	if numWorkers == 0 {
		return &CostResultWithErrors{}, nil
	}

	jobs := make(chan job, len(resources))
	resultsChan := make(chan workerResult, len(resources))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			if ctx.Err() != nil {
				return
			}

			resource := j.resource
			var resourceResults []CostResult
			var resourceErrors []ErrorDetail
			log := logging.FromContext(ctx)

			// Select plugin matches using router (if configured) or all clients
			selectedMatches := e.selectPluginMatchesForResource(ctx, resource, "ProjectedCosts")

			// Try each selected plugin with fallback chain logic
			fallbackChainBroken := false
			for i, match := range selectedMatches {
				if fallbackChainBroken {
					break
				}

				client := match.Client
				pluginResult, err := e.getProjectedCostFromPlugin(ctx, client, resource)
				if err != nil {
					// Check if fallback is enabled for this plugin
					if !match.Fallback {
						log.Info().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Str("plugin", client.Name).
							Err(err).
							Msg("plugin failed and fallback disabled, stopping fallback chain")
						fallbackChainBroken = true
					} else if i < len(selectedMatches)-1 {
						// Log fallback event at INFO level per FR-020
						nextPlugin := selectedMatches[i+1].Client.Name
						log.Info().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Str("failed_plugin", client.Name).
							Str("next_plugin", nextPlugin).
							Err(err).
							Msg("plugin failed, falling back to next priority plugin")
					}

					// Track error
					resourceErrors = append(resourceErrors, ErrorDetail{
						ResourceType: resource.Type,
						ResourceID:   resource.ID,
						PluginName:   client.Name,
						Error:        fmt.Errorf("plugin call failed: %w", err),
						Timestamp:    time.Now(),
					})
					continue
				}
				if pluginResult != nil {
					engineResult := *pluginResult
					resourceResults = append(resourceResults, engineResult)
				}
			}

			// If no results from plugins, try spec fallback
			if len(resourceResults) == 0 {
				fallbackUsed := false
				if e.loader != nil {
					if specRes := e.getProjectedCostFromSpec(ctx, resource); specRes != nil {
						resourceResults = append(resourceResults, *specRes)
						fallbackUsed = true
					}
				}

				if !fallbackUsed {
					// Final fallback: no cost data available
					resourceResults = append(resourceResults, CostResult{
						ResourceType: resource.Type,
						ResourceID:   resource.ID,
						Adapter:      "none",
						Currency:     defaultCurrency,
						Monthly:      0,
						Hourly:       0,
						Notes:        "No pricing information available",
					})
				}
			}

			resultsChan <- workerResult{
				index:   j.index,
				results: resourceResults,
				errors:  resourceErrors,
			}
		}
	}

	for range numWorkers {
		wg.Add(1)
		go worker()
	}

	for i, resource := range resources {
		jobs <- job{index: i, resource: resource}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var collectedResults []workerResult
	for res := range resultsChan {
		collectedResults = append(collectedResults, res)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	sort.Slice(collectedResults, func(i, j int) bool {
		return collectedResults[i].index < collectedResults[j].index
	})

	finalResult := &CostResultWithErrors{
		Results: []CostResult{},
		Errors:  []ErrorDetail{},
	}

	for _, cr := range collectedResults {
		finalResult.Results = append(finalResult.Results, cr.results...)
		finalResult.Errors = append(finalResult.Errors, cr.errors...)
	}

	return finalResult, nil
}

// GetActualCost retrieves historical actual costs from plugins for the specified time range.
func (e *Engine) GetActualCost(
	ctx context.Context,
	resources []ResourceDescriptor,
	from, to time.Time,
) ([]CostResult, error) {
	return e.GetActualCostWithOptions(ctx, ActualCostRequest{
		Resources: resources,
		From:      from,
		To:        to,
	})
}

// GetActualCostWithOptions retrieves actual costs with advanced filtering, grouping, and time range options.
//
//nolint:funlen,gocognit // Comprehensive logging and cost calculation pipeline requires additional complexity.
func (e *Engine) GetActualCostWithOptions(
	ctx context.Context,
	request ActualCostRequest,
) ([]CostResult, error) {
	log := logging.FromContext(ctx)
	start := time.Now()

	// Apply overall query timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultQueryTimeout)
		defer cancel()
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "get_actual_cost").
		Int("resource_count", len(request.Resources)).
		Str("from", request.From.Format("2006-01-02")).
		Str("to", request.To.Format("2006-01-02")).
		Str("group_by", request.GroupBy).
		Msg("starting actual cost calculation")

	// Validate all resources before processing
	for i, resource := range request.Resources {
		if err := resource.Validate(); err != nil {
			return nil, fmt.Errorf("invalid resource at index %d: %w", i, err)
		}
	}

	type job struct {
		index    int
		resource ResourceDescriptor
	}

	type workerResult struct {
		index        int
		result       *CostResult
		partialError error
	}

	numWorkers := e.getWorkerCount(len(request.Resources))
	if numWorkers == 0 {
		return []CostResult{}, nil
	}

	jobs := make(chan job, len(request.Resources))
	resultsChan := make(chan workerResult, len(request.Resources))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			if ctx.Err() != nil {
				return
			}

			resource := j.resource

			// Filter by tags if specified
			if len(request.Tags) > 0 && !MatchesTags(resource, request.Tags) {
				log.Debug().
					Ctx(ctx).
					Str("component", "engine").
					Str("resource_type", resource.Type).
					Str("resource_id", resource.ID).
					Msg("resource filtered out by tags")
				resultsChan <- workerResult{index: j.index, result: nil}
				continue
			}

			var resourceResult *CostResult
			var partialErr error

			// Select plugin matches using router (if configured) or all clients
			selectedMatches := e.selectPluginMatchesForResource(ctx, resource, "ActualCosts")

			fallbackChainBroken := false
			for i, match := range selectedMatches {
				if fallbackChainBroken {
					break
				}

				client := match.Client

				// Apply explicit adapter filter if specified
				if request.Adapter != "" && client.Name != request.Adapter {
					continue
				}

				log.Debug().
					Ctx(ctx).
					Str("component", "engine").
					Str("resource_type", resource.Type).
					Str("resource_id", resource.ID).
					Str("plugin", client.Name).
					Int("priority", match.Priority).
					Msg("querying plugin for actual cost")

				// Apply per-resource timeout for plugin calls
				resourceCtx, resourceCancel := context.WithTimeout(ctx, perResourceTimeout)
				result, err := e.getActualCostFromPlugin(
					resourceCtx,
					client,
					resource,
					request.From,
					request.To,
				)
				resourceCancel()
				if err != nil {
					// Check if fallback is enabled for this plugin
					if !match.Fallback {
						log.Info().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Str("plugin", client.Name).
							Err(err).
							Msg("plugin failed and fallback disabled, stopping fallback chain")
						fallbackChainBroken = true
					} else if i < len(selectedMatches)-1 {
						// Log fallback event at INFO level per FR-020
						nextPlugin := selectedMatches[i+1].Client.Name
						log.Info().
							Ctx(ctx).
							Str("component", "engine").
							Str("resource_type", resource.Type).
							Str("failed_plugin", client.Name).
							Str("next_plugin", nextPlugin).
							Err(err).
							Msg("plugin failed, falling back to next priority plugin")
					}

					newErr := fmt.Errorf("plugin %s: %w", client.Name, err)
					if partialErr == nil {
						partialErr = newErr
					} else {
						partialErr = errors.Join(partialErr, newErr)
					}
					continue
				}
				if result != nil {
					log.Debug().
						Ctx(ctx).
						Str("component", "engine").
						Str("resource_type", resource.Type).
						Str("plugin", client.Name).
						Float64("total_cost", result.TotalCost).
						Msg("plugin returned actual cost data")
					resourceResult = result
					break
				}
			}

			// If no plugin provided data, create a placeholder result
			if resourceResult == nil {
				log.Warn().
					Ctx(ctx).
					Str("component", "engine").
					Str("resource_type", resource.Type).
					Str("resource_id", resource.ID).
					Msg("no actual cost data available from plugins")

				resourceResult = &CostResult{
					ResourceType: resource.Type,
					ResourceID:   resource.ID,
					Adapter:      "none",
					Currency:     defaultCurrency,
					TotalCost:    0,
					Notes:        "No actual cost data available",
					StartDate:    request.From,
					EndDate:      request.To,
					CostPeriod:   FormatPeriod(request.From, request.To),
				}
			}

			resultsChan <- workerResult{index: j.index, result: resourceResult, partialError: partialErr}
		}
	}

	for range numWorkers {
		wg.Add(1)
		go worker()
	}

	for i, resource := range request.Resources {
		jobs <- job{index: i, resource: resource}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var collectedResults []workerResult
	var partialErrors []error

	for res := range resultsChan {
		collectedResults = append(collectedResults, res)
		if res.partialError != nil {
			partialErrors = append(partialErrors, res.partialError)
		}
	}

	if ctx.Err() != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "engine").
			Int("processed", len(collectedResults)).
			Int("total", len(request.Resources)).
			Msg("query timeout reached, returning partial results")
		return nil, fmt.Errorf("actual cost calculation cancelled: %w", ctx.Err())
	}

	sort.Slice(collectedResults, func(i, j int) bool {
		return collectedResults[i].index < collectedResults[j].index
	})

	var results []CostResult
	for _, cr := range collectedResults {
		if cr.result != nil {
			results = append(results, *cr.result)
		}
	}

	// Group results if requested
	if request.GroupBy != "" {
		log.Debug().
			Ctx(ctx).
			Str("component", "engine").
			Str("group_by", request.GroupBy).
			Int("pre_group_count", len(results)).
			Msg("grouping results")
		results = e.GroupResults(results, GroupBy(request.GroupBy))
	}

	// Log partial errors
	if len(partialErrors) > 0 {
		log.Warn().
			Ctx(ctx).
			Str("component", "engine").
			Int("error_count", len(partialErrors)).
			Msg("some plugins returned errors during actual cost calculation")
	}

	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "get_actual_cost").
		Int("result_count", len(results)).
		Dur("duration_ms", time.Since(start)).
		Msg("actual cost calculation complete")

	return results, nil
}

// GetActualCostWithOptionsAndErrors retrieves actual costs with comprehensive error tracking.
// It returns results for all resources (with placeholders for failures) and aggregated error details.
func (e *Engine) GetActualCostWithOptionsAndErrors(
	ctx context.Context,
	request ActualCostRequest,
) (*CostResultWithErrors, error) {
	type job struct {
		index    int
		resource ResourceDescriptor
	}

	type workerResult struct {
		index  int
		result *CostResult
		errors []ErrorDetail
	}

	numWorkers := e.getWorkerCount(len(request.Resources))
	if numWorkers == 0 {
		return &CostResultWithErrors{}, nil
	}

	jobs := make(chan job, len(request.Resources))
	resultsChan := make(chan workerResult, len(request.Resources))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			if ctx.Err() != nil {
				return
			}

			resource := j.resource

			// Filter by tags if specified
			if len(request.Tags) > 0 && !MatchesTags(resource, request.Tags) {
				resultsChan <- workerResult{index: j.index, result: nil, errors: nil}
				continue
			}

			resourceResult, errors := e.getActualCostForResource(ctx, resource, request)
			resultsChan <- workerResult{index: j.index, result: &resourceResult, errors: errors}
		}
	}

	for range numWorkers {
		wg.Add(1)
		go worker()
	}

	for i, resource := range request.Resources {
		jobs <- job{index: i, resource: resource}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var collectedResults []workerResult
	for res := range resultsChan {
		collectedResults = append(collectedResults, res)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	sort.Slice(collectedResults, func(i, j int) bool {
		return collectedResults[i].index < collectedResults[j].index
	})

	result := &CostResultWithErrors{
		Results: []CostResult{},
		Errors:  []ErrorDetail{},
	}

	for _, cr := range collectedResults {
		if cr.result != nil {
			result.Results = append(result.Results, *cr.result)
		}
		result.Errors = append(result.Errors, cr.errors...)
	}

	// Group results if requested
	if request.GroupBy != "" {
		result.Results = e.GroupResults(result.Results, GroupBy(request.GroupBy))
	}

	return result, nil
}

// getActualCostForResource processes a single resource for actual cost with error tracking.
//
//nolint:gocognit,funlen // Cost calculation with fallback chain requires multiple nested conditions.
func (e *Engine) getActualCostForResource(
	ctx context.Context,
	resource ResourceDescriptor,
	request ActualCostRequest,
) (CostResult, []ErrorDetail) {
	var errors []ErrorDetail
	var resourceResult *CostResult
	log := logging.FromContext(ctx)

	// Select plugin matches using router (if configured) or all clients
	selectedMatches := e.selectPluginMatchesForResource(ctx, resource, "ActualCosts")

	fallbackChainBroken := false
	for i, match := range selectedMatches {
		if fallbackChainBroken {
			break
		}

		client := match.Client

		// Apply explicit adapter filter if specified
		if request.Adapter != "" && client.Name != request.Adapter {
			continue
		}

		costResult, err := e.getActualCostFromPlugin(
			ctx,
			client,
			resource,
			request.From,
			request.To,
		)
		if err != nil {
			// Check if fallback is enabled for this plugin
			if !match.Fallback {
				log.Info().
					Ctx(ctx).
					Str("component", "engine").
					Str("resource_type", resource.Type).
					Str("plugin", client.Name).
					Err(err).
					Msg("plugin failed and fallback disabled, stopping fallback chain")
				fallbackChainBroken = true
			} else if i < len(selectedMatches)-1 {
				// Log fallback event at INFO level per FR-020
				nextPlugin := selectedMatches[i+1].Client.Name
				log.Info().
					Ctx(ctx).
					Str("component", "engine").
					Str("resource_type", resource.Type).
					Str("failed_plugin", client.Name).
					Str("next_plugin", nextPlugin).
					Err(err).
					Msg("plugin failed, falling back to next priority plugin")
			}

			errors = append(errors, ErrorDetail{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				PluginName:   client.Name,
				Error:        fmt.Errorf("plugin call failed: %w", err),
				Timestamp:    time.Now(),
			})
			continue
		}
		if costResult != nil {
			engineResult := *costResult
			// Only allocate sustainability map if source has values
			if len(costResult.Sustainability) > 0 {
				engineResult.Sustainability = make(
					map[string]SustainabilityMetric,
					len(costResult.Sustainability),
				)
				for k, v := range costResult.Sustainability {
					engineResult.Sustainability[k] = SustainabilityMetric{
						Value: v.Value,
						Unit:  v.Unit,
					}
				}
			}
			resourceResult = &engineResult
			break
		}
	}

	if resourceResult != nil {
		return *resourceResult, errors
	}

	// Try state-based cost estimation as fallback
	if stateResult := e.tryStateBasedEstimation(ctx, resource, request); stateResult != nil {
		return *stateResult, errors
	}

	// Create placeholder result
	notes := "No actual cost data available"
	if len(errors) > 0 {
		notes = "ERROR: plugin call failed"
	}

	return CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Adapter:      "none",
		Currency:     defaultCurrency,
		TotalCost:    0,
		Confidence:   ConfidenceUnknown,
		Notes:        notes,
		StartDate:    request.From,
		EndDate:      request.To,
		CostPeriod:   FormatPeriod(request.From, request.To),
	}, errors
}

// tryStateBasedEstimation attempts to calculate costs using state-based estimation.
// Returns nil if state-based estimation is not possible for this resource.
func (e *Engine) tryStateBasedEstimation(
	ctx context.Context,
	resource ResourceDescriptor,
	request ActualCostRequest,
) *CostResult {
	log := logging.FromContext(ctx)

	created, extractErr := ExtractCreatedTimestamp(resource)
	if extractErr != nil {
		log.Debug().Ctx(ctx).Str("resource_id", resource.ID).Err(extractErr).
			Msg("cannot extract created timestamp, skipping state-based estimation")
		return nil
	}

	if created.IsZero() {
		return nil
	}

	hourlyRate, rateErr := e.getHourlyRateForResource(ctx, resource)
	if rateErr != nil {
		log.Debug().Ctx(ctx).Str("resource_id", resource.ID).Err(rateErr).
			Msg("cannot get hourly rate for state-based estimation")
		return nil
	}

	ref := time.Now()
	if !request.To.IsZero() && request.To.Before(time.Now()) {
		ref = request.To
	}

	start := created
	if !request.From.IsZero() && request.From.After(created) {
		start = request.From
	}

	isExternal := IsExternalResource(resource)
	stateCost := CalculateStateCost(StateCostInput{
		Resource:   resource,
		HourlyRate: hourlyRate,
		CreatedAt:  start,
		IsExternal: isExternal,
	}, ref)

	confidence := DetermineConfidence(false, isExternal)
	notes := stateCost.Notes
	if notes != "" {
		notes += "; " + UptimeAssumptionNote
	} else {
		notes = UptimeAssumptionNote
	}

	return &CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Adapter:      "estimate",
		Currency:     defaultCurrency,
		Hourly:       hourlyRate,
		TotalCost:    stateCost.TotalCost,
		Confidence:   confidence,
		Notes:        notes,
		StartDate:    start,
		EndDate:      ref,
		CostPeriod:   FormatPeriod(start, ref),
	}
}

// getHourlyRateForResource gets the hourly rate for a resource from projected cost.
func (e *Engine) getHourlyRateForResource(
	ctx context.Context,
	resource ResourceDescriptor,
) (float64, error) {
	for _, client := range e.clients {
		costResult, err := e.getProjectedCostFromPlugin(ctx, client, resource)
		if err != nil {
			continue
		}
		if costResult != nil {
			return costResult.Hourly, nil
		}
	}
	return 0, errors.New("no plugin returned projected cost")
}

func (e *Engine) getProjectedCostFromPlugin(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
) (*CostResult, error) {
	// Try to get pricing from plugin first
	req := &proto.GetProjectedCostRequest{
		Resources: []*proto.ResourceDescriptor{
			{
				ID:         resource.ID,
				Type:       resource.Type,
				Provider:   resource.Provider,
				Properties: convertToProto(resource.Properties),
			},
		},
	}

	// Note: Utilization from ctx (ContextKeyUtilization) is available for future use
	// when adapter supports passing it via gRPC metadata.

	resp, err := client.API.GetProjectedCost(ctx, req)
	if err == nil && len(resp.Results) > 0 {
		result := resp.Results[0]
		engineResult := &CostResult{
			ResourceType:   resource.Type,
			ResourceID:     resource.ID,
			Adapter:        client.Name,
			Currency:       result.Currency,
			Monthly:        result.MonthlyCost,
			Hourly:         result.HourlyCost,
			Notes:          result.Notes,
			Breakdown:      result.CostBreakdown,
			Sustainability: make(map[string]SustainabilityMetric),
		}

		for k, v := range result.Sustainability {
			engineResult.Sustainability[k] = SustainabilityMetric{
				Value: v.Value,
				Unit:  v.Unit,
			}
		}
		return engineResult, nil
	}

	return nil, ErrNoCostData
}

func (e *Engine) getProjectedCostFromSpec(
	ctx context.Context,
	resource ResourceDescriptor,
) *CostResult {
	service := extractService(resource.Type)
	sku := extractSKU(resource)

	spec := e.loadSpecWithFallback(ctx, resource.Provider, service, sku)
	if spec == nil {
		return nil
	}

	monthly, hourly := calculateCostsFromSpec(spec, resource)
	return e.createSpecBasedResult(resource, spec, monthly, hourly)
}

func (e *Engine) loadSpecWithFallback(
	ctx context.Context,
	provider, service, sku string,
) *PricingSpec {
	// Try provider-service-sku pattern first
	if sku != "" {
		if spec := e.tryLoadSpec(ctx, provider, service, sku); spec != nil {
			return spec
		}
	}

	// Fallback to provider-service-default pattern
	if spec := e.tryLoadSpec(ctx, provider, service, defaultServiceName); spec != nil {
		return spec
	}

	// Last resort: try common resource patterns
	commonSKUs := []string{"standard", "basic"}
	for _, commonSKU := range commonSKUs {
		if spec := e.tryLoadSpec(ctx, provider, service, commonSKU); spec != nil {
			return spec
		}
	}

	return nil
}

func (e *Engine) tryLoadSpec(ctx context.Context, provider, service, sku string) *PricingSpec {
	if loader, ok := e.loader.(interface {
		LoadSpecWithContext(ctx context.Context, provider, service, sku string) (interface{}, error)
	}); ok {
		specData, err := loader.LoadSpecWithContext(ctx, provider, service, sku)
		if err != nil {
			return nil
		}
		if spec, isSpec := specData.(*PricingSpec); isSpec {
			return spec
		}
		return nil
	}

	specData, err := e.loader.LoadSpec(provider, service, sku)
	if err != nil {
		return nil
	}
	if spec, isSpec := specData.(*PricingSpec); isSpec {
		return spec
	}
	return nil
}

func (e *Engine) createSpecBasedResult(
	resource ResourceDescriptor,
	spec *PricingSpec,
	monthly, hourly float64,
) *CostResult {
	return &CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Adapter:      "local-spec",
		Currency:     spec.Currency,
		Monthly:      monthly,
		Hourly:       hourly,
		Notes: fmt.Sprintf(
			"Calculated from local spec: %s-%s-%s",
			spec.Provider,
			spec.Service,
			spec.SKU,
		),
		Breakdown: map[string]float64{
			"base_cost": monthly,
		},
	}
}

func (e *Engine) getActualCostFromPlugin(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
	from, to time.Time,
) (*CostResult, error) {
	req := &proto.GetActualCostRequest{
		ResourceIDs: []string{resource.ID},
		StartTime:   from.Unix(),
		EndTime:     to.Unix(),
	}

	resp, err := client.API.GetActualCost(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Results) == 0 {
		return nil, ErrNoCostData
	}

	result := resp.Results[0]
	totalHours := to.Sub(from).Hours()
	totalDays := int(totalHours / hoursPerDay)

	// Calculate daily costs if we have breakdown data
	var dailyCosts []float64
	if totalDays > 0 {
		dailyCosts = make([]float64, totalDays)
		avgDaily := result.TotalCost / float64(totalDays)
		for i := range dailyCosts {
			dailyCosts[i] = avgDaily
		}
	}

	// Calculate monthly projection, avoiding divide by zero
	var monthlyRate float64
	var hourlyRate float64
	if totalDays > 0 {
		monthlyRate = result.TotalCost * avgDaysPerMonth / float64(totalDays)
	} else if totalHours > 0 {
		// If less than a day, project based on hourly rate
		monthlyRate = (result.TotalCost / totalHours) * hoursPerMonth
	}

	if totalHours > 0 {
		hourlyRate = result.TotalCost / totalHours
	}

	return &CostResult{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Adapter:      client.Name,
		Currency:     result.Currency,
		Monthly:      monthlyRate,
		Hourly:       hourlyRate,
		TotalCost:    result.TotalCost,
		DailyCosts:   dailyCosts,
		Notes: fmt.Sprintf(
			"Actual cost from %s to %s",
			from.Format("2006-01-02"),
			to.Format("2006-01-02"),
		),
		Breakdown:  result.CostBreakdown,
		StartDate:  from,
		EndDate:    to,
		CostPeriod: FormatPeriod(from, to),
	}, nil
}

func convertToProto(properties map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range properties {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

func extractService(resourceType string) string {
	// Extract service from resource type like "aws:ec2/instance:Instance" -> "ec2"
	parts := strings.Split(resourceType, ":")
	if len(parts) >= minProviderServiceParts {
		servicePath := parts[1]
		// Handle service/type format (e.g., "ec2/instance" -> "ec2")
		if slashPos := strings.Index(servicePath, "/"); slashPos > 0 {
			return servicePath[:slashPos]
		}
		return servicePath
	}
	return defaultServiceName
}

func extractSKU(resource ResourceDescriptor) string {
	// Try to extract SKU from resource properties first
	if sku := extractSKUFromProperties(resource.Properties); sku != "" {
		return sku
	}

	// Fallback to extracting from resource type
	return extractSKUFromType(resource.Type)
}

func extractSKUFromProperties(properties map[string]interface{}) string {
	if properties == nil {
		return ""
	}

	skuKeys := []string{"instanceType", "sku", "size", "type"}
	for _, key := range skuKeys {
		if skuStr, found := getStringProperty(properties, key); found {
			return skuStr
		}
	}
	return ""
}

func extractSKUFromType(resourceType string) string {
	parts := strings.Split(resourceType, ":")
	if len(parts) >= minProviderServiceTypeParts {
		return strings.ToLower(parts[2])
	}
	return ""
}

func getStringProperty(properties map[string]interface{}, key string) (string, bool) {
	if sku, ok := properties[key]; ok {
		if skuStr, isStr := sku.(string); isStr {
			return skuStr, true
		}
	}
	return "", false
}

func calculateCostsFromSpec(spec *PricingSpec, resource ResourceDescriptor) (float64, float64) {
	// Try to extract cost information from spec pricing
	if spec.Pricing != nil {
		if monthlyRate, hourlyRate, found := tryExtractCostsFromPricing(spec.Pricing, resource); found {
			return monthlyRate, hourlyRate
		}
	}

	// Ultimate fallback - conservative estimate based on resource type
	monthly := getDefaultMonthlyByType(resource.Type)
	hourly := monthly / hoursPerMonth
	return monthly, hourly
}

func tryExtractCostsFromPricing(
	pricing map[string]interface{},
	resource ResourceDescriptor,
) (float64, float64, bool) {
	// Try direct monthly estimate first
	if monthly, hourly, found := tryMonthlyEstimate(pricing); found {
		return monthly, hourly, true
	}

	// Try hourly rates
	if monthly, hourly, found := tryHourlyRates(pricing); found {
		return monthly, hourly, true
	}

	// Try storage-based pricing
	if monthly, hourly, found := tryStoragePricing(pricing, resource); found {
		return monthly, hourly, true
	}

	// Fallback to any numeric value
	return tryFallbackNumericValue(pricing)
}

func tryMonthlyEstimate(pricing map[string]interface{}) (float64, float64, bool) {
	if monthlyFloat, ok := getFloatFromPricing(pricing, "monthlyEstimate"); ok {
		monthly := monthlyFloat
		hourly := monthly / hoursPerMonth
		return monthly, hourly, true
	}
	return 0, 0, false
}

// tryHourlyRates attempts to extract an hourly rate from the given pricing map
// and derives a monthly estimate from it.
//
// The function looks for known hourly keys ("onDemandHourly", "hourlyRate") and,
// if found, returns the computed monthly cost (hourly * hoursPerMonth), the
// hourly rate, and true. If no hourly rate is present, it returns 0, 0, false.
//
// pricing: a map of pricing fields keyed by string; values are expected to contain
// numeric float64 entries for hourly rates.
//
// Returns the monthly estimate, the hourly rate, and a boolean indicating whether
// a valid hourly rate was found.
func tryHourlyRates(pricing map[string]interface{}) (float64, float64, bool) {
	hourlyKeys := []string{"onDemandHourly", "hourlyRate"}
	for _, key := range hourlyKeys {
		if hourlyFloat, ok := getFloatFromPricing(pricing, key); ok {
			hourly := hourlyFloat
			monthly := hourly * hoursPerMonth
			return monthly, hourly, true
		}
	}
	return 0, 0, false
}

// tryStoragePricing determines monthly and hourly costs for a storage resource when
// pricing contains a per-GB-per-month value.
//
// It returns the computed monthly cost (sizeGB * pricePerGBMonth), the hourly
// equivalent (monthly divided by hoursPerMonth), and `true` if both a storage size
// and a `pricePerGBMonth` numeric value were available. If the resource lacks a
// size or the pricing does not contain `pricePerGBMonth` as a float64, it returns
// zeros and `false`.
func tryStoragePricing(
	pricing map[string]interface{},
	resource ResourceDescriptor,
) (float64, float64, bool) {
	sizeGB, hasSize := getStorageSize(resource)
	if !hasSize {
		return 0, 0, false
	}

	if priceFloat, ok := getFloatFromPricing(pricing, "pricePerGBMonth"); ok {
		monthly := sizeGB * priceFloat
		hourly := monthly / hoursPerMonth
		return monthly, hourly, true
	}
	return 0, 0, false
}

func tryFallbackNumericValue(pricing map[string]interface{}) (float64, float64, bool) {
	for _, value := range pricing {
		if floatValue, ok := value.(float64); ok && floatValue > 0 {
			monthly := floatValue * hoursPerMonth // Assume it's hourly
			hourly := floatValue
			return monthly, hourly, true
		}
	}
	return 0, 0, false
}

func getFloatFromPricing(pricing map[string]interface{}, key string) (float64, bool) {
	if value, ok := pricing[key]; ok {
		if floatValue, isFloat := value.(float64); isFloat {
			return floatValue, true
		}
	}
	return 0, false
}

func getDefaultMonthlyByType(resourceType string) float64 {
	resourceTypeLower := strings.ToLower(resourceType)
	switch {
	case strings.Contains(resourceTypeLower, "database") ||
		strings.Contains(resourceTypeLower, "rds") ||
		strings.Contains(resourceTypeLower, ":db:") ||
		strings.Contains(resourceTypeLower, "sql"):
		return defaultDatabaseMonthlyCost
	case strings.Contains(resourceTypeLower, "storage") ||
		strings.Contains(resourceTypeLower, "s3"):
		return defaultStorageMonthlyCost
	default:
		return defaultComputeMonthlyCost
	}
}

func getStorageSize(resource ResourceDescriptor) (float64, bool) {
	if resource.Properties == nil {
		return 0, false
	}

	// Check common size property names
	sizeKeys := []string{"size", "sizeGb", "volumeSize", "allocatedStorage"}
	for _, key := range sizeKeys {
		if value, ok := resource.Properties[key]; ok {
			if size, parsed := parseFloatValue(value); parsed {
				return size, true
			}
		}
	}

	return 0, false
}

// parseFloatValue attempts to convert value to a float64.
// It accepts a float64, an int (converted to float64), or a numeric string.
// It returns the converted float64 and true on success, or 0 and false if conversion fails.
func parseFloatValue(value interface{}) (float64, bool) {
	if sizeFloat, ok := value.(float64); ok {
		return sizeFloat, true
	}
	if sizeInt, ok := value.(int); ok {
		return float64(sizeInt), true
	}
	if sizeStr, ok := value.(string); ok {
		// Try to parse size from string
		if parsed, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// AggregateResults aggregates a slice of CostResult into an AggregatedResults summary.
//
// AggregateResults computes total monthly and hourly costs, and builds maps of monthly
// costs grouped by provider, service, and adapter. The returned AggregatedResults
// contains the original Resources slice and a CostSummary populated with:
//   - Currency: taken from the first result (or defaultCurrency when input is empty).
//   - TotalMonthly and TotalHourly: summed across all results.
//   - ByProvider, ByService, ByAdapter: maps of monthly totals keyed by provider, service,
//     and adapter respectively.
//
// If the input slice is empty, AggregateResults returns an AggregatedResults with an
// AggregateResults aggregates a slice of CostResult into an AggregatedResults summary.
//
// If results is empty, it returns an AggregatedResults with zero totals, empty maps,
// an empty Resources slice, and Currency set to defaultCurrency. For a non-empty
// input, totals (TotalMonthly, TotalHourly) are summed across results, ByProvider,
// ByService, and ByAdapter maps accumulate monthly totals, Currency is taken from the
// first result, and Resources contains the original input slice.
func AggregateResults(results []CostResult) *AggregatedResults {
	if len(results) == 0 {
		return &AggregatedResults{
			Summary: CostSummary{
				Currency:   defaultCurrency,
				ByProvider: make(map[string]float64),
				ByService:  make(map[string]float64),
				ByAdapter:  make(map[string]float64),
				Resources:  []CostResult{},
			},
			Resources: []CostResult{},
		}
	}

	summary := CostSummary{
		Currency:   results[0].Currency, // Use currency from first result
		ByProvider: make(map[string]float64),
		ByService:  make(map[string]float64),
		ByAdapter:  make(map[string]float64),
		Resources:  results,
	}

	for _, result := range results {
		// Aggregate totals
		summary.TotalMonthly += result.Monthly
		summary.TotalHourly += result.Hourly

		// Aggregate by provider
		provider := extractProviderFromType(result.ResourceType)
		summary.ByProvider[provider] += result.Monthly

		// Aggregate by service
		service := extractService(result.ResourceType)
		summary.ByService[service] += result.Monthly

		// Aggregate by adapter
		summary.ByAdapter[result.Adapter] += result.Monthly
	}

	return &AggregatedResults{
		Summary:   summary,
		Resources: results,
	}
}

func extractProviderFromType(resourceType string) string {
	// Extract provider from resource type like "aws:ec2:Instance" -> "aws"
	parts := strings.Split(resourceType, ":")
	if len(parts) >= 1 {
		return parts[0]
	}
	return "unknown"
}

// FilterResources selects resources that match the provided filter expression.
// The filter is a single key=value expression (for example "provider=aws" or "tag:env=prod").
// An empty filter returns the input slice unchanged.
// The returned slice contains only the resources that satisfy the filter.
func FilterResources(resources []ResourceDescriptor, filter string) []ResourceDescriptor {
	if filter == "" {
		return resources
	}

	var filtered []ResourceDescriptor
	for _, resource := range resources {
		if matchesFilter(resource, filter) {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

// ValidateFilter validates the syntax of a filter expression.
// It returns an error if the filter is malformed: missing '=' separator,
// or if either the key or value is empty (for example: `type=aws:ec2/instance` or `tag:env=prod`).
func ValidateFilter(filter string) error {
	if filter == "" {
		return nil
	}
	parts := strings.SplitN(filter, "=", filterKeyValueParts)
	if len(parts) != filterKeyValueParts {
		return errors.New(
			"invalid filter syntax: expected 'key=value' (e.g., 'type=aws:ec2/instance' or 'tag:env=prod')",
		)
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return errors.New("filter key and value must be non-empty")
	}
	return nil
}

// matchesFilter reports whether the given resource satisfies the simple filter expression.
// The filter must be in the form "key=value". If the filter is empty or malformed, the
// function returns true (resource is included).
//
// Matching is case-insensitive and uses substring matching (contains).
// Supported builtin keys:
//   - "type": matches against ResourceDescriptor.Type
//   - "provider": matches the provider extracted from the type
//   - "service": matches the service extracted from the type
//   - "id": matches ResourceDescriptor.ID
//
// Any other key is evaluated against the resource's properties via matchesProperties.
func matchesFilter(resource ResourceDescriptor, filter string) bool {
	// Parse filter format like "type=aws:ec2/instance" or "provider=aws"
	parts := strings.SplitN(filter, "=", filterKeyValueParts)
	if len(parts) != filterKeyValueParts {
		return true // Invalid filter, include all
	}

	key := strings.ToLower(strings.TrimSpace(parts[0]))
	value := strings.ToLower(strings.TrimSpace(parts[1]))

	switch key {
	case "type":
		return strings.Contains(strings.ToLower(resource.Type), value)
	case "provider":
		provider := extractProviderFromType(resource.Type)
		return strings.Contains(strings.ToLower(provider), value)
	case "service":
		service := extractService(resource.Type)
		return strings.Contains(strings.ToLower(service), value)
	case "id":
		return strings.Contains(strings.ToLower(resource.ID), value)
	default:
		return matchesProperties(resource, key, value)
	}
}

// matchesProperties reports whether the properties of resource match the provided key and value.
// It performs case-insensitive comparisons. If key has the prefix "tag:", the prefix is removed
// and the function searches within "tags" or "labels" maps for a matching entry. The value is
// matched as a case-insensitive substring of the property's string representation or of a map
// entry's string value.
// resource: the resource whose Properties map is inspected.
// key: the property key to match; may be prefixed with "tag:" to force searching tags/labels.
// value: the substring to search for within the property's value.
// Returns true if a matching property or tag/label entry is found, false otherwise.
func matchesProperties(resource ResourceDescriptor, key, value string) bool {
	if resource.Properties == nil {
		return false
	}

	propKey := key
	if strings.HasPrefix(key, "tag:") {
		propKey = strings.TrimPrefix(key, "tag:")
	}

	for k, v := range resource.Properties {
		if strings.ToLower(k) == propKey {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", v)), value) {
				return true
			}
		}
		// Also check if this property is a map and contains the key (for tags/labels)
		kl := strings.ToLower(k)
		if kl == "tags" || kl == "labels" {
			if m, ok := v.(map[string]interface{}); ok {
				if matchInMap(m, propKey, value) {
					return true
				}
			}
		}
	}
	return false
}

// matchInMap reports whether m contains an entry whose key equals propKey (case-insensitive)
// and whose value, when formatted to a string, contains value as a substring (case-insensitive).
//
// propKey and value are matched against the map contents in a case-insensitive manner; if a
// matching key is found and its stringified value contains the provided value, the function
// returns true, otherwise it returns false.
func matchInMap(m map[string]interface{}, propKey, value string) bool {
	for mk, mv := range m {
		if strings.ToLower(mk) == propKey {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", mv)), value) {
				return true
			}
		}
	}
	return false
}

// MatchesTags checks if resource properties match the specified tag filters.
func MatchesTags(resource ResourceDescriptor, tags map[string]string) bool {
	if len(tags) == 0 {
		return true
	}

	for key, expectedValue := range tags {
		if prop, exists := resource.Properties[key]; exists {
			if propStr := fmt.Sprintf("%v", prop); propStr == expectedValue {
				continue
			}
		}
		return false
	}
	return true
}

// FormatPeriod formats a time duration into a human-readable period string.
func FormatPeriod(from, to time.Time) string {
	days := int(to.Sub(from).Hours() / hoursPerDay)
	if days == 1 {
		return "1 day"
	}
	if days < daysPerWeek {
		return fmt.Sprintf("%d days", days)
	}
	if days < daysPerMonth {
		weeks := days / daysPerWeek
		return fmt.Sprintf("%d weeks", weeks)
	}
	months := days / daysPerMonth
	return fmt.Sprintf("%d months", months)
}

// GroupResults groups cost results by the specified grouping strategy.
func (e *Engine) GroupResults(results []CostResult, groupBy GroupBy) []CostResult {
	if groupBy == GroupByNone {
		return results
	}

	groups := make(map[string][]CostResult)

	for _, result := range results {
		var key string
		switch groupBy {
		case GroupByNone:
			// Should not reach here since we return early if GroupByNone
			key = defaultServiceName
		case GroupByResource:
			key = fmt.Sprintf("%s/%s", result.ResourceType, result.ResourceID)
		case GroupByType:
			key = result.ResourceType
		case GroupByProvider:
			// Extract provider from resource type (e.g., "aws:ec2/instance:Instance" -> "aws")
			if parts := strings.Split(result.ResourceType, ":"); len(parts) > 0 {
				key = parts[0]
			} else {
				key = "unknown"
			}
		case GroupByDate:
			key = result.StartDate.Format("2006-01-02")
		case GroupByDaily:
			key = result.StartDate.Format("2006-01-02")
		case GroupByMonthly:
			key = result.StartDate.Format("2006-01")
		default:
			key = defaultServiceName
		}

		groups[key] = append(groups[key], result)
	}

	var grouped []CostResult
	for groupKey, groupResults := range groups {
		if len(groupResults) == 1 {
			grouped = append(grouped, groupResults[0])
		} else {
			// Aggregate multiple results into one
			aggregated := AggregateResultsInternal(groupResults, groupKey)
			grouped = append(grouped, aggregated)
		}
	}

	return grouped
}

// AggregateResultsInternal aggregates multiple CostResult entries into a single CostResult.
// AggregateResultsInternal sums numeric totals (Monthly, Hourly, TotalCost), merges breakdown maps
// by summing values for matching keys, and combines daily cost series by aligning indices and summing
// per-day values. The returned CostResult preserves the Currency, StartDate, EndDate and CostPeriod
// from the first entry in results, sets ResourceType to groupName, ResourceID to
// "aggregated-<N>-resources", Adapter to "aggregated", and Notes to indicate the number of
// AggregateResultsInternal aggregates multiple CostResult entries into a single CostResult that represents the grouped resources.
//
// AggregateResultsInternal sums numeric totals (Monthly, Hourly, TotalCost), merges Breakdown maps by summing values for matching keys,
// and aligns/sums DailyCosts across results, extending the slice to match the longest daily-cost series. The returned result preserves
// Currency, StartDate, EndDate, and CostPeriod from the first entry, and sets ResourceType to the provided groupName, ResourceID to
// "aggregated-N-resources", Adapter to "aggregated", and Notes to indicate the number of aggregated resources.
//
// Parameters:
//   - results: slice of CostResult entries to aggregate. If empty, an empty CostResult is returned.
//   - groupName: name to assign to the aggregated ResourceType.
//
// Returns:
//
//	The aggregated CostResult combining all provided entries.
func AggregateResultsInternal(results []CostResult, groupName string) CostResult {
	if len(results) == 0 {
		return CostResult{}
	}

	first := results[0]
	aggregated := CostResult{
		ResourceType: groupName,
		ResourceID:   fmt.Sprintf("aggregated-%d-resources", len(results)),
		Adapter:      "aggregated",
		Currency:     first.Currency,
		StartDate:    first.StartDate,
		EndDate:      first.EndDate,
		CostPeriod:   first.CostPeriod,
		Breakdown:    make(map[string]float64),
	}

	for _, result := range results {
		aggregated.Monthly += result.Monthly
		aggregated.Hourly += result.Hourly
		aggregated.TotalCost += result.TotalCost

		// Merge breakdowns
		for key, value := range result.Breakdown {
			aggregated.Breakdown[key] += value
		}

		// Extend daily costs
		if len(result.DailyCosts) > 0 {
			if len(aggregated.DailyCosts) < len(result.DailyCosts) {
				// Extend slice to match the longest daily costs
				extended := make([]float64, len(result.DailyCosts))
				copy(extended, aggregated.DailyCosts)
				aggregated.DailyCosts = extended
			}
			for i, dailyCost := range result.DailyCosts {
				if i < len(aggregated.DailyCosts) {
					aggregated.DailyCosts[i] += dailyCost
				}
			}
		}
	}

	aggregated.Notes = fmt.Sprintf("Aggregated costs from %d resources", len(results))
	return aggregated
}

// CreateCrossProviderAggregation creates daily/monthly cross-provider aggregation from cost results.
//
// This function aggregates costs across multiple cloud providers and time periods, enabling
// comprehensive cost analysis and reporting. It supports both daily and monthly aggregation
// with built-in currency validation and input validation.
//
// Parameters:
//   - results: Slice of CostResult objects containing cost data from various providers.
//     Each result must have consistent currency and valid date ranges.
//   - groupBy: Must be GroupByDaily or GroupByMonthly. Other grouping types will return ErrInvalidGroupBy.
//
// Returns:
//   - []CrossProviderAggregation: Sorted aggregations by time period, each containing:
//   - Period: Date ("2006-01-02") for daily or month ("2006-01") for monthly
//   - Providers: Map of provider names to their costs for that period
//   - Total: Sum of all provider costs for the period
//   - Currency: Consistent currency across all results
//   - error: Various validation errors:
//   - ErrEmptyResults: If results slice is empty
//   - ErrInvalidGroupBy: If groupBy is not time-based (daily/monthly)
//   - ErrMixedCurrencies: If results contain different currencies
//   - ErrInvalidDateRange: If any result has EndDate before StartDate
//
// Usage Examples:
//
//	// Daily aggregation across AWS and Azure costs
//	results := []CostResult{
//		{ResourceType: "aws:ec2:Instance", TotalCost: 100.0, Currency: "USD", StartDate: jan1, EndDate: jan2},
//		{ResourceType: "azure:compute:VirtualMachine", TotalCost: 150.0, Currency: "USD", StartDate: jan1, EndDate: jan2},
//	}
//	aggregations, err := CreateCrossProviderAggregation(results, GroupByDaily)
//	if err != nil {
//		return fmt.Errorf("aggregation failed: %w", err)
//	}
//	// Result: [{Period: "2024-01-01", Providers: {"aws": 100.0, "azure": 150.0}, Total: 250.0, Currency: "USD"}]
//
//	// Monthly aggregation for cost trend analysis
//	aggregations, err := CreateCrossProviderAggregation(results, GroupByMonthly)
//	// Result: [{Period: "2024-01", Providers: {"aws": 3100.0, "azure": 4650.0}, Total: 7750.0, Currency: "USD"}]
//
// Error Scenarios:
//   - Mixed currencies: Results with different currencies (USD vs EUR) will fail validation
//   - Invalid grouping: Using GroupByResource or GroupByType will return ErrInvalidGroupBy
//   - Date validation: Results with EndDate before StartDate will fail
//   - Empty input: Empty results slice returns ErrEmptyResults
//
// Performance Considerations:
//   - Efficient for datasets up to 10,000 cost results
//   - Memory usage scales linearly with unique time periods
//
// CreateCrossProviderAggregation groups cost results into time-based cross-provider aggregations.
//
// CreateCrossProviderAggregation validates the input results and the provided time-based grouping, ensures all results use a single currency, then groups costs by period (daily or monthly) and provider. It produces a sorted slice of CrossProviderAggregation entries where each entry contains the period string, per-provider totals, the period total, and the common currency.
//
// Parameters:
//   - results: the list of CostResult entries to aggregate.
//   - groupBy: a time-based GroupBy value (daily or monthly) that determines the period granularity.
//
// Returns:
//   - a sorted slice of CrossProviderAggregation entries ordered by period.
//
// CreateCrossProviderAggregation groups a slice of CostResult entries by the specified time period
// and returns cross-provider aggregations for each period sorted by period key.
//
// The function accepts:
//   - results: the cost results to aggregate (must be non-empty).
//   - groupBy: the time-based grouping to apply (daily or monthly).
//
// It validates inputs, enforces that all results use a single currency (empty currency values
// are treated as the package default), and groups costs by period and provider. Each returned
// CrossProviderAggregation contains the period key, per-provider totals, the overall total, and
// the resolved currency.
//
// Possible errors:
//   - ErrEmptyResults if `results` is empty.
//   - ErrInvalidGroupBy if `groupBy` is not a time-based grouping.
//   - ErrInvalidDateRange if any result has an EndDate not after its StartDate.
//
// - the results contain more than one distinct currency (ErrMixedCurrencies).
func CreateCrossProviderAggregation(
	results []CostResult,
	groupBy GroupBy,
) ([]CrossProviderAggregation, error) {
	// Input validation
	if err := validateAggregationInputs(results, groupBy); err != nil {
		return nil, err
	}

	// Validate currency consistency
	if err := validateCurrencyConsistency(results); err != nil {
		return nil, err
	}

	// Group results by time period
	periods, baseCurrency := groupResultsByPeriod(results, groupBy)

	// Convert to sorted aggregations
	aggregations := createSortedAggregations(periods, baseCurrency)

	return aggregations, nil
}

// validateAggregationInputs validates the inputs for cross-provider aggregation.
//
// This function performs comprehensive input validation to ensure cross-provider
// aggregation can proceed safely with valid data.
//
// Parameters:
//   - results: Cost results to validate. Must be non-empty slice.
//   - groupBy: Grouping type. Must be time-based (GroupByDaily or GroupByMonthly).
//
// Returns:
//   - error: Specific validation errors:
//   - ErrEmptyResults: If results slice is empty or nil
//   - ErrInvalidGroupBy: If groupBy is not time-based grouping
//   - ErrInvalidDateRange: If any result has invalid date range (EndDate ≤ StartDate)
//
// Validation Rules:
//  1. Results slice must contain at least one element
//  2. GroupBy must be GroupByDaily or GroupByMonthly (checked via IsTimeBasedGrouping())
//  3. All results with both StartDate and EndDate must have EndDate after StartDate
//  4. Zero dates (time.IsZero()) are allowed and skipped during validation
//
// Usage Example:
//
//	// Validate before aggregation
//	if err := validateAggregationInputs(results, GroupByDaily); err != nil {
//		return fmt.Errorf("invalid aggregation inputs: %w", err)
//
// validateAggregationInputs checks that the inputs for cross-provider aggregation are valid.
// It returns ErrEmptyResults if no results are provided, ErrInvalidGroupBy if the provided
// groupBy is not a time-based grouping, and ErrInvalidDateRange if any result has both
// validateAggregationInputs verifies that the inputs to cross-provider aggregation are valid.
// It returns an error if the results slice is empty, if the provided GroupBy is not a time-based
// grouping, or if any CostResult has both StartDate and EndDate set where EndDate is not after StartDate.
// Parameters:
//   - results: slice of CostResult entries to validate.
//   - groupBy: grouping mode to validate for time-based grouping requirements.
//
// Returns:
//   - nil if all inputs are valid.
//   - ErrEmptyResults if results is empty.
//   - ErrInvalidGroupBy if groupBy is not time-based.
//   - ErrInvalidDateRange if any result has an EndDate that is not after its StartDate.
func validateAggregationInputs(results []CostResult, groupBy GroupBy) error {
	if len(results) == 0 {
		return ErrEmptyResults
	}

	if !groupBy.IsTimeBasedGrouping() {
		return ErrInvalidGroupBy
	}

	// Validate date ranges in results
	for _, result := range results {
		if !result.EndDate.IsZero() && !result.StartDate.IsZero() {
			if !result.EndDate.After(result.StartDate) {
				return ErrInvalidDateRange
			}
		}
	}

	return nil
}

// validateCurrencyConsistency ensures all results use the same currency.
//
// Cross-provider aggregation requires consistent currency to produce meaningful
// cost totals. This function validates that all cost results use the same currency,
// with automatic defaulting to USD for empty currency fields.
//
// Parameters:
//   - results: Slice of CostResult objects to validate for currency consistency.
//
// Returns:
//   - error: Currency validation error:
//   - nil: All results use consistent currency
//   - ErrMixedCurrencies: Results contain different currencies with details
//
// Currency Handling Rules:
//  1. Empty currency strings are automatically treated as USD (defaultCurrency)
//  2. First result's currency (or USD if empty) becomes the baseline
//  3. All subsequent results must match the baseline currency
//  4. Comparison is case-sensitive ("USD" ≠ "usd")
//  5. Empty results slice passes validation (returns nil)
//
// Usage Examples:
//
//	// Valid: All USD currencies
//	results := []CostResult{
//		{Currency: "USD", TotalCost: 100.0},
//		{Currency: "USD", TotalCost: 200.0},
//		{Currency: "", TotalCost: 50.0}, // Treated as USD
//	}
//	err := validateCurrencyConsistency(results) // Returns nil
//
//	// Invalid: Mixed currencies
//	results := []CostResult{
//		{Currency: "USD", TotalCost: 100.0},
//		{Currency: "EUR", TotalCost: 200.0},
//	}
//	err := validateCurrencyConsistency(results)
//	// Returns: ErrMixedCurrencies: found USD and EUR
//
// Error Message Format:
//
// validateCurrencyConsistency checks that all CostResult entries use the same currency.
// It treats an empty currency on any result as the package default currency (`defaultCurrency`).
// If results is empty the function returns nil.
// validateCurrencyConsistency checks that all CostResult entries use the same currency.
// It treats an empty Currency on a result as the package-level defaultCurrency.
// If results is empty or all currencies (after applying the default) match, it returns nil.
// If more than one distinct currency is found, it returns ErrMixedCurrencies wrapped with
// details of the differing currencies.
func validateCurrencyConsistency(results []CostResult) error {
	if len(results) == 0 {
		return nil
	}

	baseCurrency := results[0].Currency
	if baseCurrency == "" {
		baseCurrency = defaultCurrency // Default to USD if empty
	}

	for _, result := range results {
		currency := result.Currency
		if currency == "" {
			currency = defaultCurrency // Default to USD if empty
		}
		if currency != baseCurrency {
			return fmt.Errorf("%w: found %s and %s", ErrMixedCurrencies, baseCurrency, currency)
		}
	}

	return nil
}

// groupResultsByPeriod groups results by time period and returns the base currency.
//
// This function organizes cost results into time-based buckets (daily or monthly)
// and aggregates costs by cloud provider within each period. It forms the core
// data structure for cross-provider cost analysis and reporting.
//
// Parameters:
//   - results: Slice of CostResult objects with valid StartDate fields.
//   - groupBy: Time-based grouping (GroupByDaily or GroupByMonthly).
//
// Returns:
//   - map[string]map[string]float64: Nested map structure:
//   - Outer key: Time period ("2006-01-02" for daily, "2006-01" for monthly)
//   - Inner key: Provider name extracted from ResourceType ("aws", "azure", "gcp")
//   - Inner value: Aggregated cost for that provider in that period
//   - string: Base currency from first result (or USD if first result has empty currency)
//
// Period Formatting:
//   - Daily: "2024-01-15" (YYYY-MM-DD format)
//   - Monthly: "2024-01" (YYYY-MM format)
//
// Cost Calculation Logic:
//  1. Uses TotalCost if available (actual historical costs)
//  2. Falls back to Monthly cost if TotalCost is zero
//  3. For daily grouping, converts monthly estimate to daily (Monthly ÷ 30.44)
//  4. Aggregates multiple results for same provider/period
//
// Provider Extraction:
//   - Extracts from ResourceType: "aws:ec2:Instance" → "aws"
//   - Handles malformed types: "invalid" → "unknown"
//
// Usage Example:
//
//	results := []CostResult{
//		{ResourceType: "aws:ec2:Instance", TotalCost: 100.0, Currency: "USD", StartDate: jan15},
//		{ResourceType: "azure:compute:VM", TotalCost: 150.0, Currency: "USD", StartDate: jan15},
//		{ResourceType: "aws:s3:Bucket", TotalCost: 25.0, Currency: "USD", StartDate: jan16},
//	}
//	periods, currency := groupResultsByPeriod(results, GroupByDaily)
//	// Result:
//	// periods = {
//	//   "2024-01-15": {"aws": 100.0, "azure": 150.0},
//	//   "2024-01-16": {"aws": 25.0}
//	// }
//
// distributeDailyCosts adds the entries from result.DailyCosts into the periods map for the specified provider,
// grouping each daily cost into either a daily ("YYYY-MM-DD") or monthly ("YYYY-MM") period based on groupBy.
// The function mutates the provided periods map and creates nested maps as needed.
// Parameters:
//   - periods: map keyed by period string to a map of provider -> accumulated cost.
//   - result: CostResult whose StartDate and DailyCosts define the per-day values to distribute.
//   - provider: provider identifier used as the key within each period's nested map.
//   - groupBy: determines whether costs are grouped by day (GroupByDaily) or by month (GroupByMonthly).
func distributeDailyCosts(
	periods map[string]map[string]float64,
	result CostResult,
	provider string,
	groupBy GroupBy,
) {
	for i, dc := range result.DailyCosts {
		day := result.StartDate.Add(time.Duration(i) * 24 * time.Hour)
		var p string
		if groupBy == GroupByDaily {
			p = day.Format("2006-01-02")
		} else { // GroupByMonthly
			p = day.Format("2006-01")
		}
		if periods[p] == nil {
			periods[p] = make(map[string]float64)
		}
		periods[p][provider] += dc
	}
}

// groupResultsByPeriod groups cost results into time periods and accumulates costs per provider.
// It returns a map keyed by a formatted period string (e.g., "YYYY-MM-DD" for daily, "YYYY-MM" for monthly)
// whose values are maps from provider name to aggregated cost for that period.
// The second return value is the base currency chosen from the first result (or the package default if empty or no results).
//
// Parameters:
//   - results: slice of CostResult entries to group. Each entry's StartDate and ResourceType are used to determine the period and provider.
//   - groupBy: grouping period (daily or monthly) used to format the period keys.
//
// Returns:
//   - periods: map[period]string -> map[provider]string -> total cost for that provider in the period.
//
// groupResultsByPeriod groups CostResult entries by the time period indicated by groupBy and aggregates costs per provider for each period.
// Results that include DailyCosts are distributed across periods by day; otherwise each result's cost is attributed to the period computed from its StartDate.
// The function returns a map keyed by period string (period -> provider -> aggregated cost) and the base currency used for the aggregation.
// The base currency is taken from the first result's Currency if present, otherwise the package defaultCurrency is used.
// Parameters:
//   - results: slice of CostResult to group and aggregate.
//
// groupResultsByPeriod groups cost results into time-based periods and sums costs per provider.
// It assigns each CostResult's cost to a period derived from its StartDate using the given
// groupBy granularity (daily or monthly). If a result contains DailyCosts and a valid
// StartDate, those daily amounts are distributed across the corresponding periods; otherwise
// the result's total cost or monthly projection is used for the single period containing
// StartDate.
// Parameters:
//   - results: slice of CostResult entries to group.
//   - groupBy: grouping granularity (e.g., GroupByDaily or GroupByMonthly) used to format
//     period keys and compute period costs.
//
// Returns a map keyed by period string to a map of provider -> aggregated cost, and the
// base currency determined from the first result (or defaultCurrency when absent or when
// results is empty).
func groupResultsByPeriod(
	results []CostResult,
	groupBy GroupBy,
) (map[string]map[string]float64, string) {
	periods := make(map[string]map[string]float64) // period -> provider -> cost
	baseCurrency := defaultCurrency                // Default currency

	if len(results) > 0 {
		baseCurrency = results[0].Currency
		if baseCurrency == "" {
			baseCurrency = defaultCurrency
		}
	}

	for _, result := range results {
		provider := extractProviderFromType(result.ResourceType)

		// Prefer distributing per-day amounts when available
		if len(result.DailyCosts) > 0 && !result.StartDate.IsZero() {
			distributeDailyCosts(periods, result, provider, groupBy)
			continue
		}

		// Fallback: treat the whole result as a single-period amount
		period := formatPeriodForGrouping(result.StartDate, groupBy)
		if periods[period] == nil {
			periods[period] = make(map[string]float64)
		}
		cost := calculateCostForPeriod(result, groupBy)
		periods[period][provider] += cost
	}

	return periods, baseCurrency
}

// formatPeriodForGrouping formats a date according to the grouping type.
//
// This function converts time.Time values into string periods suitable for
// grouping and sorting in cross-provider aggregations.
//
// Parameters:
//   - date: Time value to format (typically from CostResult.StartDate).
//   - groupBy: Determines output format (GroupByDaily or GroupByMonthly).
//
// Returns:
//   - string: Formatted period:
//   - GroupByDaily: "2006-01-02" (ISO date format)
//   - GroupByMonthly: "2006-01" (year-month format)
//   - Other groupBy values: Default to monthly format
//
// Format Examples:
//   - Daily: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC) → "2024-01-15"
//   - Monthly: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC) → "2024-01"
//
// Usage in Aggregation:
//   - Periods are used as map keys for grouping
//   - String format ensures consistent sorting
//
// formatPeriodForGrouping returns a period string for the given date suitable for grouping.
// formatPeriodForGrouping returns a period string for the given date based on groupBy.
// For GroupByDaily it returns "YYYY-MM-DD"; for other time-based groupings it returns "YYYY-MM".
// date is the time to format and groupBy selects the time resolution used for formatting.
func formatPeriodForGrouping(date time.Time, groupBy GroupBy) string {
	if groupBy == GroupByDaily {
		return date.Format("2006-01-02")
	}
	return date.Format("2006-01")
}

// calculateCostForPeriod calculates the appropriate cost for a given period.
//
// This function determines the most accurate cost value for aggregation based
// on available cost data and the time period being aggregated.
//
// Parameters:
//   - result: CostResult containing cost data and metadata.
//   - groupBy: Time period type (GroupByDaily or GroupByMonthly).
//
// Returns:
//   - float64: Calculated cost appropriate for the time period:
//   - For actual costs: Uses TotalCost (historical data from cloud APIs)
//   - For projected costs: Uses Monthly cost with daily conversion if needed
//   - Zero if no cost data available
//
// Cost Selection Logic:
//  1. **Actual Costs (Preferred)**: Uses TotalCost if > 0 (historical data)
//  2. **Projected Costs (Fallback)**: Uses Monthly if TotalCost is 0
//  3. **Daily Conversion**: For GroupByDaily, converts Monthly ÷ 30.44 (avgDaysPerMonth)
//  4. **Zero Fallback**: Returns 0 if no cost data available
//
// Conversion Constants:
//   - avgDaysPerMonth = 30.44 (accurate average for monthly→daily conversion)
//   - Maintains precision for financial calculations
//
// Usage Examples:
//
//	// Actual cost (historical data)
//	result := CostResult{TotalCost: 150.0, Monthly: 3100.0}
//	cost := calculateCostForPeriod(result, GroupByDaily)   // Returns 150.0
//	cost = calculateCostForPeriod(result, GroupByMonthly) // Returns 150.0
//
//	// Projected cost (estimated)
//	result := CostResult{TotalCost: 0, Monthly: 3100.0}
//	cost := calculateCostForPeriod(result, GroupByDaily)   // Returns 101.81 (3100/30.44)
//
// calculateCostForPeriod computes the cost to use for a given time grouping for a single CostResult.
// It prefers an explicit per-day breakdown: if DailyCosts is present it returns the sum of those values.
// If DailyCosts is empty it uses TotalCost when non-zero.
// If TotalCost is zero and Monthly is available it uses Monthly, converting Monthly to a daily estimate when groupBy is GroupByDaily.
// calculateCostForPeriod computes the cost for the specified grouping period using a CostResult.
// It prefers per-day entries when present, falls back to TotalCost, and otherwise derives a period
// estimate from Monthly (converting to a daily value when groupBy is GroupByDaily).
//
// Parameters:
//   - result: the CostResult containing potential DailyCosts, TotalCost, and Monthly values.
//   - groupBy: the grouping granularity used to interpret Monthly values (daily vs. monthly).
//
// Returns:
//   - the cost for the requested period as a float64. Returns 0 if no suitable cost fields are available.
func calculateCostForPeriod(result CostResult, groupBy GroupBy) float64 {
	// Use DailyCosts when available for more accurate cost calculations
	if len(result.DailyCosts) > 0 {
		// For all grouping types, sum all daily costs to get total period cost
		var totalCost float64
		for _, dailyCost := range result.DailyCosts {
			totalCost += dailyCost
		}
		return totalCost
	}

	// Fallback to TotalCost if available, otherwise use Monthly projection
	cost := result.TotalCost
	if cost == 0 && result.Monthly > 0 {
		if groupBy == GroupByDaily {
			// Convert monthly to daily estimate
			cost = result.Monthly / avgDaysPerMonth
		} else {
			cost = result.Monthly
		}
	}
	return cost
}

// createSortedAggregations creates sorted aggregations from period data.
//
// This function converts the grouped period data into a sorted slice of
// CrossProviderAggregation objects suitable for reporting and analysis.
//
// Parameters:
//   - periods: Nested map from groupResultsByPeriod():
//   - Key: Time period string ("2006-01-02" or "2006-01")
//   - Value: Map of provider names to aggregated costs
//   - baseCurrency: Currency code to apply to all aggregations (e.g., "USD")
//
// Returns:
//   - []CrossProviderAggregation: Sorted slice with each element containing:
//   - Period: Time period string
//   - Providers: Map of provider names to costs
//   - Total: Sum of all provider costs for the period
//   - Currency: Consistent currency across all aggregations
//
// Processing Steps:
//  1. **Iterate**: Process each period in the input map
//  2. **Calculate Totals**: Sum all provider costs within each period
//  3. **Build Objects**: Create CrossProviderAggregation with complete data
//  4. **Sort**: Order by period string (lexicographic = chronological)
//
// Sorting Behavior:
//   - Daily periods: "2024-01-01" < "2024-01-02" < "2024-01-15"
//   - Monthly periods: "2024-01" < "2024-02" < "2024-12"
//   - Lexicographic string comparison ensures chronological order
//
// Usage Example:
//
//	periods := map[string]map[string]float64{
//		"2024-01-02": {"aws": 100.0, "azure": 200.0},
//		"2024-01-01": {"aws": 150.0, "gcp": 75.0},
//	}
//	aggregations := createSortedAggregations(periods, "USD")
//	// Result: [
//	//   {Period: "2024-01-01", Providers: {"aws": 150.0, "gcp": 75.0}, Total: 225.0, Currency: "USD"},
//	//   {Period: "2024-01-02", Providers: {"aws": 100.0, "azure": 200.0}, Total: 300.0, Currency: "USD"}
//
// createSortedAggregations converts a map of period -> provider -> cost into a slice of
// CrossProviderAggregation entries and returns them sorted by the Period string in ascending order.
// The `periods` parameter maps a period identifier (e.g., "YYYY-MM-DD" or "YYYY-MM") to a map
// of provider names to their aggregated cost for that period. The `baseCurrency` value is set
// as the Currency for every returned aggregation.
// The returned slice contains one CrossProviderAggregation per period with Providers populated,
// createSortedAggregations builds a sorted slice of CrossProviderAggregation from a map of periods to provider costs.
//
// The `periods` parameter is a map where each key is a period string (e.g., "2025-10-01" or "2025-10") and the value
// is a map of provider name to cost for that period. For each period an aggregation is produced with `Total` set to the
// sum of all provider costs and `Currency` set to `baseCurrency`.
//
// The resulting slice is sorted by the `Period` field in ascending (lexicographic) order. If `periods` is empty, an
// createSortedAggregations converts a map of period -> provider -> cost into a
// sorted slice of CrossProviderAggregation entries.
//
// The `periods` parameter maps a period string (e.g., "YYYY-MM" or "YYYY-MM-DD")
// to a map of provider names and their aggregated cost for that period. The
// `baseCurrency` parameter is used as the Currency value for every aggregation.
// The returned slice is sorted in ascending order by the Period field. If
// `periods` is empty an empty slice is returned.
func createSortedAggregations(
	periods map[string]map[string]float64,
	baseCurrency string,
) []CrossProviderAggregation {
	var aggregations []CrossProviderAggregation

	for period, providers := range periods {
		total := 0.0
		for _, cost := range providers {
			total += cost
		}

		aggregations = append(aggregations, CrossProviderAggregation{
			Period:    period,
			Providers: providers,
			Total:     total,
			Currency:  baseCurrency,
		})
	}

	// Sort aggregations by period
	sort.Slice(aggregations, func(i, j int) bool {
		return aggregations[i].Period < aggregations[j].Period
	})

	return aggregations
}

// convertProtoRecommendation converts a proto recommendation to an engine Recommendation.
// The plugin is expected to populate ResourceID from the Id field sent in ResourceDescriptor.
func convertProtoRecommendation(rec *proto.Recommendation) Recommendation {
	engineRec := Recommendation{
		ResourceID:  rec.ResourceID,
		Type:        rec.ActionType,
		Description: rec.Description,
	}

	if engineRec.Type == "" {
		engineRec.Type = rec.Category
	}

	if rec.Impact != nil {
		engineRec.EstimatedSavings = rec.Impact.EstimatedSavings
		engineRec.Currency = rec.Impact.Currency
	}

	return engineRec
}

// GetRecommendationsForResources fetches cost optimization recommendations for the given resources.
// For large datasets (>100 resources), it uses batch processing to improve performance and memory usage.
//
//nolint:gocognit // Complex orchestration function with caching and batch processing.
func (e *Engine) GetRecommendationsForResources(
	ctx context.Context,
	resources []ResourceDescriptor,
) (*RecommendationsResult, error) {
	log := logging.FromContext(ctx)
	result := &RecommendationsResult{
		Currency: defaultCurrency,
	}

	if len(resources) == 0 {
		return result, nil
	}

	// Check cache if enabled
	//nolint:nestif // Cache lookup requires nested checks for key generation, cache hit, and unmarshaling.
	if e.cache != nil && e.cache.IsEnabled() {
		cacheKey, keyErr := e.generateRecommendationsCacheKey(resources)
		if keyErr == nil {
			if cachedEntry, cacheErr := e.cache.Get(cacheKey); cacheErr == nil && cachedEntry != nil {
				log.Debug().
					Ctx(ctx).
					Str("component", "engine").
					Str("cache_key", cacheKey).
					Msg("cache hit for recommendations")

				// Unmarshal cached result
				var cachedResult RecommendationsResult
				if unmarshalErr := json.Unmarshal(cachedEntry.Data, &cachedResult); unmarshalErr == nil {
					return &cachedResult, nil
				}
				log.Warn().
					Ctx(ctx).
					Str("component", "engine").
					Msg("failed to unmarshal cached recommendations")
			}
		}
	}

	// Use batch processing for large datasets
	useBatchProcessing := len(resources) > batchProcessingThreshold

	for _, client := range e.clients {
		log.Info().
			Ctx(ctx).
			Str("component", "engine").
			Str("plugin", client.Name).
			Int("resource_count", len(resources)).
			Bool("batch_processing", useBatchProcessing).
			Msg("fetching recommendations from plugin")

		if useBatchProcessing {
			if err := e.fetchRecommendationsWithBatching(ctx, client, resources, result); err != nil {
				log.Warn().
					Ctx(ctx).
					Str("component", "engine").
					Str("plugin", client.Name).
					Err(err).
					Msg("failed to fetch recommendations with batching")

				result.Errors = append(result.Errors, RecommendationError{
					PluginName: client.Name,
					Error:      err.Error(),
				})
			}
		} else {
			if err := e.fetchRecommendationsSequential(ctx, client, resources, result); err != nil {
				log.Warn().
					Ctx(ctx).
					Str("component", "engine").
					Str("plugin", client.Name).
					Err(err).
					Msg("failed to fetch recommendations")

				result.Errors = append(result.Errors, RecommendationError{
					PluginName: client.Name,
					Error:      err.Error(),
				})
			}
		}
	}

	// Store result in cache if enabled
	//nolint:nestif // Cache storage requires nested checks for key generation, marshaling, and storage.
	if e.cache != nil && e.cache.IsEnabled() {
		cacheKey, keyErr := e.generateRecommendationsCacheKey(resources)
		if keyErr == nil {
			resultData, marshalErr := json.Marshal(result)
			if marshalErr == nil {
				if setErr := e.cache.Set(cacheKey, json.RawMessage(resultData)); setErr != nil {
					log.Warn().
						Ctx(ctx).
						Str("component", "engine").
						Err(setErr).
						Msg("failed to store recommendations in cache")
				} else {
					log.Debug().
						Ctx(ctx).
						Str("component", "engine").
						Str("cache_key", cacheKey).
						Msg("stored recommendations in cache")
				}
			}
		}
	}

	return result, nil
}

// generateRecommendationsCacheKey generates a cache key for the given resources.
func (e *Engine) generateRecommendationsCacheKey(resources []ResourceDescriptor) (string, error) {
	// Extract resource types for key generation
	resourceTypes := make([]string, 0, len(resources))
	for _, r := range resources {
		resourceTypes = append(resourceTypes, r.Type)
	}

	// Build cache key using resource types and operation
	keyParams := cache.KeyParams{
		Operation:     "recommendations",
		Provider:      "multi", // Cross-provider recommendations
		ResourceTypes: resourceTypes,
	}

	key, err := cache.GenerateKey(keyParams)
	if err != nil {
		return "", fmt.Errorf("generating recommendations cache key: %w", err)
	}
	return key, nil
}

// fetchRecommendationsSequential fetches recommendations without batching (for small datasets).
func (e *Engine) fetchRecommendationsSequential(
	ctx context.Context,
	client *pluginhost.Client,
	resources []ResourceDescriptor,
	result *RecommendationsResult,
) error {
	log := logging.FromContext(ctx)

	targetResources := make([]*proto.ResourceDescriptor, 0, len(resources))
	for _, r := range resources {
		targetResources = append(targetResources, &proto.ResourceDescriptor{
			ID:         r.ID,
			Type:       r.Type,
			Provider:   r.Provider,
			Properties: convertToProto(r.Properties),
		})
	}

	req := &proto.GetRecommendationsRequest{
		TargetResources:  targetResources,
		ProjectionPeriod: "monthly",
	}

	resp, err := client.API.GetRecommendations(ctx, req)
	if err != nil {
		return err
	}

	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Str("plugin", client.Name).
		Int("recommendation_count", len(resp.Recommendations)).
		Msg("received recommendations from plugin")

	for _, rec := range resp.Recommendations {
		engineRec := convertProtoRecommendation(rec)
		if result.Currency == defaultCurrency && engineRec.Currency != "" {
			result.Currency = engineRec.Currency
		}
		result.TotalSavings += engineRec.EstimatedSavings
		result.Recommendations = append(result.Recommendations, engineRec)
	}

	return nil
}

// fetchRecommendationsWithBatching fetches recommendations using batch processing for large datasets.
func (e *Engine) fetchRecommendationsWithBatching(
	ctx context.Context,
	client *pluginhost.Client,
	resources []ResourceDescriptor,
	result *RecommendationsResult,
) error {
	log := logging.FromContext(ctx)

	// Create batch processor with threshold-sized batches
	processor, err := batch.NewProcessor[ResourceDescriptor](batchProcessingThreshold)
	if err != nil {
		return fmt.Errorf("failed to create batch processor: %w", err)
	}

	// Process resources in batches
	err = processor.Process(
		ctx,
		resources,
		func(ctx context.Context, batchResources []ResourceDescriptor, batchIndex int) error {
			// Convert batch to proto format
			targetResources := make([]*proto.ResourceDescriptor, 0, len(batchResources))
			for _, r := range batchResources {
				targetResources = append(targetResources, &proto.ResourceDescriptor{
					ID:         r.ID,
					Type:       r.Type,
					Provider:   r.Provider,
					Properties: convertToProto(r.Properties),
				})
			}

			// Fetch recommendations for this batch
			req := &proto.GetRecommendationsRequest{
				TargetResources:  targetResources,
				ProjectionPeriod: "monthly",
			}

			resp, recErr := client.API.GetRecommendations(ctx, req)
			if recErr != nil {
				log.Warn().
					Ctx(ctx).
					Str("component", "engine").
					Str("plugin", client.Name).
					Int("batch_index", batchIndex).
					Int("batch_size", len(batchResources)).
					Err(recErr).
					Msg("failed to fetch recommendations for batch")
				return fmt.Errorf("batch %d failed: %w", batchIndex, recErr)
			}

			log.Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("plugin", client.Name).
				Int("batch_index", batchIndex).
				Int("recommendation_count", len(resp.Recommendations)).
				Msg("received recommendations for batch")

			// Aggregate results (thread-safe append)
			for _, rec := range resp.Recommendations {
				engineRec := convertProtoRecommendation(rec)
				if result.Currency == defaultCurrency && engineRec.Currency != "" {
					result.Currency = engineRec.Currency
				}
				result.TotalSavings += engineRec.EstimatedSavings
				result.Recommendations = append(result.Recommendations, engineRec)
			}

			return nil
		},
	)

	if err != nil {
		return err
	}

	log.Info().
		Ctx(ctx).
		Str("component", "engine").
		Str("plugin", client.Name).
		Int("total_recommendations", len(result.Recommendations)).
		Msg("completed batch processing")

	return nil
}

// MergeRecommendations merges recommendations into CostResults by matching ResourceID.
func MergeRecommendations(
	costs []CostResult,
	recommendations []Recommendation,
) []CostResult {
	if len(recommendations) == 0 {
		return costs
	}

	// Index recommendations by ResourceID
	recMap := make(map[string][]Recommendation)
	for _, rec := range recommendations {
		if rec.ResourceID != "" {
			recMap[rec.ResourceID] = append(recMap[rec.ResourceID], rec)
		}
	}

	merged := make([]CostResult, len(costs))
	for i, cost := range costs {
		// Copy cost result
		res := cost
		if recs, ok := recMap[cost.ResourceID]; ok {
			res.Recommendations = recs
		}
		merged[i] = res
	}

	return merged
}
