package config

import (
	"fmt"
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

func TestValidateESCCredentials(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	restore := SetESCCatalogNamesForTest([]string{"recorder"})
	t.Cleanup(restore)
	const (
		nameText     = "esc.plugins.recorder.credentials: a credential name must start with a letter"
		variableText = "esc.plugins.recorder.credentials: every value must be an environment variable name"
		reservedText = "esc.plugins.recorder.credentials: a value names a variable reserved for plugin startup"
	)
	tooMany := make([]string, 0, 17)
	for i := range 17 {
		tooMany = append(tooMany, fmt.Sprintf(`"name%d": "OK_NAME"`, i))
	}
	tests := []struct {
		name    string
		entries string
		want    string
	}{
		{name: "valid", entries: `"access_key_id": "AWS_ACCESS_KEY_ID", "vantage-token": "VANTAGE_TOKEN"`},
		{name: "empty", entries: ``},
		{name: "one variable under two names", entries: `"a": "OK_NAME", "b": "OK_NAME"`},
		{name: "bad name", entries: `"test secret": "OK_NAME"`, want: nameText},
		{name: "names equal without case", entries: `"Token": "OK_NAME", "token": "OTHER"`, want: nameText},
		{name: "long name", entries: `"` + strings.Repeat("a", 65) + `": "OK_NAME"`, want: nameText},
		{name: "too many", entries: strings.Join(tooMany, ","), want: nameText},
		{name: "bad variable", entries: `"token": "test secret"`, want: variableText},
		{name: "reserved variable", entries: `"token": "FINFOCUS_PLUGIN_PORT"`, want: reservedText},
	}
	for _, tt := range tests { //nolint:paralleltest // the parent sets FINFOCUS_HOME and the catalog names
		t.Run(tt.name, func(t *testing.T) {
			result, _ := ValidateConfigSource("config.hujson",
				[]byte(`{"esc": {"plugins": {"recorder": {"credentials": {`+tt.entries+`}}}}}`))
			joined := joinMessages(result)
			if tt.want == "" {
				assert.True(t, result.Valid, joined)
				return
			}
			assert.False(t, result.Valid)
			assert.Contains(t, joined, tt.want)
			assert.NotContains(t, joined, "test secret")
		})
	}
}

func joinMessages(result ValidationResult) string {
	var out strings.Builder
	for _, item := range result.Errors {
		out.WriteString(item.Message)
		out.WriteByte('\n')
	}
	return out.String()
}
