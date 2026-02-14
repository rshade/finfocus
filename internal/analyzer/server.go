package analyzer

import (
	"context"
	"sync"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"

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
// zeroCostResult constructs an engine.CostResult representing zero cloud cost for an internal Pulumi resource.
// resourceType is the resource's provider type (for example, "pulumi:pulumi:Stack"); resourceID is the resource's identifier.
// The returned CostResult uses USD currency, sets Monthly and Hourly to 0, and includes a note indicating the resource has no cloud cost.
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
// version set and its internal cost cache initialized.
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
	_ context.Context,
	_ *pulumirpc.AnalyzeStackRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	// Use costs cached from individual Analyze() calls for accurate summary
	// This avoids re-querying plugins which may return different results
	// due to different property formats between AnalyzeRequest and AnalyzerResource
	cachedCosts := s.getCachedCosts()

	// Only return the stack summary diagnostic
	// Per-resource diagnostics are already returned by Analyze() calls
	summary := StackSummaryDiagnostic(cachedCosts, s.version)

	return &pulumirpc.AnalyzeResponse{
		Diagnostics: []*pulumirpc.AnalyzeDiagnostic{summary},
	}, nil
}

// GetAnalyzerInfo returns metadata about this analyzer.
//
// This method provides information about the policies contained in this
// analyzer, including their enforcement levels and descriptions.
func (s *Server) GetAnalyzerInfo(
	_ context.Context,
	_ *emptypb.Empty,
) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{
		Name:        policyPackName,
		DisplayName: analyzerDisplayName,
		Version:     s.version,
		Description: analyzerDescription,
		Policies: []*pulumirpc.PolicyInfo{
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
		},
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
	// Clear the cost cache at the start of a new stack analysis
	// This ensures we don't carry over costs from previous analyses
	s.clearCostCache()

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