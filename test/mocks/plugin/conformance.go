package plugin

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// ConformancePlugin is a reference implementation that correctly implements
// all protocol requirements. It's used as a baseline for conformance testing.
type ConformancePlugin struct {
	pbc.UnimplementedCostSourceServiceServer

	// mu protects concurrent access to mutable fields.
	mu sync.RWMutex

	// Name is the plugin name returned by the Name RPC.
	PluginName string
	// Version is the plugin version.
	PluginVersion string
	// ProtocolVersion is the protocol version this plugin implements.
	PluginProtocolVersion string

	// SupportedResourceTypes lists resource types this plugin can price.
	SupportedResourceTypes []string

	// SimulateLatency adds artificial latency to responses (for timeout testing).
	SimulateLatency time.Duration

	// FailOnMethod causes the plugin to return an error for specific methods.
	FailOnMethod map[string]error
}

// NewConformancePlugin creates a new reference conformance plugin.
func NewConformancePlugin() *ConformancePlugin {
	return &ConformancePlugin{
		PluginName:            "conformance-reference",
		PluginVersion:         "1.0.0",
		PluginProtocolVersion: "1.0",
		SupportedResourceTypes: []string{
			"aws:ec2/instance:Instance",
			"aws:s3/bucket:Bucket",
			"aws:rds/instance:Instance",
			"aws:lambda/function:Function",
		},
		FailOnMethod: make(map[string]error),
	}
}

// Name implements the Name RPC method per the protocol specification.
func (p *ConformancePlugin) Name(ctx context.Context, _ *pbc.NameRequest) (*pbc.NameResponse, error) {
	if err := p.checkMethodError("Name"); err != nil {
		return nil, err
	}

	if err := p.simulateLatency(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	name := p.PluginName
	p.mu.RUnlock()

	return &pbc.NameResponse{
		Name: name,
	}, nil
}

// GetProjectedCost implements the GetProjectedCost RPC method per the protocol specification.
func (p *ConformancePlugin) GetProjectedCost(
	ctx context.Context,
	req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	// Validate request first, before method error/latency simulation
	if req == nil || req.GetResource() == nil {
		return nil, status.Error(codes.InvalidArgument, "resource is required")
	}

	resourceType := req.GetResource().GetResourceType()
	if resourceType == "" {
		return nil, status.Error(codes.InvalidArgument, "resource_type is required")
	}

	if err := p.checkMethodError("GetProjectedCost"); err != nil {
		return nil, err
	}

	// Check for magic resource types that trigger specific error codes for conformance testing
	switch resourceType {
	case "forbidden:resource":
		return nil, status.Error(codes.PermissionDenied, "permission denied for this resource")
	case "error:internal":
		return nil, status.Error(codes.Internal, "simulated internal error")
	case "error:unavailable":
		return nil, status.Error(codes.Unavailable, "service unavailable")
	}

	// Check if resource type is supported
	if !p.supportsResourceType(resourceType) {
		return nil, status.Error(codes.NotFound, "unsupported resource type: "+resourceType)
	}

	// Return a mock cost response
	return &pbc.GetProjectedCostResponse{
		Currency:      "USD",
		CostPerMonth:  100.0,
		UnitPrice:     0.137,
		BillingDetail: "Mock cost for " + resourceType,
	}, nil
}

// GetActualCost implements the GetActualCost RPC method per the protocol specification.
func (p *ConformancePlugin) GetActualCost(
	ctx context.Context,
	req *pbc.GetActualCostRequest,
) (*pbc.GetActualCostResponse, error) {
	if err := p.checkMethodError("GetActualCost"); err != nil {
		return nil, err
	}

	if err := p.simulateLatency(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	resourceID := req.GetResourceId()
	if resourceID == "" {
		return nil, status.Error(codes.InvalidArgument, "resource_id is required")
	}

	// Return a mock actual cost response
	return &pbc.GetActualCostResponse{
		Results: []*pbc.ActualCostResult{
			{
				Source: "mock",
				Cost:   50.0,
			},
		},
	}, nil
}

// supportsResourceType checks if the plugin supports a given resource type.
func (p *ConformancePlugin) supportsResourceType(resourceType string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, supported := range p.SupportedResourceTypes {
		if supported == resourceType {
			return true
		}
	}
	return false
}

// checkMethodError returns an error if one is configured for the given method.
func (p *ConformancePlugin) checkMethodError(method string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if err, ok := p.FailOnMethod[method]; ok && err != nil {
		return err
	}
	return nil
}

// simulateLatency adds artificial delay if configured, respecting context cancellation.
func (p *ConformancePlugin) simulateLatency(ctx context.Context) error {
	p.mu.RLock()
	latency := p.SimulateLatency
	p.mu.RUnlock()

	if latency <= 0 {
		return nil
	}

	timer := time.NewTimer(latency)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		// Preserve whether this was a cancel vs deadline from the caller.
		return status.Error(codes.DeadlineExceeded, ctx.Err().Error())
	}
}

// SetFailure configures the plugin to fail on a specific method.
func (p *ConformancePlugin) SetFailure(method string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FailOnMethod[method] = err
}

// ClearFailure removes the failure configuration for a method.
func (p *ConformancePlugin) ClearFailure(method string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.FailOnMethod, method)
}

// ClearAllFailures removes all failure configurations.
func (p *ConformancePlugin) ClearAllFailures() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FailOnMethod = make(map[string]error)
}

// AddSupportedResourceType adds a resource type to the supported list.
func (p *ConformancePlugin) AddSupportedResourceType(resourceType string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.SupportedResourceTypes = append(p.SupportedResourceTypes, resourceType)
}

// Reset restores the plugin to its default state.
func (p *ConformancePlugin) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.PluginName = "conformance-reference"
	p.PluginVersion = "1.0.0"
	p.PluginProtocolVersion = "1.0"
	p.SupportedResourceTypes = []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
		"aws:rds/instance:Instance",
		"aws:lambda/function:Function",
	}
	p.SimulateLatency = 0
	p.FailOnMethod = make(map[string]error)
}
