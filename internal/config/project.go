package config

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/rshade/finfocus/internal/pulumi"
)

// resolvedProjectDir holds the resolved project directory path for use
// by other config functions during the lifetime of a CLI invocation.
var (
	resolvedProjectDir   string       //nolint:gochecknoglobals // Set once at startup, read by config loaders
	resolvedProjectDirMu sync.RWMutex //nolint:gochecknoglobals // Protects resolvedProjectDir
)

// SetResolvedProjectDir stores the resolved project directory for use by other config functions.
func SetResolvedProjectDir(dir string) {
	resolvedProjectDirMu.Lock()
	defer resolvedProjectDirMu.Unlock()
	resolvedProjectDir = dir
}

// GetResolvedProjectDir returns the stored resolved project directory.
func GetResolvedProjectDir() string {
	resolvedProjectDirMu.RLock()
	defer resolvedProjectDirMu.RUnlock()
	return resolvedProjectDir
}

// ResolveProjectDir determines the project-local .finfocus directory path.
// It checks (in order):
//  1. flagValue (--project-dir CLI flag)
//  2. FINFOCUS_PROJECT_DIR env var
//  3. pulumi.FindProject(startDir) walk-up
//
// Returns the path to $PROJECT/.finfocus/ or empty string if no project found.
// Does NOT create the directory (read-only operation).
// Returned path is always absolute (or empty).
func ResolveProjectDir(flagValue, startDir string) string {
	if flagValue != "" {
		return toAbsFinfocusDir(flagValue)
	}

	if envDir := os.Getenv("FINFOCUS_PROJECT_DIR"); envDir != "" {
		return toAbsFinfocusDir(envDir)
	}

	projectRoot, err := pulumi.FindProject(startDir)
	if err != nil {
		if !errors.Is(err, pulumi.ErrNoProject) {
			log.Warn().
				Err(err).
				Str("start_dir", startDir).
				Msg("unexpected error during Pulumi project discovery")
		}
		return ""
	}

	return filepath.Join(projectRoot, ".finfocus")
}

// NewWithProjectDir creates a Config by loading global config then
// shallow-merging project-local config on top. If projectDir is empty,
// behaves identically to New().
func NewWithProjectDir(projectDir string) *Config {
	cfg := New()

	if projectDir == "" {
		return cfg
	}

	overlayPath := filepath.Join(projectDir, "config.yaml")
	if _, err := os.Stat(overlayPath); err != nil {
		// Missing project config is not an error — use global defaults.
		return cfg
	}

	if err := ShallowMergeYAML(cfg, overlayPath); err != nil {
		log.Warn().
			Err(err).
			Str("overlay_path", overlayPath).
			Msg("failed to merge project config, using global defaults")
		return cfg
	}

	return cfg
}

// toAbsFinfocusDir converts dir to an absolute path and appends ".finfocus".
// If the path already ends with ".finfocus", it is returned as-is (after
// resolving to an absolute path) to prevent double-append.
func toAbsFinfocusDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		log.Warn().
			Err(err).
			Str("dir", dir).
			Msg("failed to resolve absolute path for project directory")
		abs = dir
	}

	if filepath.Base(abs) == ".finfocus" {
		return abs
	}

	return filepath.Join(abs, ".finfocus")
}
