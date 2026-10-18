package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

//nolint:paralleltest // t.Setenv and config.SetGlobalConfig change process-wide state
func TestHistoryDir_FinFocusHomeLeavesUserHomeUntouched(t *testing.T) {
	home := t.TempDir()
	finfocusHome := t.TempDir()
	setHistoryResolutionEnv(t, home, finfocusHome, "", "")
	cfg := installHistoryConfig(t, "")

	store, cleanup := initHistoryFromConfig(context.Background(), cfg)
	t.Cleanup(cleanup)
	require.NotNil(t, store)

	want := filepath.Join(finfocusHome, "history")
	assert.Equal(t, want, resolveHistoryDir(context.Background(), cfg))
	assert.Equal(t, want, historyDir(""))
	assert.FileExists(t, filepath.Join(want, "history.db"))

	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

//nolint:paralleltest // t.Setenv and config.SetGlobalConfig change process-wide state
func TestHistoryDir_ResolutionOrder(t *testing.T) {
	home := t.TempDir()
	finfocusHome := t.TempDir()
	envDir := filepath.Join(t.TempDir(), "from-env")
	configDir := filepath.Join(t.TempDir(), "from-config")

	tests := []struct {
		name         string
		finfocusHome string
		historyEnv   string
		configDir    string
		want         string
	}{
		{
			name:         "env beats config and finfocus home",
			finfocusHome: finfocusHome,
			historyEnv:   envDir,
			configDir:    configDir,
			want:         envDir,
		},
		{
			name:         "config directory beats finfocus home",
			finfocusHome: finfocusHome,
			configDir:    configDir,
			want:         configDir,
		},
		{
			name:         "finfocus home",
			finfocusHome: finfocusHome,
			want:         filepath.Join(finfocusHome, "history"),
		},
		{
			name: "user home when finfocus home is unset",
			want: filepath.Join(home, ".finfocus", "history"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setHistoryResolutionEnv(t, home, tt.finfocusHome, "", tt.historyEnv)
			cfg := installHistoryConfig(t, tt.configDir)

			want := tt.want
			got := resolveHistoryDir(context.Background(), cfg)
			assert.Equal(t, want, got)
			assert.Equal(t, got, historyDir(""))
		})
	}
}

func setHistoryResolutionEnv(t *testing.T, home, finfocusHome, pulumiHome, historyEnv string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv(config.HistoryEnvDir, historyEnv)
}

func installHistoryConfig(t *testing.T, directory string) *config.Config {
	t.Helper()
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	cfg := &config.Config{}
	cfg.Cost.History.Directory = directory
	config.SetGlobalConfig(cfg)
	return cfg
}

//nolint:paralleltest // t.Setenv and config.SetResolvedProjectDir change process-wide state
func TestHistoryDir_ProjectTierTrust(t *testing.T) {
	home := t.TempDir()
	finfocusHome := t.TempDir()
	absDir := filepath.Join(t.TempDir(), "abs")
	defaultDir := filepath.Join(finfocusHome, "history")

	tests := []struct {
		name       string
		projectDir string
		globalDir  string
		historyEnv string
		want       string
	}{
		{name: "project absolute ignored", projectDir: absDir, want: defaultDir},
		{name: "project dotdot ignored", projectDir: "../x", want: defaultDir},
		{name: "project relative accepted", projectDir: "hist/sub", want: "hist/sub"},
		{name: "global absolute accepted", globalDir: absDir, want: absDir},
		{name: "env accepted over project", projectDir: absDir, historyEnv: "/tmp/from-env", want: "/tmp/from-env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setHistoryResolutionEnv(t, home, finfocusHome, "", tt.historyEnv)
			projectCfgDir := t.TempDir()
			if tt.projectDir != "" {
				body := `{"cost": {"history": {"directory": ` + strconv.Quote(tt.projectDir) + `}}}`
				require.NoError(t, os.WriteFile(filepath.Join(projectCfgDir, "config.hujson"), []byte(body), 0o600))
			}
			config.SetResolvedProjectDir(projectCfgDir)
			t.Cleanup(func() { config.SetResolvedProjectDir("") })

			merged := tt.globalDir
			if tt.projectDir != "" {
				merged = tt.projectDir
			}
			cfg := installHistoryConfig(t, merged)

			assert.Equal(t, tt.want, resolveHistoryDir(context.Background(), cfg))
		})
	}
}

func TestHistoryDir_WorkingDirFallbackIsAbsolute(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("FINFOCUS_HOME", "")
	t.Setenv("PULUMI_HOME", "")
	t.Setenv(config.HistoryEnvDir, "")
	require.True(t, config.UsesWorkingDirFallback())
	cfg := installHistoryConfig(t, "")

	got := resolveHistoryDir(context.Background(), cfg)
	assert.True(t, filepath.IsAbs(got), got)
	assert.Equal(t, "history", filepath.Base(got))
}
