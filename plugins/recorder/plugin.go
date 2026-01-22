package recorder

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	maxPageSize = 1000
)

// RecorderPlugin implements the CostSourceService interface.
// It records all incoming gRPC requests to JSON files and optionally
// returns mock cost responses for testing purposes.
//
// This plugin serves as a reference implementation demonstrating
// pluginsdk v0.4.6 patterns including:
//   - BasePlugin embedding for common functionality
//   - Request validation using SDK helpers
//   - Graceful shutdown handling
//   - Thread-safe operation
//
//nolint:revive // RecorderPlugin naming is intentional for clarity in external usage
type RecorderPlugin struct {
	*pluginsdk.BasePlugin

	config   *Config
	recorder *Recorder
	mocker   *Mocker
	logger   zerolog.Logger
	mu       sync.Mutex
}

// NewRecorderPlugin creates a new recorder plugin instance.
// The plugin is configured via the provided Config and will:
//   - Record all requests to JSON files in Config.OutputDir
//   - Return mock responses if Config.MockResponse is true
//   - Return zero/empty costs if Config.MockResponse is false
func NewRecorderPlugin(cfg *Config, logger zerolog.Logger) *RecorderPlugin {
	base := pluginsdk.NewBasePlugin("recorder")

	p := &RecorderPlugin{
		BasePlugin: base,
		config:     cfg,
		logger:     logger.With().Str("component", "recorder-plugin").Logger(),
	}

	// Initialize recorder for capturing requests
	p.recorder = NewRecorder(cfg.OutputDir, p.logger)

	// Initialize mocker only if mock responses are enabled
	if cfg.MockResponse {
		p.mocker = NewMocker(p.logger)
		p.logger.Info().Msg("mock response mode enabled")
	}

	p.logger.Info().
		Str("output_dir", cfg.OutputDir).
		Bool("mock_response", cfg.MockResponse).
		Msg("recorder plugin initialized")

	return p
}

// Name returns the plugin identifier.
// This method satisfies the pluginsdk.Plugin interface.
func (p *RecorderPlugin) Name() string {
	return "recorder"
}

// GetProjectedCost handles projected cost requests.
// It validates the request, records it to disk, and returns either:
//   - Mock cost data (if MockResponse is enabled)
//   - Zero cost with explanatory note (if MockResponse is disabled)
func (p *RecorderPlugin) GetProjectedCost(
	_ context.Context, req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate request using pluginsdk v0.4.6 helpers
	if err := pluginsdk.ValidateProjectedCostRequest(req); err != nil {
		p.logger.Warn().Err(err).Msg("invalid GetProjectedCost request")
		// Record invalid request for debugging purposes
		_ = p.recorder.RecordRequest("GetProjectedCost", req)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Record the request to disk
	if err := p.recorder.RecordRequest("GetProjectedCost", req); err != nil {
		// Log warning but continue - recording failure shouldn't fail the request
		p.logger.Warn().Err(err).Msg("failed to record request")
	}

	// Return mock or zero response
	if p.mocker != nil {
		return p.mocker.CreateProjectedCostResponse(), nil
	}

	// Return zero cost when mock mode is disabled
	return &pbc.GetProjectedCostResponse{
		CostPerMonth:  0.0,
		UnitPrice:     0.0,
		Currency:      "USD",
		BillingDetail: "Recorder plugin - mock responses disabled",
	}, nil
}

// GetActualCost handles actual cost requests.
// It validates the request, records it to disk, and returns either:
//   - Mock cost data (if MockResponse is enabled)
//   - Empty results (if MockResponse is disabled)
func (p *RecorderPlugin) GetActualCost(
	_ context.Context, req *pbc.GetActualCostRequest,
) (*pbc.GetActualCostResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate request using pluginsdk v0.4.6 helpers
	if err := pluginsdk.ValidateActualCostRequest(req); err != nil {
		p.logger.Warn().Err(err).Msg("invalid GetActualCost request")
		// Record invalid request for debugging purposes
		_ = p.recorder.RecordRequest("GetActualCost", req)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Record the request to disk
	if err := p.recorder.RecordRequest("GetActualCost", req); err != nil {
		// Log warning but continue - recording failure shouldn't fail the request
		p.logger.Warn().Err(err).Msg("failed to record request")
	}

	// Return mock or empty response
	if p.mocker != nil {
		return p.mocker.CreateActualCostResponse(), nil
	}

	// Return empty results when mock mode is disabled
	return &pbc.GetActualCostResponse{
		Results: []*pbc.ActualCostResult{},
	}, nil
}

// GetRecommendations handles recommendations requests.
func (p *RecorderPlugin) GetRecommendations(
	_ context.Context, req *pbc.GetRecommendationsRequest,
) (*pbc.GetRecommendationsResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate request
	if req == nil {
		p.logger.Warn().Msg("GetRecommendations request is nil")
		return nil, status.Errorf(codes.InvalidArgument, "request is required")
	}

	// Validate PageSize bounds if set
	if req.GetPageSize() < 0 {
		p.logger.Warn().
			Int32("page_size", req.GetPageSize()).
			Msg("invalid page_size in GetRecommendations request")
		_ = p.recorder.RecordRequest("GetRecommendations", req)
		return nil, status.Errorf(codes.InvalidArgument, "page_size must be non-negative")
	}
	if req.GetPageSize() > maxPageSize {
		p.logger.Warn().
			Int32("page_size", req.GetPageSize()).
			Msg("page_size exceeds maximum in GetRecommendations request")
		_ = p.recorder.RecordRequest("GetRecommendations", req)
		return nil, status.Errorf(codes.InvalidArgument, "page_size must not exceed %d", maxPageSize)
	}

	// Record the request to disk
	if err := p.recorder.RecordRequest("GetRecommendations", req); err != nil {
		p.logger.Warn().Err(err).Msg("failed to record request")
	}

	// Return mock or empty response
	if p.mocker != nil {
		resp := p.mocker.CreateRecommendationsResponse()
		recs := resp.GetRecommendations()

		// Apply pagination
		limit := int(req.GetPageSize())
		if limit == 0 {
			limit = len(recs) // Treat 0 as no-limit
		}

		// Slice to the minimum of limit and actual length
		if limit < len(recs) {
			recs = recs[:limit]
		}

		return &pbc.GetRecommendationsResponse{
			Recommendations: recs,
		}, nil
	}

	return &pbc.GetRecommendationsResponse{
		Recommendations: []*pbc.Recommendation{},
	}, nil
}

// GetPluginInfo returns information about the plugin.
func (p *RecorderPlugin) GetPluginInfo(
	_ context.Context, req *pbc.GetPluginInfoRequest,
) (*pbc.GetPluginInfoResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate request is non-nil
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request is required")
	}

	// Record the request to disk
	if err := p.recorder.RecordRequest("GetPluginInfo", req); err != nil {
		p.logger.Warn().Err(err).Msg("failed to record request")
	}

	return &pbc.GetPluginInfoResponse{
		Name:        "recorder",
		Version:     "0.1.0",
		SpecVersion: "0.5.2",
		Providers: []string{
			"test",
		},
		Capabilities: []pbc.PluginCapability{
			pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
			pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS,
			pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS,
		},
	}, nil
}

// Shutdown performs graceful shutdown of the plugin.
// It flushes any pending writes and releases resources.
func (p *RecorderPlugin) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info().Msg("shutting down recorder plugin")

	if p.recorder != nil {
		p.recorder.Close()
	}
}
