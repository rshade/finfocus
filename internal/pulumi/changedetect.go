package pulumi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/logging"
)

// ChangeSignal is the result of lightweight change detection.
// It is populated by DetectChanges and consumed by the overview command
// to decide whether to run pulumi preview and what prompt to show.
type ChangeSignal struct {
	// HasLikelyChanges is true when source files are newer than the last deployment.
	HasLikelyChanges bool `json:"hasLikelyChanges"`
	// IsFirstDeploy is true when no deployment timestamp exists in the stack manifest.
	// An empty stack (never deployed) always has HasLikelyChanges=true.
	IsFirstDeploy bool `json:"isFirstDeploy"`
	// ModifiedFiles contains the names of files whose mtime exceeded the deployment timestamp.
	// Populated only when HasLikelyChanges=true and IsFirstDeploy=false.
	ModifiedFiles []string `json:"modifiedFiles,omitempty"`
}

// DetectChanges compares the last deployment timestamp in the Pulumi stack manifest
// against the modification times of Pulumi source files in projectDir.
//
// It returns a ChangeSignal indicating whether pending changes are likely.
// It never runs any external commands — all data comes from the already-parsed manifest.
//
// manifestTime is the value of StackExportManifest.Time (RFC3339 string, may be empty
// for stacks that have never been deployed).
// projectDir is the directory containing Pulumi.yaml (used to find source files).
func DetectChanges(ctx context.Context, manifestTime string, projectDir string) (ChangeSignal, error) {
	log := logging.FromContext(ctx)

	if strings.TrimSpace(projectDir) == "" {
		return ChangeSignal{}, errors.New("detecting changes: projectDir is required")
	}

	// 1. Empty manifestTime means the stack has never been deployed.
	if manifestTime == "" {
		return ChangeSignal{HasLikelyChanges: true, IsFirstDeploy: true}, nil
	}

	// 2. Parse the manifest timestamp.
	lastDeployment, parseErr := time.Parse(time.RFC3339, manifestTime)
	if parseErr != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "changedetect").
			Str("operation", "parse_manifest_time").
			Str("manifest_time", manifestTime).
			Err(parseErr).
			Msg("failed to parse manifest time; assuming changes likely")
		return ChangeSignal{HasLikelyChanges: true}, nil
	}

	// 3. Read the project directory (non-recursive — only the project root).
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return ChangeSignal{}, fmt.Errorf("detecting changes in %s: %w", projectDir, err)
	}

	var modifiedFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Non-recursive: only the project root.
		}
		name := entry.Name()
		if !pulumiSourceFile(name) {
			continue
		}
		path := filepath.Join(projectDir, name)
		info, statErr := os.Stat(path)
		if statErr != nil {
			log.Debug().
				Ctx(ctx).
				Str("component", "changedetect").
				Str("operation", "stat_source_file").
				Str("file", path).
				Err(statErr).
				Msg("stat failed; skipping file")
			continue
		}
		if info.ModTime().After(lastDeployment) {
			modifiedFiles = append(modifiedFiles, name)
		}
	}

	if len(modifiedFiles) > 0 {
		return ChangeSignal{HasLikelyChanges: true, ModifiedFiles: modifiedFiles}, nil
	}
	return ChangeSignal{HasLikelyChanges: false}, nil
}

// pulumiSourceFile returns true if the filename matches known Pulumi source file patterns.
// This is intentionally a simple filename check (no path walking) — recursive scanning
// is out of scope and would add latency.
func pulumiSourceFile(name string) bool {
	// Exact filename matches for dependency and Go module files.
	switch name {
	case "go.mod", "go.sum",
		"requirements.txt", "Pipfile", "pyproject.toml",
		"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml":
		return true
	}

	// Pulumi.yaml, Pulumi.yml, and Pulumi.*.yaml / Pulumi.*.yml (stack configs).
	// Case-insensitive check handles mixed-case variants (e.g., "PULUMI.YAML").
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "pulumi.") &&
		(strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")) {
		return true
	}

	// Extension-based matches for source files.
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".ts", ".js", ".mts", ".mjs", // TypeScript / JavaScript
		".py",   // Python
		".go",   // Go
		".cs",   // C# (.NET)
		".fs",   // F# (.NET)
		".java": // Java
		return true
	}

	return false
}
