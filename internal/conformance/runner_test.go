package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunner(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	require.NotNil(t, runner)
	assert.Equal(t, VerbosityNormal, runner.verbosity)
}

func TestRunner_RunTest_Pass(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	testCase := TestCase{
		Name:        "TestPass",
		Category:    CategoryProtocol,
		Description: "A test that always passes",
		Timeout:     5 * time.Second,
		TestFunc: func(_ *TestContext) *TestResult {
			return &TestResult{
				TestName: "TestPass",
				Category: CategoryProtocol,
				Status:   StatusPass,
			}
		},
	}

	ctx := context.Background()
	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	assert.Equal(t, "TestPass", result.TestName)
	assert.Equal(t, StatusPass, result.Status)
	assert.Empty(t, result.Error)
}

func TestRunner_RunTest_Fail(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	testCase := TestCase{
		Name:        "TestFail",
		Category:    CategoryError,
		Description: "A test that always fails",
		Timeout:     5 * time.Second,
		TestFunc: func(_ *TestContext) *TestResult {
			return &TestResult{
				TestName: "TestFail",
				Category: CategoryError,
				Status:   StatusFail,
				Error:    "expected NotFound, got InvalidArgument",
			}
		},
	}

	ctx := context.Background()
	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	assert.Equal(t, "TestFail", result.TestName)
	assert.Equal(t, StatusFail, result.Status)
	assert.Equal(t, "expected NotFound, got InvalidArgument", result.Error)
}

func TestRunner_RunTest_Skip(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	testCase := TestCase{
		Name:        "TestSkip",
		Category:    CategoryProtocol,
		Description: "A test that is skipped",
		Timeout:     5 * time.Second,
		TestFunc: func(_ *TestContext) *TestResult {
			return &TestResult{
				TestName: "TestSkip",
				Category: CategoryProtocol,
				Status:   StatusSkip,
				Error:    "credentials not configured",
			}
		},
	}

	ctx := context.Background()
	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	assert.Equal(t, "TestSkip", result.TestName)
	assert.Equal(t, StatusSkip, result.Status)
	assert.Equal(t, "credentials not configured", result.Error)
}

func TestRunner_RunTest_Timeout(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	testCase := TestCase{
		Name:        "TestTimeout",
		Category:    CategoryPerformance,
		Description: "A test that times out",
		Timeout:     50 * time.Millisecond,
		TestFunc: func(_ *TestContext) *TestResult {
			// Simulate a long-running test that should timeout
			time.Sleep(5 * time.Second)
			return &TestResult{
				TestName: "TestTimeout",
				Status:   StatusPass,
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	assert.Equal(t, "TestTimeout", result.TestName)
	// The test should return with error status due to timeout
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "timeout")
}

func TestRunner_RunTest_Panic(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	testCase := TestCase{
		Name:        "TestPanic",
		Category:    CategoryError,
		Description: "A test that panics",
		Timeout:     5 * time.Second,
		TestFunc: func(_ *TestContext) *TestResult {
			panic("test panic")
		},
	}

	ctx := context.Background()
	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	assert.Equal(t, "TestPanic", result.TestName)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "panic")
}

func TestRunner_RunTest_RecordsDuration(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	testCase := TestCase{
		Name:        "TestDuration",
		Category:    CategoryProtocol,
		Description: "A test that takes some time",
		Timeout:     5 * time.Second,
		TestFunc: func(_ *TestContext) *TestResult {
			time.Sleep(10 * time.Millisecond)
			return &TestResult{
				TestName: "TestDuration",
				Status:   StatusPass,
			}
		},
	}

	ctx := context.Background()
	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	// Allow small timing variance on slow CI environments
	assert.GreaterOrEqual(t, result.Duration, 9*time.Millisecond)
	assert.NotZero(t, result.Timestamp)
}

func TestRunner_RunTest_WithVerboseOutput(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityVerbose)

	testCase := TestCase{
		Name:        "TestVerbose",
		Category:    CategoryProtocol,
		Description: "A test with verbose output",
		Timeout:     5 * time.Second,
		TestFunc: func(_ *TestContext) *TestResult {
			return &TestResult{
				TestName: "TestVerbose",
				Status:   StatusPass,
				Details:  "Request: {...}, Response: {...}",
			}
		},
	}

	ctx := context.Background()
	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	assert.Equal(t, "Request: {...}, Response: {...}", result.Details)
}

func TestRunner_RunTest_NilTestFunc(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	runner := NewRunner(logger, VerbosityNormal)

	testCase := TestCase{
		Name:        "TestNilFunc",
		Category:    CategoryProtocol,
		Description: "A test with nil function",
		Timeout:     5 * time.Second,
		TestFunc:    nil,
	}

	ctx := context.Background()
	result := runner.RunTest(ctx, testCase, nil)

	require.NotNil(t, result)
	assert.Equal(t, "TestNilFunc", result.TestName)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "test function is nil")
}
