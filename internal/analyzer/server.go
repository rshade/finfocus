package analyzer

import (
	"context"
	"strings"
	"sync"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/router"
)

const (
	// defaultVersion is used when no version is provided.
	defaultVersion = "0.0.0-dev"

	// analyzerDisplayName is the human-readable name for the analyzer.
	analyzerDisplayName = "FinFocus Analyzer"

	// analyzerDescription provides a description of the analyzer's purpose.
	analyzerDescription = "Provides real-time cost estimation for Pulumi infrastructure resources during preview operations."
)

// zeroCostResult returns an engine.CostResult representing zero cloud cost for an internal Pulumi resource.
// The result uses USD currency, sets monthly and hourly costs to 0, and includes a note indicating the resource is internal.
func zeroCostResult(resourceType, resourceID string) engine.CostResult {
	return engine.CostResult{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Currency:     "USD",
		Monthly:      0,
		Hourly:       0,
		Notes:        "Internal Pulumi resource (no cloud cost)",
	}
}

// CostCalculator is the interface for calculating projected costs.
//
// This interface abstracts the cost calculation engine, allowing for
// easier testing and decoupling from the concrete engine implementation.
type CostCalculator interface {
	GetProjectedCost(
		ctx context.Context,
		resources []engine.ResourceDescriptor,
	) ([]engine.CostResult, error)

	GetRecommendationsForResources(
		ctx context.Context,
		resources []engine.ResourceDescriptor,
	) (*engine.RecommendationsResult, error)
}

// Server implements the Pulumi Analyzer gRPC service for cost estimation.
//
// The Server processes resources during `pulumi preview` and returns
// cost diagnostics that appear in the Pulumi CLI output. It coordinates
// with the cost calculation engine to estimate resource costs and
// formats the results as AnalyzeDiagnostic messages.
//
// Key behaviors:
//   - All diagnostics use ADVISORY enforcement (never blocks deployments)
//   - Reports both per-resource costs and a stack-level summary
//   - Handles unsupported resources gracefully with informative messages
//   - Caches costs from Analyze() calls for accurate AnalyzeStack() summaries
type Server struct {
	pulumirpc.UnimplementedAnalyzerServer

	calculator CostCalculator
	version    string

	// Stack context from ConfigureStack RPC
	stackName    string
	projectName  string
	organization string
	dryRun       bool

	// Cost cache for accumulating costs from Analyze() calls
	// Used by AnalyzeStack() to generate accurate stack summaries
	costCacheMu sync.RWMutex
	costCache   map[string]engine.CostResult // resourceID -> CostResult

	// Configuration for threshold enforcement and summary file
	cfg *config.Config

	// summaryDir is the directory for writing cost summary files.
	// Set via WithSummaryDir(). If empty, summary file writing is skipped.
	summaryDir string

	// Cancellation support
	cancelMu sync.Mutex
	canceled bool
}

// NewServer creates a new Analyzer server with the given cost calculator.
//
// Parameters:
//   - calculator: The cost calculation engine to use for estimating costs
//   - version: The version string for this analyzer plugin
//
// NewServer creates a Server that uses the provided CostCalculator to estimate resource costs.
// If the provided version is empty, it defaults to "0.0.0-dev". The returned Server has its
// NewServer creates a Server configured with the provided CostCalculator and version.
// If the provided version is empty, NewServer uses defaultVersion. The server's internal
// per-resource cost cache is initialized.
// calculator is the CostCalculator implementation used for cost estimation.
// version is the plugin version string; pass an empty string to use the fallback default.
// It returns a pointer to the initialized Server.
func NewServer(calculator CostCalculator, version string) *Server {
	if version == "" {
		version = defaultVersion
	}
	return &Server{
		calculator: calculator,
		version:    version,
		costCache:  make(map[string]engine.CostResult),
	}
}

// WithConfig sets the configuration for threshold enforcement and summary file writing.
// This is a builder method that returns the Server for chaining.
// If cfg is nil, the server operates without threshold enforcement or summary file writing.
func (s *Server) WithConfig(cfg *config.Config) *Server {
	s.cfg = cfg
	return s
}

// WithSummaryDir sets the directory where the cost summary file will be written
// after each AnalyzeStack call. This is a builder method that returns the Server
// for chaining. If dir is empty, summary file writing is skipped.
func (s *Server) WithSummaryDir(dir string) *Server {
	s.summaryDir = dir
	return s
}

// cacheCost stores a cost result in the cache for later use by AnalyzeStack.
func (s *Server) cacheCost(resourceID string, cost engine.CostResult) {
	s.costCacheMu.Lock()
	defer s.costCacheMu.Unlock()
	s.costCache[resourceID] = cost
}

// getCachedCosts returns all cached costs as a slice.
func (s *Server) getCachedCosts() []engine.CostResult {
	s.costCacheMu.RLock()
	defer s.costCacheMu.RUnlock()
	costs := make([]engine.CostResult, 0, len(s.costCache))
	for _, cost := range s.costCache {
		costs = append(costs, cost)
	}
	return costs
}

// clearCostCache resets the cost cache (called at start of new stack analysis).
func (s *Server) clearCostCache() {
	s.costCacheMu.Lock()
	defer s.costCacheMu.Unlock()
	clear(s.costCache)
}

// Analyze analyzes a single resource and returns cost diagnostics.
//
// This method is called by the Pulumi engine for each resource as it is
// registered during preview. It receives the resource inputs before any
// mutations and returns diagnostics with cost estimates.
//
// All diagnostics use ADVISORY enforcement per FR-005.
func (s *Server) Analyze(
	ctx context.Context,
	req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	log := logging.FromContext(ctx)
	resourceType := req.GetType()
	resourceID := extractResourceID(req.GetUrn())

	// Handle internal Pulumi types (e.g., pulumi:pulumi:Stack, pulumi:providers:aws)
	// These have no cloud cost and should return $0.00
	if router.IsInternalPulumiType(resourceType) {
		cost := zeroCostResult(resourceType, resourceID)
		// Cache the cost for AnalyzeStack summary
		s.cacheCost(resourceID, cost)
		return &pulumirpc.AnalyzeResponse{
			Diagnostics: []*pulumirpc.AnalyzeDiagnostic{
				CostToDiagnostic(cost, req.GetUrn(), s.version),
			},
		}, nil
	}

	// Convert AnalyzeRequest to ResourceDescriptor
	resource := engine.ResourceDescriptor{
		Type:       resourceType,
		ID:         resourceID,
		Provider:   extractProviderFromRequest(req),
		Properties: structToMap(req.GetProperties()),
	}

	// Calculate costs using the engine
	costs, calcErr := s.calculator.GetProjectedCost(ctx, []engine.ResourceDescriptor{resource})
	if calcErr != nil {
		// Cache a zero-cost error result so AnalyzeStack can see that the resource
		// was attempted. Without this, failed resources are invisible to the stack summary.
		errResult := engine.CostResult{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Currency:     "USD",
			Notes:        "ERROR: " + calcErr.Error(),
		}
		s.cacheCost(resourceID, errResult)

		// Return a warning diagnostic if cost calculation fails.
		// We intentionally return nil error because we want to continue preview
		// with a warning diagnostic rather than failing the analysis.
		//nolint:nilerr // Intentional: return warning diagnostic instead of error
		return &pulumirpc.AnalyzeResponse{
			Diagnostics: []*pulumirpc.AnalyzeDiagnostic{
				WarningDiagnostic(
					"Cost calculation failed: "+calcErr.Error(),
					req.GetUrn(),
					s.version,
				),
			},
		}, nil
	}

	// Fetch recommendations (graceful failure - don't block costs if recs fail)
	recs, recErr := s.calculator.GetRecommendationsForResources(ctx, []engine.ResourceDescriptor{resource})
	if recErr == nil && recs != nil && len(recs.Recommendations) > 0 {
		log.Debug().
			Ctx(ctx).
			Str("component", "analyzer").
			Str("resource_id", resource.ID).
			Int("recommendation_count", len(recs.Recommendations)).
			Msg("received recommendations for resource")
		costs = engine.MergeRecommendations(costs, recs.Recommendations)
	} else if recErr != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "analyzer").
			Str("resource_id", resource.ID).
			Err(recErr).
			Msg("failed to fetch recommendations")
	}

	// Build diagnostics and cache costs for AnalyzeStack summary
	diagnostics := make([]*pulumirpc.AnalyzeDiagnostic, 0, len(costs))
	for _, cost := range costs {
		// Cache the cost for later use by AnalyzeStack
		s.cacheCost(cost.ResourceID, cost)
		diag := CostToDiagnostic(cost, req.GetUrn(), s.version)
		diagnostics = append(diagnostics, diag)
	}

	// If no costs returned, add a zero-cost diagnostic
	if len(diagnostics) == 0 {
		cost := engine.CostResult{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Currency:     "USD",
			Monthly:      0,
			Hourly:       0,
			Notes:        "No pricing information available",
		}
		// Cache even zero-cost results so they appear in the summary
		s.cacheCost(resourceID, cost)
		diagnostics = append(diagnostics, CostToDiagnostic(cost, req.GetUrn(), s.version))
	}

	return &pulumirpc.AnalyzeResponse{
		Diagnostics: diagnostics,
	}, nil
}

// AnalyzeStack analyzes all resources in a stack and returns cost diagnostics.
//
// This method is called by the Pulumi engine at the end of a successful
// preview or update. It receives the complete list of resources.
//
// Since Analyze() is called for each resource individually and already returns
// per-resource cost diagnostics, AnalyzeStack() only returns the stack-level
// summary to avoid duplicate diagnostics in the output.
//
// All diagnostics use ADVISORY enforcement per FR-005.
func (s *Server) AnalyzeStack(
	ctx context.Context,
	_ *pulumirpc.AnalyzeStackRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	log := logging.FromContext(ctx)

	// Use costs cached from individual Analyze() calls for accurate summary
	// This avoids re-querying plugins which may return different results
	// due to different property formats between AnalyzeRequest and AnalyzerResource
	cachedCosts := s.getCachedCosts()

	// Build cost summary once — used for diagnostics, threshold evaluation, and file output
	summary := BuildCostSummary(cachedCosts, s.stackName, s.projectName, time.Time{})

	// Build diagnostics list starting with the existing stack summary
	diagnostics := []*pulumirpc.AnalyzeDiagnostic{
		StackSummaryDiagnostic(cachedCosts, s.version),
	}

	// Threshold enforcement (FR-005, FR-006, FR-007)
	// Derive threshold inputs from the canonical CostSummary to avoid duplication
	if s.cfg != nil && s.cfg.Analyzer.MaxMonthlyCost > 0 {
		allFailed := len(cachedCosts) > 0 && summary.ResourceCount == 0

		switch {
		case allFailed:
			// FR-007 exception: all resources failed, total cost unknown — skip threshold
			log.Warn().
				Ctx(ctx).
				Str("component", "analyzer").
				Str("operation", "threshold_evaluation").
				Msg("all resource costs failed, skipping threshold evaluation")
		case summary.MixedCurrencies:
			// FR-013: mixed currencies — skip threshold enforcement
			log.Warn().
				Ctx(ctx).
				Str("component", "analyzer").
				Str("operation", "threshold_evaluation").
				Msg("mixed currencies detected, skipping threshold enforcement")
		default:
			thresholdDiag := ThresholdDiagnostic(
				summary.TotalMonthlyCost,
				s.cfg.Analyzer.MaxMonthlyCost,
				summary.Currency,
				s.version,
			)
			diagnostics = append(diagnostics, thresholdDiag)
		}
	}

	// Write cost summary file (graceful degradation: log warning, don't fail RPC)
	if s.summaryDir != "" {
		if writeErr := WriteCostSummary(summary, s.summaryDir); writeErr != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "analyzer").
				Str("operation", "write_cost_summary").
				Err(writeErr).
				Msg("failed to write cost summary file")
		}
	}

	// Clear the cost cache after the summary is built and emitted.
	// This prevents costs from leaking into subsequent stack analyses (e.g., re-runs).
	// Must happen AFTER reading the cache to ensure the summary reflects all Analyze() calls.
	s.clearCostCache()

	return &pulumirpc.AnalyzeResponse{
		Diagnostics: diagnostics,
	}, nil
}

// isErrorNote reports whether the provided notes string indicates an error by starting with
// the prefix "VALIDATION:" or "ERROR:". It returns true when the notes begin with either prefix, false otherwise.
func isErrorNote(notes string) bool {
	return strings.HasPrefix(notes, "VALIDATION:") || strings.HasPrefix(notes, "ERROR:")
}

// GetAnalyzerInfo returns metadata about this analyzer.
//
// This method provides information about the policies contained in this
// analyzer, including their enforcement levels and descriptions.
func (s *Server) GetAnalyzerInfo(
	_ context.Context,
	_ *emptypb.Empty,
) (*pulumirpc.AnalyzerInfo, error) {
	policies := []*pulumirpc.PolicyInfo{
		{
			Name:             policyNameCost,
			DisplayName:      "Cost Estimate",
			Description:      "Provides estimated monthly cost for individual resources",
			EnforcementLevel: pulumirpc.EnforcementLevel_ADVISORY,
		},
		{
			Name:             policyNameSum,
			DisplayName:      "Stack Cost Summary",
			Description:      "Provides total estimated monthly cost across all resources in the stack",
			EnforcementLevel: pulumirpc.EnforcementLevel_ADVISORY,
		},
	}

	// Register cost-threshold policy when threshold is configured.
	// EnforcementLevel is always ADVISORY per the Pulumi Analyzer contract — the
	// analyzer never blocks deployments. cfg.Analyzer.Enforcement is reserved for
	// external tooling (CLI exit codes, CI/CD gates) and is not applied here.
	if s.cfg != nil && s.cfg.Analyzer.MaxMonthlyCost > 0 {
		policies = append(policies, &pulumirpc.PolicyInfo{
			Name:             policyNameThreshold,
			DisplayName:      "Cost Threshold",
			Description:      "Enforces maximum monthly cost threshold for the stack",
			EnforcementLevel: pulumirpc.EnforcementLevel_ADVISORY,
		})
	}

	return &pulumirpc.AnalyzerInfo{
		Name:           policyPackName,
		DisplayName:    analyzerDisplayName,
		Version:        s.version,
		Description:    analyzerDescription,
		Policies:       policies,
		SupportsConfig: false,
	}, nil
}

// GetPluginInfo returns generic information about this plugin.
//
// This method returns the plugin version which is used by the Pulumi
// engine for version compatibility checks.
func (s *Server) GetPluginInfo(
	_ context.Context,
	_ *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: s.version,
	}, nil
}

// Handshake establishes connection with the Pulumi engine.
//
// This method is called immediately after the plugin starts. It receives
// the engine's gRPC address and directory information. For the cost
// analyzer, we simply acknowledge the handshake as we don't need to
// make callbacks to the engine.
func (s *Server) Handshake(
	_ context.Context,
	_ *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	// Store engine address if needed for future callbacks (not used in MVP)
	// The handshake is primarily informational - we just acknowledge it
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

// ConfigureStack receives stack context before analysis begins.
//
// This method is called before AnalyzeStack to provide context about
// the stack being analyzed. The information is stored for logging
// and diagnostic purposes.
func (s *Server) ConfigureStack(
	_ context.Context,
	req *pulumirpc.AnalyzerStackConfigureRequest,
) (*pulumirpc.AnalyzerStackConfigureResponse, error) {
	// Store stack context for logging and diagnostic enrichment
	s.stackName = req.GetStack()
	s.projectName = req.GetProject()
	s.organization = req.GetOrganization()
	s.dryRun = req.GetDryRun()

	return &pulumirpc.AnalyzerStackConfigureResponse{}, nil
}

// Cancel signals graceful shutdown of the analyzer.
//
// This method is called when the Pulumi engine is shutting down or
// when the user cancels the operation. It sets the canceled flag
// which can be checked by long-running operations.
func (s *Server) Cancel(
	_ context.Context,
	_ *emptypb.Empty,
) (*emptypb.Empty, error) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.canceled = true
	return &emptypb.Empty{}, nil
}

// IsCanceled returns true if the Cancel RPC has been called.
//
// This method is thread-safe and can be called concurrently.
func (s *Server) IsCanceled() bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return s.canceled
}
