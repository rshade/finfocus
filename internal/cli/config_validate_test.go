package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate_JSONReport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hujson")
	body := []byte("{\n" +
		"  \"cost\": {\n" +
		"    \"budgets\": {\n" +
		"      \"global\": {\n" +
		"        \"ammount\": 100\n" +
		"      }\n" +
		"    }\n" +
		"  }\n" +
		"}\n")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	cmd := NewConfigValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--file", path, "--output", "json"})
	err := cmd.Execute()
	require.NoError(t, err)

	var report struct {
		Valid    bool `json:"valid"`
		Warnings []struct {
			Line       int    `json:"line"`
			Path       string `json:"path"`
			Message    string `json:"message"`
			Suggestion string `json:"suggestion"`
		} `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.True(t, report.Valid)
	require.NotEmpty(t, report.Warnings)
	assert.Equal(t, 5, report.Warnings[0].Line)
	assert.Equal(t, "cost.budgets.global.ammount", report.Warnings[0].Path)
	assert.Contains(t, report.Warnings[0].Message, "ammount")
	assert.Equal(t, "Did you mean: 'amount'?", report.Warnings[0].Suggestion)
}

func TestConfigValidate_TextError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hujson")
	body := []byte("{\n" +
		"  \"cost\": {\"budgets\": {\"global\": {\"period\": \"weekly\",\n" +
		"    \"amount\": 10, \"currency\": \"USD\"}}}\n" +
		"}\n")
	require.NoError(t, os.WriteFile(path, body, 0o600))

	cmd := NewConfigValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--file", path, "--output", "text"})
	err := cmd.Execute()
	require.ErrorIs(t, err, errConfigurationInvalid)
	assert.Contains(t, stdout.String(), "Error at line 2:")
	assert.Contains(t, stdout.String(), "monthly")
	assert.Contains(t, stdout.String(), "Hint:")
	assert.Contains(t, stdout.String(), "period: monthly")
}

func TestConfigValidate_RejectsUnknownOutput(t *testing.T) {
	t.Parallel()
	cmd := NewConfigValidateCmd()
	cmd.SetArgs([]string{"--output", "yaml"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --output")
}

func TestValidateCostConfig_RejectsInvalidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hujson")
	body := []byte(`{"cost":{"budgets":{"global":{"amount":10,"currency":"USD","period":"weekly"}}}}`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	cmd := &cobra.Command{Use: "cost"}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := validateCostConfig(cmd, []string{path, ""})
	require.ErrorIs(t, err, errConfigurationInvalid)
	assert.Contains(t, stderr.String(), "monthly")
	assert.Contains(t, stderr.String(), "Hint:")
}

func TestValidateCostConfig_MissingFile(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "cost"}
	err := validateCostConfig(cmd, []string{filepath.Join(t.TempDir(), "missing.hujson")})
	require.NoError(t, err)
}
