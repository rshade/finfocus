package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
	setHistoryResolutionEnv(t, home, finfocusHome, "")
	cfg := installHistoryConfig(t, "")

	store, cleanup := initHistoryFromConfig(context.Background(), cfg, "")
	t.Cleanup(cleanup)
	require.NotNil(t, store)

	want := filepath.Join(finfocusHome, "history")
	assert.Equal(t, want, resolveHistoryDir(context.Background(), cfg, ""))
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
			setHistoryResolutionEnv(t, home, tt.finfocusHome, tt.historyEnv)
			cfg := installHistoryConfig(t, tt.configDir)

			want := tt.want
			got := resolveHistoryDir(context.Background(), cfg, "")
			assert.Equal(t, want, got)
			assert.Equal(t, got, historyDir(""))
		})
	}
}

func setHistoryResolutionEnv(t *testing.T, home, finfocusHome, historyEnv string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	t.Setenv("PULUMI_HOME", "")
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

func TestHistoryDir_ProjectTierTrust(t *testing.T) {
	home := t.TempDir()
	finfocusHome := t.TempDir()
	absDir := filepath.Join(t.TempDir(), "abs")
	defaultDir := filepath.Join(finfocusHome, "history")

	tests := []struct {
		name       string
		projectDir string
		symlink    bool
		globalDir  string
		historyEnv string
		wantRel    string
		want       string
	}{
		{name: "project absolute ignored", projectDir: absDir, want: defaultDir},
		{name: "project dotdot ignored", projectDir: "../x", want: defaultDir},
		{name: "project symlink out of project ignored", projectDir: "link/sub", symlink: true, want: defaultDir},
		{name: "project relative accepted", projectDir: "hist/sub", wantRel: "hist/sub"},
		{name: "global absolute accepted", globalDir: absDir, want: absDir},
		{name: "env accepted over project", projectDir: absDir, historyEnv: "/tmp/from-env", want: "/tmp/from-env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.symlink && runtime.GOOS == goosWindows {
				t.Skip("creating symlinks needs elevated privileges on Windows")
			}
			setHistoryResolutionEnv(t, home, finfocusHome, tt.historyEnv)
			root := t.TempDir()
			projectCfgDir := filepath.Join(root, ".finfocus")
			require.NoError(t, os.MkdirAll(projectCfgDir, 0o750))
			if tt.symlink {
				require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(root, "link")))
			}
			if tt.projectDir != "" {
				body := `{"cost": {"history": {"directory": ` + strconv.Quote(tt.projectDir) + `}}}`
				require.NoError(t, os.WriteFile(filepath.Join(projectCfgDir, "config.hujson"), []byte(body), 0o600))
			}
			t.Chdir(root)
			// The analyzer builds its config from its own project dir while the
			// process-wide resolved dir is empty or points elsewhere.
			config.SetResolvedProjectDir("")
			t.Cleanup(func() { config.SetResolvedProjectDir("") })

			var cfg *config.Config
			if tt.projectDir != "" {
				cfg = config.NewWithProjectDir(context.Background(), projectCfgDir)
				require.Equal(t, tt.projectDir, cfg.Cost.History.Directory)
			} else {
				cfg = installHistoryConfig(t, tt.globalDir)
			}

			want := tt.want
			if tt.wantRel != "" {
				want = filepath.FromSlash(tt.wantRel)
			}
			assert.Equal(t, want, filepath.FromSlash(resolveHistoryDir(context.Background(), cfg, projectCfgDir)))
			config.SetResolvedProjectDir(t.TempDir())
			assert.Equal(t, want, filepath.FromSlash(resolveHistoryDir(context.Background(), cfg, projectCfgDir)))
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

	got := resolveHistoryDir(context.Background(), cfg, "")
	assert.True(t, filepath.IsAbs(got), got)
	assert.Equal(t, "history", filepath.Base(got))
}

func TestHistoryDir_ProjectTierSymlinkedDatabase(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		symlink  bool
	}{
		{name: "history.db symlink rejected", fileName: "history.db", symlink: true},
		{name: "project history file symlink rejected", fileName: "proj@dev.history.db", symlink: true},
		{name: "uppercase HISTORY.DB symlink rejected", fileName: "HISTORY.DB", symlink: true},
		{name: "uppercase suffix symlink rejected", fileName: "proj@dev.HISTORY.DB", symlink: true},
		{name: "compact temp symlink rejected", fileName: "proj@dev.history.db.compact", symlink: true},
		{name: "history.db sidecar symlink rejected", fileName: "history.db.compact", symlink: true},
		{name: "regular history.db accepted", fileName: "history.db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.symlink && runtime.GOOS == goosWindows {
				t.Skip("creating symlinks needs elevated privileges on Windows")
			}
			finfocusHome := t.TempDir()
			setHistoryResolutionEnv(t, t.TempDir(), finfocusHome, "")
			root := t.TempDir()
			projectCfgDir := filepath.Join(root, ".finfocus")
			require.NoError(t, os.MkdirAll(projectCfgDir, 0o750))
			require.NoError(t, os.MkdirAll(filepath.Join(root, "hist"), 0o750))
			body := `{"cost": {"history": {"directory": "hist"}}}`
			require.NoError(t, os.WriteFile(filepath.Join(projectCfgDir, "config.hujson"), []byte(body), 0o600))

			inProject := filepath.Join(root, "hist", tt.fileName)
			outside := filepath.Join(t.TempDir(), "outside.db")
			if tt.symlink {
				require.NoError(t, os.Symlink(outside, inProject))
			} else {
				require.NoError(t, os.WriteFile(inProject, nil, 0o600))
			}
			t.Chdir(root)
			config.SetResolvedProjectDir("")
			t.Cleanup(func() { config.SetResolvedProjectDir("") })

			cfg := config.NewWithProjectDir(context.Background(), projectCfgDir)
			got := resolveHistoryDir(context.Background(), cfg, projectCfgDir)
			if !tt.symlink {
				assert.Equal(t, "hist", filepath.FromSlash(got))
				return
			}
			assert.Equal(t, filepath.Join(finfocusHome, "history"), got)
			_, err := os.Stat(outside)
			assert.ErrorIs(t, err, os.ErrNotExist, "the link target must stay untouched")
		})
	}
}

func TestHistoryDir_GlobalDirectorySurvivesUnsafeProjectOverlay(t *testing.T) {
	finfocusHome := t.TempDir()
	setHistoryResolutionEnv(t, t.TempDir(), finfocusHome, "")
	root := t.TempDir()
	projectCfgDir := filepath.Join(root, ".finfocus")
	require.NoError(t, os.MkdirAll(projectCfgDir, 0o750))
	unsafe := filepath.Join(t.TempDir(), "elsewhere")
	body := `{"cost": {"history": {"directory": ` + strconv.Quote(unsafe) + `}}}`
	require.NoError(t, os.WriteFile(filepath.Join(projectCfgDir, "config.hujson"), []byte(body), 0o600))
	t.Chdir(root)
	config.SetResolvedProjectDir("")
	t.Cleanup(func() { config.SetResolvedProjectDir("") })

	globalDir := filepath.Join(t.TempDir(), "global-history")
	cfg := installHistoryConfig(t, globalDir)

	assert.Equal(t, globalDir, resolveHistoryDir(context.Background(), cfg, projectCfgDir))
}
