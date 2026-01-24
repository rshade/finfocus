// Package plugin provides a configurable mock plugin for testing FinFocus's plugin communication.
// The mock plugin implements the CostSourceService gRPC interface with support for:
// - Configurable responses for different resource types
// - Error injection for testing failure scenarios
// - Performance simulation for load testing
// - State reset between tests for isolation
package plugin

import (
	"errors"
	"sync"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/proto"
)

// MockConfig defines configuration for mock plugin behavior.
type MockConfig struct {
	// ProjectedCostResponses maps resource types to their configured cost responses
	ProjectedCostResponses map[string]*proto.CostResult

	// ActualCostResponses maps resource IDs to their configured actual cost responses
	ActualCostResponses map[string]*proto.ActualCostResult

	// BudgetsResponse configures the response for GetBudgets call
	BudgetsResponse []*pbc.Budget

	// ErrorType specifies which error to inject (if any)
	ErrorType ErrorType

	// ErrorMethod specifies which method should return an error
	ErrorMethod string

	// LatencyMS specifies simulated latency in milliseconds
	LatencyMS int

	// PluginVersion specifies the plugin version to return in GetPluginInfo
	PluginVersion string

	// PluginSpecVersion specifies the spec version to return in GetPluginInfo
	PluginSpecVersion string
}

// ErrorType represents different types of errors the mock can simulate.
type ErrorType string

const (
	// ErrorNone indicates no error should be returned.
	ErrorNone ErrorType = ""

	// ErrorTimeout simulates a timeout error.
	ErrorTimeout ErrorType = "timeout"

	// ErrorProtocol simulates a protocol/gRPC error.
	ErrorProtocol ErrorType = "protocol"

	// ErrorInvalidData simulates invalid data from the plugin.
	ErrorInvalidData ErrorType = "invalid_data"

	// ErrorUnavailable simulates the service being unavailable.
	ErrorUnavailable ErrorType = "unavailable"
)

var (
	// ErrMockTimeout is returned when timeout error is configured.
	ErrMockTimeout = errors.New("mock plugin: simulated timeout")

	// ErrMockProtocol is returned when protocol error is configured.
	ErrMockProtocol = errors.New("mock plugin: simulated protocol error")

	// ErrMockInvalidData is returned when invalid data error is configured.
	ErrMockInvalidData = errors.New("mock plugin: simulated invalid data")

	// ErrMockUnavailable is returned when unavailable error is configured.
	ErrMockUnavailable = errors.New("mock plugin: service unavailable")

	// ErrMockNotConfigured is returned when no response is configured for a request.
	ErrMockNotConfigured = errors.New("mock plugin: no response configured for resource")
)

// MockPlugin represents a configurable mock plugin server.
type MockPlugin struct {
	config MockConfig
	mu     sync.RWMutex
}

// NewMockPlugin creates a new mock plugin with default configuration.
func NewMockPlugin() *MockPlugin {
	return &MockPlugin{
		config: MockConfig{
			ProjectedCostResponses: make(map[string]*proto.CostResult),
			ActualCostResponses:    make(map[string]*proto.ActualCostResult),
			BudgetsResponse:        nil,
			ErrorType:              ErrorNone,
			ErrorMethod:            "",
			LatencyMS:              0,
		},
	}
}

// Configure sets the mock plugin configuration.
// This method can be called multiple times to update configuration.
func (m *MockPlugin) Configure(config MockConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// SetProjectedCostResponse configures a response for a specific resource type.
// This is a convenience method for setting individual responses.
func (m *MockPlugin) SetProjectedCostResponse(resourceType string, response *proto.CostResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config.ProjectedCostResponses == nil {
		m.config.ProjectedCostResponses = make(map[string]*proto.CostResult)
	}
	m.config.ProjectedCostResponses[resourceType] = response
}

// SetActualCostResponse configures a response for a specific resource ID.
// This is a convenience method for setting individual responses.
func (m *MockPlugin) SetActualCostResponse(resourceID string, response *proto.ActualCostResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config.ActualCostResponses == nil {
		m.config.ActualCostResponses = make(map[string]*proto.ActualCostResult)
	}
	m.config.ActualCostResponses[resourceID] = response
}

// SetBudgetsResponse configures the response for GetBudgets.
func (m *MockPlugin) SetBudgetsResponse(budgets []*pbc.Budget) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.BudgetsResponse = budgets
}

// SetError configures the mock to return an error for a specific method.
// methodName should be "GetProjectedCost" or "GetActualCost".
// Set errorType to ErrorNone to clear error injection.
func (m *MockPlugin) SetError(methodName string, errorType ErrorType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.ErrorMethod = methodName
	m.config.ErrorType = errorType
}

// SetLatency configures simulated latency in milliseconds.
// Set to 0 to disable latency simulation.
func (m *MockPlugin) SetLatency(latencyMS int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.LatencyMS = latencyMS
}

// Reset clears all configuration and returns the mock to its default state.
// This should be called between tests to ensure isolation.
func (m *MockPlugin) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = MockConfig{
		ProjectedCostResponses: make(map[string]*proto.CostResult),
		ActualCostResponses:    make(map[string]*proto.ActualCostResult),
		BudgetsResponse:        nil,
		ErrorType:              ErrorNone,
		ErrorMethod:            "",
		LatencyMS:              0,
	}
}

// GetConfig returns the current mock configuration (for testing/debugging).
func (m *MockPlugin) GetConfig() MockConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetLatency returns the configured latency.
func (m *MockPlugin) GetLatency() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.LatencyMS
}

// GetProjectedResponse returns the configured response for a resource type.
func (m *MockPlugin) GetProjectedResponse(resourceType string) (*proto.CostResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	resp, ok := m.config.ProjectedCostResponses[resourceType]
	return resp, ok
}

// GetActualResponse returns the configured response for a resource ID.
func (m *MockPlugin) GetActualResponse(resourceID string) (*proto.ActualCostResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	resp, ok := m.config.ActualCostResponses[resourceID]
	return resp, ok
}

// GetBudgetsResponse returns the configured budgets.
func (m *MockPlugin) GetBudgetsResponse() []*pbc.Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.BudgetsResponse
}

// ShouldInjectError determines if an error should be injected for the given method.
func (m *MockPlugin) ShouldInjectError(methodName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.ErrorMethod != methodName || m.config.ErrorType == ErrorNone {
		return nil
	}

	switch m.config.ErrorType { //nolint:exhaustive // ErrorNone handled by early return guard above
	case ErrorTimeout:
		return ErrMockTimeout
	case ErrorProtocol:
		return ErrMockProtocol
	case ErrorInvalidData:
		return ErrMockInvalidData
	case ErrorUnavailable:
		return ErrMockUnavailable
	default:
		return nil
	}
}
