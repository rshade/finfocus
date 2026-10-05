package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMarksWorkingDirFallbackDestinationsAsProject(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", "")
	t.Setenv("PULUMI_HOME", "")
	t.Setenv("HOME", "")
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".finfocus"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".finfocus", "config.hujson"), []byte(`{
  "cost": {"budgets": {"global": {"amount": 100, "currency": "USD", "alerts": [
    {"threshold": 80, "type": "actual",
     "notifications": [{"type": "slack", "url": "${FINFOCUS_NOTIFY_SLACK_URL}"}]}]}}}}`), 0o600))

	require.True(t, UsesWorkingDirFallback())
	cfg := New()
	dests := cfg.Cost.Budgets.Global.Alerts[0].Notifications
	require.Len(t, dests, 1)
	assert.True(t, dests[0].FromProject())

	strict, err := NewStrict()
	require.NoError(t, err)
	assert.True(t, strict.Cost.Budgets.Global.Alerts[0].Notifications[0].FromProject())
}

func TestNewKeepsHomeDestinationsGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), []byte(`{
  "cost": {"budgets": {"global": {"amount": 100, "currency": "USD", "alerts": [
    {"threshold": 80, "type": "actual",
     "notifications": [{"type": "slack", "url": "${FINFOCUS_NOTIFY_SLACK_URL}"}]}]}}}}`), 0o600))

	assert.False(t, UsesWorkingDirFallback())
	assert.False(t, New().Cost.Budgets.Global.Alerts[0].Notifications[0].FromProject())
}
