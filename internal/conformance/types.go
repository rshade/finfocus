package conformance

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// Status represents the outcome of a conformance test.
type Status string

const (
	// StatusPass indicates the test assertions passed.
	StatusPass Status = "pass"
	// StatusFail indicates the test assertions failed.
	StatusFail Status = "fail"
	// StatusSkip indicates the test was skipped (precondition not met).
	StatusSkip Status = "skip"
	// StatusError indicates an infrastructure error (plugin crash, timeout, connection lost).
	StatusError Status = "error"
)

// Category represents a test grouping for filtering.
type Category string

const (
	// CategoryProtocol tests basic protocol compliance (Name, GetProjectedCost, etc.).
	CategoryProtocol Category = "protocol"
	// CategoryPerformance tests timeout behavior and batch handling.
	CategoryPerformance Category = "performance"
	// CategoryError tests error handling and gRPC status codes.
	CategoryError Category = "error"
	// CategoryContext tests context cancellation and deadline propagation.
	CategoryContext Category = "context"
)

// AllCategories returns all available test categories.
func AllCategories() []Category {
	return []Category{
		CategoryProtocol,
		CategoryPerformance,
		CategoryError,
		CategoryContext,
	}
}

// IsValidCategory checks if a category string is valid.
func IsValidCategory(cat string) bool {
	switch Category(cat) {
	case CategoryProtocol, CategoryPerformance, CategoryError, CategoryContext:
		return true
	default:
		return false
	}
}

// Verbosity represents the logging detail level.
type Verbosity string

const (
	// VerbosityQuiet shows pass/fail summary only.
	VerbosityQuiet Verbosity = "quiet"
	// VerbosityNormal shows test names and results (default).
	VerbosityNormal Verbosity = "normal"
	// VerbosityVerbose shows request/response payloads.
	VerbosityVerbose Verbosity = "verbose"
	// VerbosityDebug shows timing, connection state, retry details.
	VerbosityDebug Verbosity = "debug"
)

// IsValidVerbosity checks if a verbosity string is valid.
func IsValidVerbosity(v string) bool {
	switch Verbosity(v) {
	case VerbosityQuiet, VerbosityNormal, VerbosityVerbose, VerbosityDebug:
		return true
	default:
		return false
	}
}

// CommMode represents the plugin communication mode.
type CommMode string

const (
	// CommModeTCP uses TCP socket communication (default).
	CommModeTCP CommMode = "tcp"
	// CommModeStdio uses stdin/stdout communication.
	CommModeStdio CommMode = "stdio"
)

// IsValidCommMode checks if a communication mode string is valid.
func IsValidCommMode(mode string) bool {
	switch CommMode(mode) {
	case CommModeTCP, CommModeStdio:
		return true
	default:
		return false
	}
}

// TestCase represents a single protocol compliance check.
type TestCase struct {
	// Name is the unique test identifier (e.g., "Name_ReturnsPluginIdentifier").
	Name string
	// Category is the test category: protocol, performance, error, context.
	Category Category
	// Description is a human-readable test description.
	Description string
	// Timeout is the maximum time for this test (default: 10s).
	Timeout time.Duration
	// RequiredMethods lists gRPC methods this test validates.
	RequiredMethods []string
	// TestFunc is the function that executes the test.
	TestFunc TestFunc
}

// TestFunc is the signature for test execution functions.
// It receives a TestContext and returns a TestResult.
type TestFunc func(ctx *TestContext) *TestResult

// ConnectFunc is a factory function for connecting to a plugin.
// It returns a gRPC client, a function to close the connection/process, and any error.
type ConnectFunc func(ctx context.Context) (pbc any, closeFn func() error, err error)

// TestContext provides context for test execution.
type TestContext struct {
	// PluginClient is the gRPC client connected to the plugin under test.
	PluginClient any
	// Logger is the configured logger for this test run.
	Logger zerolog.Logger
	// Verbosity is the configured verbosity level.
	Verbosity Verbosity
	// Timeout is the timeout for this specific test.
	Timeout time.Duration
}

// TestResult represents the outcome of running a single test.
type TestResult struct {
	// TestName references the TestCase.Name.
	TestName string `json:"name"`
	// Category is the test category.
	Category Category `json:"category"`
	// Status is the test outcome: pass, fail, skip, error.
	Status Status `json:"status"`
	// Duration is the actual execution time (serialized as nanoseconds).
	Duration time.Duration `json:"duration_ns"`
	// Error is the error message if Status != pass.
	Error string `json:"error,omitempty"`
	// Details provides additional context (request/response logs).
	Details string `json:"details,omitempty"`
	// Timestamp is when the test completed.
	Timestamp time.Time `json:"timestamp"`
}

// PluginUnderTest represents the plugin binary being validated.
type PluginUnderTest struct {
	// Path is the absolute path to plugin binary.
	Path string `json:"path"`
	// Name is the plugin name (from Name() RPC).
	Name string `json:"name"`
	// Version is the plugin version (from Name() RPC).
	Version string `json:"version"`
	// ProtocolVersion is the protocol version implemented.
	ProtocolVersion string `json:"protocol_version"`
	// CommMode is the communication mode: "tcp" or "stdio".
	CommMode CommMode `json:"comm_mode"`
}

// SuiteConfig is the configuration for running the conformance suite.
type SuiteConfig struct {
	// PluginPath is the path to plugin binary (required).
	PluginPath string
	// CommMode is "tcp" (default) or "stdio".
	CommMode CommMode
	// Verbosity is the logging level (default: normal).
	Verbosity Verbosity
	// OutputFormat is the output format: table, json, junit.
	OutputFormat string
	// OutputPath is the path for output file (optional, defaults to stdout).
	OutputPath string
	// Timeout is the global timeout for entire suite (default: 5m).
	Timeout time.Duration
	// Categories filters to specific test categories.
	Categories []Category
	// TestFilter is a regex filter for test names.
	TestFilter string
	// Logger is the custom logger (optional).
	Logger zerolog.Logger
}

// Summary contains aggregate test counts.
type Summary struct {
	// Total is the total tests executed.
	Total int `json:"total"`
	// Passed is the count of tests with pass status.
	Passed int `json:"passed"`
	// Failed is the count of tests with fail status.
	Failed int `json:"failed"`
	// Skipped is the count of tests with skip status.
	Skipped int `json:"skipped"`
	// Errors is the count of tests with error status.
	Errors int `json:"errors"`
}

// SuiteReport is the aggregate report for a conformance suite run.
type SuiteReport struct {
	// SuiteName is "conformance" or "e2e".
	SuiteName string `json:"suite"`
	// Plugin contains plugin metadata.
	Plugin PluginUnderTest `json:"plugin"`
	// Results contains individual test results.
	Results []TestResult `json:"results"`
	// Summary contains aggregate counts.
	Summary Summary `json:"summary"`
	// StartTime is the suite start timestamp.
	StartTime time.Time `json:"start_time"`
	// EndTime is the suite end timestamp.
	EndTime time.Time `json:"end_time"`
	// TotalTime is the total execution time (serialized as nanoseconds).
	TotalTime time.Duration `json:"duration_ns"`
	// Timestamp is the report generation time (for JSON output).
	Timestamp time.Time `json:"timestamp"`
}

// DefaultTimeout is the default timeout for individual tests (10 seconds).
const DefaultTimeout = 10 * time.Second

// DefaultSuiteTimeout is the default timeout for the entire suite (5 minutes).
const DefaultSuiteTimeout = 5 * time.Minute

// batchTestTimeoutMultiplier is the multiplier for batch tests.
const batchTestTimeoutMultiplier = 2

// MaxBatchSize is the maximum number of resources in a batch request.
const MaxBatchSize = 1000

// ProtocolVersion is the expected protocol version for conformance testing.
const ProtocolVersion = "1.0"

// errInvalidPluginClientType is the error message used when the plugin client
// does not implement pbc.CostSourceServiceClient.
const errInvalidPluginClientType = "invalid plugin client type"

// Standard resource descriptor values shared by conformance test cases.
const (
	// testProviderAWS is the provider of the standard conformance test resource.
	testProviderAWS = "aws"
	// testResourceTypeEC2Instance is the resource type of the standard conformance test resource.
	testResourceTypeEC2Instance = "aws:ec2/instance:Instance"
	// testSKUT3Micro is the SKU of the standard conformance test resource.
	testSKUT3Micro = "t3.micro"
	// testRegionUSEast1 is the region of the standard conformance test resource.
	testRegionUSEast1 = "us-east-1"
)

// RPC method names used in TestCase RequiredMethods declarations.
const (
	// methodName is the Name RPC method name.
	methodName = "Name"
	// methodGetProjectedCost is the GetProjectedCost RPC method name.
	methodGetProjectedCost = "GetProjectedCost"
)
