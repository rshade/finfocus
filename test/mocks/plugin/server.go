package plugin

import (
	"context"
	"errors"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockServer implements the CostSourceServiceServer interface for testing.
type mockServer struct {
	pbc.UnimplementedCostSourceServiceServer

	plugin *MockPlugin
}

// newMockServer creates a new mock gRPC server wrapping the given MockPlugin.
func newMockServer(plugin *MockPlugin) *mockServer {
	return &mockServer{plugin: plugin}
}

// Name implements the Name RPC method.
func (s *mockServer) Name(_ context.Context, _ *pbc.NameRequest) (*pbc.NameResponse, error) {
	// Simulate latency if configured
	latency := s.plugin.GetLatency()
	if latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	// Check for error injection
	if err := s.plugin.ShouldInjectError("Name"); err != nil {
		return nil, toGRPCError(err)
	}

	return &pbc.NameResponse{
		Name: "mock-plugin",
	}, nil
}

// GetPluginInfo implements the GetPluginInfo RPC method.
func (s *mockServer) GetPluginInfo(_ context.Context, _ *pbc.GetPluginInfoRequest) (*pbc.GetPluginInfoResponse, error) {
	// Simulate latency if configured
	latency := s.plugin.GetLatency()
	if latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	// Check for error injection
	if err := s.plugin.ShouldInjectError("GetPluginInfo"); err != nil {
		return nil, toGRPCError(err)
	}

	config := s.plugin.GetConfig()
	return &pbc.GetPluginInfoResponse{
		Version:     config.PluginVersion,
		SpecVersion: config.PluginSpecVersion,
	}, nil
}

// GetProjectedCost implements the GetProjectedCost RPC method.
func (s *mockServer) GetProjectedCost(
	_ context.Context,
	req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	// Simulate latency if configured
	latency := s.plugin.GetLatency()
	if latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	// Check for error injection
	if err := s.plugin.ShouldInjectError("GetProjectedCost"); err != nil {
		return nil, toGRPCError(err)
	}

	// Look up configured response for this resource type
	resourceType := req.GetResource().GetResourceType()
	configuredResult, found := s.plugin.GetProjectedResponse(resourceType)

	if !found {
		// Return error if no response configured
		return nil, status.Error(codes.NotFound, ErrMockNotConfigured.Error())
	}

	// Convert internal CostResult to proto response
	response := &pbc.GetProjectedCostResponse{
		Currency:      configuredResult.Currency,
		CostPerMonth:  configuredResult.MonthlyCost,
		UnitPrice:     configuredResult.HourlyCost,
		BillingDetail: configuredResult.Notes,
	}

	return response, nil
}

// GetActualCost implements the GetActualCost RPC method.
func (s *mockServer) GetActualCost(
	_ context.Context,
	req *pbc.GetActualCostRequest,
) (*pbc.GetActualCostResponse, error) {
	// Simulate latency if configured
	latency := s.plugin.GetLatency()
	if latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	// Check for error injection
	if err := s.plugin.ShouldInjectError("GetActualCost"); err != nil {
		return nil, toGRPCError(err)
	}

	// Look up configured response for this resource ID
	resourceID := req.GetResourceId()
	configuredResult, found := s.plugin.GetActualResponse(resourceID)

	if !found {
		// Return error if no response configured
		return nil, status.Error(codes.NotFound, ErrMockNotConfigured.Error())
	}

	// Convert internal ActualCostResult to proto response
	var results []*pbc.ActualCostResult
	for source, cost := range configuredResult.CostBreakdown {
		results = append(results, &pbc.ActualCostResult{
			Source: source,
			Cost:   cost,
		})
	}

	// If no breakdown, create single entry with total
	if len(results) == 0 {
		results = append(results, &pbc.ActualCostResult{
			Source: "total",
			Cost:   configuredResult.TotalCost,
		})
	}

	response := &pbc.GetActualCostResponse{
		Results: results,
	}

	return response, nil
}

// GetBudgets implements the GetBudgets RPC method.
func (s *mockServer) GetBudgets(
	_ context.Context,
	_ *pbc.GetBudgetsRequest,
) (*pbc.GetBudgetsResponse, error) {
	// Simulate latency if configured
	latency := s.plugin.GetLatency()
	if latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	// Check for error injection
	if err := s.plugin.ShouldInjectError("GetBudgets"); err != nil {
		return nil, toGRPCError(err)
	}

	budgets := s.plugin.GetBudgetsResponse()
	// Return configured budgets (or nil/empty if none)
	return &pbc.GetBudgetsResponse{
		Budgets: budgets,
	}, nil
}

// RegisterServer registers the mock server with a gRPC server instance.
func (s *mockServer) RegisterServer(grpcServer *grpc.Server) {
	pbc.RegisterCostSourceServiceServer(grpcServer, s)
}

// toGRPCError converts mock error types to appropriate gRPC status codes.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, ErrMockTimeout):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, ErrMockProtocol):
		return status.Error(codes.Internal, err.Error())
	case errors.Is(err, ErrMockInvalidData):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrMockUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, ErrMockNotConfigured):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Unknown, err.Error())
	}
}

// ConvertTimestamp converts int64 unix timestamp to protobuf timestamp (helper for tests).
func ConvertTimestamp(unixTime int64) *timestamppb.Timestamp {
	return timestamppb.New(time.Unix(unixTime, 0))
}
