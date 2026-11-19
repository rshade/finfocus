package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maskedConfig = `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":80,"type":"actual","notifications":[
    {"type":"slack","url":"${FINFOCUS_NOTIFY_SLACK_URL}","channel":"#finops"},
    {"type":"webhook","url":"https://api.example.com/hooks/literal-url-secret",
     "headers":{"Authorization":"Bearer literal-header-secret","X-Ref":"${FINFOCUS_NOTIFY_API_TOKEN}"}},
    {"type":"email","smtp":{"host":"localhost","username":"smtp-user-secret@example.com","password":"smtp-password-secret"},
     "from":"smtp-user-secret@example.com","to":["owner@example.com"]}
  ]}]}}}}`

//nolint:paralleltest // sets env and the global config singleton
func TestConfigDisplayMasksNotificationSecrets(t *testing.T) {
	notifyHome(t, maskedConfig)

	runs := [][]string{
		{"config", "list"},
		{"config", "list", "--as", "json"},
		{"config", "get", "cost", "--format", "json"},
		{"config", "get", "cost.budgets", "--format", "json"},
	}
	for _, args := range runs {
		result := runCLI(t, args...)
		require.Equal(t, 0, result.ExitCode, "args %v stderr: %s", args, result.Stderr)
		out := string(result.Stdout)
		assert.NotContains(t, out, "literal-url-secret", "args %v", args)
		assert.NotContains(t, out, "literal-header-secret", "args %v", args)
		assert.NotContains(t, out, "smtp-user-secret@example.com", "args %v", args)
		assert.NotContains(t, out, "smtp-password-secret", "args %v", args)
		assert.Contains(t, out, "[REDACTED]", "args %v", args)
		assert.Contains(t, out, "${FINFOCUS_NOTIFY_SLACK_URL}", "args %v", args)
		assert.Contains(t, out, "${FINFOCUS_NOTIFY_API_TOKEN}", "args %v", args)
	}
}

//nolint:paralleltest // sets env and the global config singleton
func TestConfigSetKeepsNotificationSecretsInFile(t *testing.T) {
	home := notifyHome(t, maskedConfig)

	result := runCLI(t, "config", "set", "output.precision", "3")
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)

	data, err := os.ReadFile(filepath.Join(home, "config.hujson"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "https://api.example.com/hooks/literal-url-secret")
	assert.Contains(t, string(data), "Bearer literal-header-secret")
	assert.Contains(t, string(data), "smtp-user-secret@example.com")
	assert.Contains(t, string(data), "smtp-password-secret")
	assert.NotContains(t, string(data), "[REDACTED]")
}
