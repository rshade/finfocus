package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateESCReferences(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	result, cfg := ValidateConfigSource("config.hujson", []byte(`{
		"esc": {
			"environments": ["a//b", "billing/aws"],
			"plugins": {
				"esc-missing-plugin": {"variables": ["1BAD", "FINFOCUS_PLUGIN_PORT", "OK_NAME"]}
			}
		}
	}`))
	require.NotNil(t, cfg)
	assert.False(t, result.Valid)
	var messages []string
	for _, item := range result.Errors {
		messages = append(messages, item.Message)
	}
	assert.Contains(t, messages, `esc.environments[0]: invalid ESC environment reference "a//b": empty segment`)
	joined := joinMessages(result)
	assert.Contains(t, joined, `invalid environment variable name "1BAD"`)
	assert.Contains(t, joined, "FINFOCUS_PLUGIN_PORT is reserved for plugin startup")
	assert.Contains(t, joined, "esc-missing-plugin")
	var warned bool
	for _, warning := range result.Warnings {
		if warning.Path == "esc.plugins.esc-missing-plugin" {
			warned = true
			assert.Contains(t, warning.Message, `no plugin named "esc-missing-plugin" is installed`)
		}
	}
	assert.True(t, warned)
}

func TestValidateESCWrongType(t *testing.T) {
	t.Parallel()
	result, _ := ValidateConfigSource("config.hujson", []byte(`{"esc":{"environments":"billing/aws"}}`))
	assert.False(t, result.Valid)
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Message, "esc.environments")
}

func TestValidateESCKnownCatalogPlugin(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	restore := SetESCCatalogNamesForTest([]string{"recorder"})
	t.Cleanup(restore)
	result, _ := ValidateConfigSource("config.hujson", []byte(`{
		"esc": {"plugins": {"recorder": {"environments": ["billing/aws"]}}}
	}`))
	for _, warning := range result.Warnings {
		assert.NotEqual(t, "esc.plugins.recorder", warning.Path)
	}
	assert.True(t, result.Valid)
}

func joinMessages(result ValidationResult) string {
	var out strings.Builder
	for _, item := range result.Errors {
		out.WriteString(item.Message)
		out.WriteByte('\n')
	}
	return out.String()
}
