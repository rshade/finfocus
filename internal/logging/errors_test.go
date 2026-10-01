package logging_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/rshade/finfocus/internal/logging"
)

func TestUserError(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying cause")
	err := logging.UserError("Invalid input", "Check your input and try again", cause)

	if err.Category != logging.ErrorCategoryUser {
		t.Errorf("Expected USER category, got %v", err.Category)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "[USER]") {
		t.Error("Error message should contain [USER] category")
	}
	if !strings.Contains(errMsg, "Invalid input") {
		t.Error("Error message should contain message")
	}
	if !strings.Contains(errMsg, "Check your input") {
		t.Error("Error message should contain solution")
	}
}

func TestSystemError(t *testing.T) {
	t.Parallel()

	cause := errors.New("network timeout")
	err := logging.SystemError("Network failure", "Check your connection", cause)

	if err.Category != logging.ErrorCategorySystem {
		t.Errorf("Expected SYSTEM category, got %v", err.Category)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "[SYSTEM]") {
		t.Error("Error message should contain [SYSTEM] category")
	}
}

func TestDeveloperError(t *testing.T) {
	t.Parallel()

	err := logging.DeveloperError("Protocol mismatch", "Update the plugin", nil)

	if err.Category != logging.ErrorCategoryDeveloper {
		t.Errorf("Expected DEVELOPER category, got %v", err.Category)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "[DEVELOPER]") {
		t.Error("Error message should contain [DEVELOPER] category")
	}
}

func TestCategorizedError_WithContext(t *testing.T) {
	t.Parallel()

	err := logging.UserError("Test error", "Fix it", nil).
		WithContext("key1", "value1").
		WithContext("key2", "value2")

	errMsg := err.Error()
	if !strings.Contains(errMsg, "key1: value1") {
		t.Error("Error should contain context key1")
	}
	if !strings.Contains(errMsg, "key2: value2") {
		t.Error("Error should contain context key2")
	}
}

func TestCategorizedError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("root cause")
	err := logging.UserError("Wrapper error", "Solution", cause)

	unwrapped := errors.Unwrap(err)
	if !errors.Is(err, cause) {
		t.Error("Unwrap should return the original cause")
	}
	if unwrapped == nil {
		t.Error("Unwrapped error should not be nil")
	}
}

func TestInvalidArgumentError(t *testing.T) {
	t.Parallel()

	err := logging.InvalidArgumentError("--invalid-flag", nil)

	if err.Category != logging.ErrorCategoryUser {
		t.Error("InvalidArgumentError should be USER category")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "--invalid-flag") {
		t.Error("Error should mention the invalid argument")
	}
	if !strings.Contains(errMsg, "--help") {
		t.Error("Error should suggest using --help")
	}
}

func TestMissingConfigError(t *testing.T) {
	t.Parallel()

	err := logging.MissingConfigError("api.key", nil)

	errMsg := err.Error()
	if !strings.Contains(errMsg, "api.key") {
		t.Error("Error should mention the config key")
	}
	if !strings.Contains(errMsg, "config init") {
		t.Error("Error should suggest config init")
	}
}

func TestInvalidPulumiJSONError(t *testing.T) {
	t.Parallel()

	err := logging.InvalidPulumiJSONError("/path/to/plan.json", nil)

	errMsg := err.Error()
	if !strings.Contains(errMsg, "/path/to/plan.json") {
		t.Error("Error should mention the file path")
	}
	if !strings.Contains(errMsg, "pulumi preview --json") {
		t.Error("Error should suggest how to generate valid JSON")
	}
}

func TestPluginNotFoundError(t *testing.T) {
	t.Parallel()

	err := logging.PluginNotFoundError("aws-plugin", nil)

	errMsg := err.Error()
	if !strings.Contains(errMsg, "aws-plugin") {
		t.Error("Error should mention the plugin name")
	}
	if !strings.Contains(errMsg, ".finfocus/plugins") {
		t.Error("Error should mention the plugin directory")
	}
}

func TestNetworkError(t *testing.T) {
	t.Parallel()

	err := logging.NetworkError("API call", errors.New("timeout"))

	if err.Category != logging.ErrorCategorySystem {
		t.Error("NetworkError should be SYSTEM category")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "API call") {
		t.Error("Error should mention the operation")
	}
}

func TestFileSystemError(t *testing.T) {
	t.Parallel()

	err := logging.FileSystemError("write", "/tmp/test.log", errors.New("permission denied"))

	if err.Category != logging.ErrorCategorySystem {
		t.Error("FileSystemError should be SYSTEM category")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "write") {
		t.Error("Error should mention the operation")
	}
	if !strings.Contains(errMsg, "/tmp/test.log") {
		t.Error("Error should mention the path")
	}
}

func TestPluginCommunicationError(t *testing.T) {
	t.Parallel()

	err := logging.PluginCommunicationError("aws-plugin", errors.New("connection refused"))

	if err.Category != logging.ErrorCategorySystem {
		t.Error("PluginCommunicationError should be SYSTEM category")
	}
}

func TestProtocolMismatchError(t *testing.T) {
	t.Parallel()

	err := logging.ProtocolMismatchError("aws-plugin", "v1.0", "v2.0")

	if err.Category != logging.ErrorCategoryDeveloper {
		t.Error("ProtocolMismatchError should be DEVELOPER category")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "v1.0") || !strings.Contains(errMsg, "v2.0") {
		t.Error("Error should mention both versions")
	}
}

func TestPluginBugError(t *testing.T) {
	t.Parallel()

	err := logging.PluginBugError("aws-plugin", "GetCost", errors.New("nil pointer"))

	if err.Category != logging.ErrorCategoryDeveloper {
		t.Error("PluginBugError should be DEVELOPER category")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "aws-plugin") {
		t.Error("Error should mention plugin name")
	}
	if !strings.Contains(errMsg, "GetCost") {
		t.Error("Error should mention operation")
	}
}
