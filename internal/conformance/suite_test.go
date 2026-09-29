package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSuite_ValidConfig(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		CommMode:   CommModeTCP,
		Verbosity:  VerbosityNormal,
		Timeout:    5 * time.Minute,
	}

	suite, err := NewSuite(cfg)

	require.NoError(t, err)
	require.NotNil(t, suite)
	assert.Equal(t, "/path/to/plugin", suite.config.PluginPath)
	assert.Equal(t, CommModeTCP, suite.config.CommMode)
	assert.Equal(t, VerbosityNormal, suite.config.Verbosity)
	assert.Equal(t, 5*time.Minute, suite.config.Timeout)
}

func TestNewSuite_DefaultValues(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
	}

	suite, err := NewSuite(cfg)

	require.NoError(t, err)
	require.NotNil(t, suite)
	assert.Equal(t, CommModeTCP, suite.config.CommMode)
	assert.Equal(t, VerbosityNormal, suite.config.Verbosity)
	assert.Equal(t, DefaultSuiteTimeout, suite.config.Timeout)
}

func TestNewSuite_EmptyPluginPath(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "",
	}

	suite, err := NewSuite(cfg)

	require.Error(t, err)
	assert.Nil(t, suite)
	assert.Contains(t, err.Error(), "plugin path")
}

func TestNewSuite_InvalidCommMode(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		CommMode:   CommMode("invalid"),
	}

	suite, err := NewSuite(cfg)

	require.Error(t, err)
	assert.Nil(t, suite)
	assert.Contains(t, err.Error(), "comm mode")
}

func TestNewSuite_InvalidVerbosity(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		Verbosity:  Verbosity("invalid"),
	}

	suite, err := NewSuite(cfg)

	require.Error(t, err)
	assert.Nil(t, suite)
	assert.Contains(t, err.Error(), "verbosity")
}

func TestNewSuite_WithCategories(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		Categories: []Category{CategoryProtocol, CategoryError},
	}

	suite, err := NewSuite(cfg)

	require.NoError(t, err)
	require.NotNil(t, suite)
	assert.Len(t, suite.config.Categories, 2)
	assert.Contains(t, suite.config.Categories, CategoryProtocol)
	assert.Contains(t, suite.config.Categories, CategoryError)
}

func TestNewSuite_WithTestFilter(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		TestFilter: "Name_.*",
	}

	suite, err := NewSuite(cfg)

	require.NoError(t, err)
	require.NotNil(t, suite)
	assert.Equal(t, "Name_.*", suite.config.TestFilter)
}

func TestNewSuite_InvalidTestFilter(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		TestFilter: "[invalid regex", // Unclosed bracket is invalid regex
	}

	suite, err := NewSuite(cfg)

	require.Error(t, err)
	assert.Nil(t, suite)
	assert.Contains(t, err.Error(), "test filter")
}

func TestNewSuite_WithLogger(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		Logger:     logger,
	}

	suite, err := NewSuite(cfg)

	require.NoError(t, err)
	require.NotNil(t, suite)
}

func TestSuite_Run_ContextCancellation(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/nonexistent/plugin",
		Timeout:    100 * time.Millisecond,
	}

	suite, err := NewSuite(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	report, err := suite.Run(ctx)

	// Should return error due to cancelled context
	require.Error(t, err)
	assert.Nil(t, report)
}

func TestSuite_GetTestCases(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
	}

	suite, err := NewSuite(cfg)
	require.NoError(t, err)

	testCases := suite.GetTestCases()

	// Should have test cases registered
	assert.NotEmpty(t, testCases)

	// Verify test cases have required fields
	for _, tc := range testCases {
		assert.NotEmpty(t, tc.Name, "test case name should not be empty")
		assert.NotEmpty(t, tc.Category, "test case category should not be empty")
		assert.NotEmpty(t, tc.Description, "test case description should not be empty")
		assert.Positive(t, tc.Timeout, "test case timeout should be positive")
		assert.NotNil(t, tc.TestFunc, "test case function should not be nil")
	}
}

func TestSuite_GetTestCases_FilterByCategory(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		Categories: []Category{CategoryProtocol},
	}

	suite, err := NewSuite(cfg)
	require.NoError(t, err)

	testCases := suite.GetTestCases()

	// All returned test cases should be in the protocol category
	for _, tc := range testCases {
		assert.Equal(t, CategoryProtocol, tc.Category,
			"test case %s should be in protocol category", tc.Name)
	}
}

func TestSuite_GetTestCases_FilterByRegex(t *testing.T) {
	t.Parallel()

	cfg := SuiteConfig{
		PluginPath: "/path/to/plugin",
		TestFilter: "^Name_.*",
	}

	suite, err := NewSuite(cfg)
	require.NoError(t, err)

	testCases := suite.GetTestCases()

	// All returned test cases should match the regex
	for _, tc := range testCases {
		assert.Regexp(t, "^Name_.*", tc.Name,
			"test case %s should match filter regex", tc.Name)
	}
}
