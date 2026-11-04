package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebLogsSettingDescriptor(t *testing.T) {
	t.Parallel()
	setting, ok := LookupSetting("web.logs.enabled")
	require.True(t, ok)
	assert.Equal(t, KindBool, setting.Kind)
	assert.Contains(t, setting.Help, "resource names")
	assert.Contains(t, setting.Help, "paths")
	assert.Equal(t, false, defaultValue("web.logs.enabled"))
}

func TestWebLogsSettingPatch(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")

	_, err := PatchFile(path, TierGlobal, "", []Change{{Key: "web.logs.enabled", Value: strPtr("true")}}, false)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"web"`)
	assert.Contains(t, string(data), "true")

	_, err = PatchFile(path, TierGlobal, "", []Change{{Key: "web.logs.enabled", Value: strPtr("false")}}, false)
	require.NoError(t, err)

	for _, bad := range []string{"maybe", ""} {
		_, err = PatchFile(path, TierGlobal, "", []Change{{Key: "web.logs.enabled", Value: strPtr(bad)}}, false)
		require.Error(t, err, "value %q", bad)
		assert.Contains(t, err.Error(), "must be a boolean", "value %q", bad)
	}
}

func TestWebLogsEffectiveValueFollowsEnvironment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		fileValue  string
		environ    []string
		wantSource string
		wantValue  any
	}{
		{"env on, no file", "", []string{"FINFOCUS_WEB_LOGS=1"}, SourceEnv, true},
		{"env true, no file", "", []string{"FINFOCUS_WEB_LOGS=true"}, SourceEnv, true},
		{"env off beats file on", "true", []string{"FINFOCUS_WEB_LOGS=0"}, SourceEnv, false},
		{"env false beats file on", "true", []string{"FINFOCUS_WEB_LOGS=false"}, SourceEnv, false},
		{"env on beats file off", "false", []string{"FINFOCUS_WEB_LOGS=1"}, SourceEnv, true},
		{"invalid env is ignored", "true", []string{"FINFOCUS_WEB_LOGS=maybe"}, SourceGlobal, true},
		{"empty env is ignored", "true", []string{"FINFOCUS_WEB_LOGS="}, SourceGlobal, true},
		{"no env, no file", "", nil, SourceDefault, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			global := filepath.Join(t.TempDir(), "config.hujson")
			if tt.fileValue != "" {
				change := Change{Key: "web.logs.enabled", Value: strPtr(tt.fileValue)}
				_, err := PatchFile(global, TierGlobal, "", []Change{change}, false)
				require.NoError(t, err)
			}
			page, err := buildPage(global, "", tt.environ)
			require.NoError(t, err)
			for _, view := range page.Settings {
				if view.Key == "web.logs.enabled" {
					assert.Equal(t, tt.wantSource, view.Effective.Source)
					assert.Equal(t, tt.wantValue, view.Effective.Value)
					return
				}
			}
			require.Fail(t, "web.logs.enabled must be on the Config page")
		})
	}
}

func TestWebLogsSettingOnPage(t *testing.T) {
	t.Parallel()
	global := filepath.Join(t.TempDir(), "config.hujson")
	page, err := BuildPage(global, "")
	require.NoError(t, err)

	found := false
	for _, view := range page.Settings {
		if view.Key == "web.logs.enabled" {
			found = true
			assert.Equal(t, SourceDefault, view.Effective.Source)
		}
	}
	assert.True(t, found, "web.logs.enabled must be on the Config page")
}
