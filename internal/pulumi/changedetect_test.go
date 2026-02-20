package pulumi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFileWithMtime creates a file in dir with the given name and sets its
// modification time to mtime. It fails the test on any error.
func writeFileWithMtime(t *testing.T, dir, name string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("test"), 0600))
	require.NoError(t, os.Chtimes(path, mtime, mtime))
}

func TestDetectChanges_NeverDeployed(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	signal, err := DetectChanges(ctx, "", tmpDir)
	require.NoError(t, err)
	assert.True(t, signal.HasLikelyChanges, "never-deployed stack should have likely changes")
	assert.True(t, signal.IsFirstDeploy, "never-deployed stack should be flagged as first deploy")
	assert.Empty(t, signal.ModifiedFiles, "first deploy should not have modified files list")
}

func TestDetectChanges_NoMatchingFiles(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Only unrecognised files in the directory.
	past := time.Now().Add(-2 * time.Hour)
	writeFileWithMtime(t, tmpDir, "README.md", past)
	writeFileWithMtime(t, tmpDir, ".gitignore", past)

	manifestTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	signal, err := DetectChanges(ctx, manifestTime, tmpDir)
	require.NoError(t, err)
	assert.False(t, signal.HasLikelyChanges, "no recognised source files → no likely changes")
	assert.False(t, signal.IsFirstDeploy)
	assert.Empty(t, signal.ModifiedFiles)
}

func TestDetectChanges_NoChanges(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	lastDeploy := time.Now().Add(-1 * time.Hour)
	manifestTime := lastDeploy.UTC().Format(time.RFC3339)

	// Source files older than last deployment.
	old := lastDeploy.Add(-30 * time.Minute)
	writeFileWithMtime(t, tmpDir, "Pulumi.yaml", old)
	writeFileWithMtime(t, tmpDir, "main.go", old)
	writeFileWithMtime(t, tmpDir, "go.mod", old)

	signal, err := DetectChanges(ctx, manifestTime, tmpDir)
	require.NoError(t, err)
	assert.False(t, signal.HasLikelyChanges, "all source files older than deployment → no likely changes")
	assert.Empty(t, signal.ModifiedFiles)
}

func TestDetectChanges_TypeScriptFileModified(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	lastDeploy := time.Now().Add(-1 * time.Hour)
	manifestTime := lastDeploy.UTC().Format(time.RFC3339)

	// Older files (no change).
	writeFileWithMtime(t, tmpDir, "Pulumi.yaml", lastDeploy.Add(-30*time.Minute))
	// Newer TypeScript file.
	writeFileWithMtime(t, tmpDir, "index.ts", lastDeploy.Add(10*time.Minute))

	signal, err := DetectChanges(ctx, manifestTime, tmpDir)
	require.NoError(t, err)
	assert.True(t, signal.HasLikelyChanges)
	assert.False(t, signal.IsFirstDeploy)
	assert.Contains(t, signal.ModifiedFiles, "index.ts")
}

func TestDetectChanges_GoModModified(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	lastDeploy := time.Now().Add(-1 * time.Hour)
	manifestTime := lastDeploy.UTC().Format(time.RFC3339)

	writeFileWithMtime(t, tmpDir, "go.mod", lastDeploy.Add(5*time.Minute))
	writeFileWithMtime(t, tmpDir, "main.go", lastDeploy.Add(-10*time.Minute))

	signal, err := DetectChanges(ctx, manifestTime, tmpDir)
	require.NoError(t, err)
	assert.True(t, signal.HasLikelyChanges)
	assert.Contains(t, signal.ModifiedFiles, "go.mod")
	assert.NotContains(t, signal.ModifiedFiles, "main.go")
}

func TestDetectChanges_PulumiYamlModified(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	lastDeploy := time.Now().Add(-1 * time.Hour)
	manifestTime := lastDeploy.UTC().Format(time.RFC3339)

	writeFileWithMtime(t, tmpDir, "Pulumi.yaml", lastDeploy.Add(2*time.Minute))

	signal, err := DetectChanges(ctx, manifestTime, tmpDir)
	require.NoError(t, err)
	assert.True(t, signal.HasLikelyChanges)
	assert.Contains(t, signal.ModifiedFiles, "Pulumi.yaml")
}

func TestDetectChanges_MalformedManifestTime(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	signal, err := DetectChanges(ctx, "not-a-timestamp", tmpDir)
	require.NoError(t, err, "malformed manifest time should return conservative result, not error")
	assert.True(t, signal.HasLikelyChanges, "malformed timestamp → conservative true")
	assert.False(t, signal.IsFirstDeploy, "malformed timestamp is not the same as never deployed")
}

func TestDetectChanges_InvalidProjectDir(t *testing.T) {
	ctx := context.Background()

	_, err := DetectChanges(ctx, time.Now().UTC().Format(time.RFC3339), "/nonexistent/path/that/does/not/exist")
	require.Error(t, err, "non-existent project dir should return error")
	assert.Contains(t, err.Error(), "detecting changes in")
}

func TestDetectChanges_StatErrorSkipsFile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	lastDeploy := time.Now().Add(-1 * time.Hour)
	manifestTime := lastDeploy.UTC().Format(time.RFC3339)

	// Create a real source file that is newer than the last deployment.
	newer := lastDeploy.Add(10 * time.Minute)
	writeFileWithMtime(t, tmpDir, "index.ts", newer)
	writeFileWithMtime(t, tmpDir, "Pulumi.yaml", lastDeploy.Add(-30*time.Minute))

	// Create a broken symlink: os.Stat (not os.Lstat) returns an error for broken symlinks,
	// so this exercises the stat-error skip branch inside DetectChanges.
	brokenTarget := filepath.Join(tmpDir, "nonexistent-target.ts")
	brokenLink := filepath.Join(tmpDir, "broken.ts")
	require.NoError(t, os.Symlink(brokenTarget, brokenLink))

	// Detection should skip the broken symlink and continue; index.ts is newer so
	// HasLikelyChanges must be true.
	signal, err := DetectChanges(ctx, manifestTime, tmpDir)
	require.NoError(t, err, "stat error on one file should not abort detection")
	assert.True(t, signal.HasLikelyChanges, "index.ts is newer so changes should be detected")
}

func TestDetectChanges_MultipleModifiedFiles(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	lastDeploy := time.Now().Add(-1 * time.Hour)
	manifestTime := lastDeploy.UTC().Format(time.RFC3339)

	newer := lastDeploy.Add(5 * time.Minute)
	writeFileWithMtime(t, tmpDir, "index.ts", newer)
	writeFileWithMtime(t, tmpDir, "Pulumi.yaml", newer)
	writeFileWithMtime(t, tmpDir, "go.mod", lastDeploy.Add(-10*time.Minute)) // older → not listed

	signal, err := DetectChanges(ctx, manifestTime, tmpDir)
	require.NoError(t, err)
	assert.True(t, signal.HasLikelyChanges)
	assert.Len(t, signal.ModifiedFiles, 2)
	assert.Contains(t, signal.ModifiedFiles, "index.ts")
	assert.Contains(t, signal.ModifiedFiles, "Pulumi.yaml")
	assert.NotContains(t, signal.ModifiedFiles, "go.mod")
}

// TestPulumiSourceFile_KnownExtensions exercises all seven pattern groups.
func TestPulumiSourceFile_KnownExtensions(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		// YAML stack configs
		{"Pulumi.yaml exact", "Pulumi.yaml", true},
		{"Pulumi.yml exact", "Pulumi.yml", true},
		{"Pulumi.dev.yaml stack config", "Pulumi.dev.yaml", true},
		{"Pulumi.prod.yml stack config", "Pulumi.prod.yml", true},
		// TypeScript / JavaScript
		{"TypeScript .ts", "index.ts", true},
		{"JavaScript .js", "index.js", true},
		{"Module TypeScript .mts", "index.mts", true},
		{"Module JavaScript .mjs", "index.mjs", true},
		// Python
		{"Python .py", "main.py", true},
		// Go
		{"Go source .go", "main.go", true},
		{"go.mod", "go.mod", true},
		{"go.sum", "go.sum", true},
		// Python deps
		{"requirements.txt", "requirements.txt", true},
		{"Pipfile", "Pipfile", true},
		{"pyproject.toml", "pyproject.toml", true},
		// Node deps
		{"package.json", "package.json", true},
		{"package-lock.json", "package-lock.json", true},
		{"yarn.lock", "yarn.lock", true},
		{"pnpm-lock.yaml", "pnpm-lock.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pulumiSourceFile(tt.filename))
		})
	}
}

// TestPulumiSourceFile_UnknownExtension verifies unrecognised files return false.
func TestPulumiSourceFile_UnknownExtension(t *testing.T) {
	unknown := []string{
		"README.md",
		".gitignore",
		"Makefile",
		"LICENSE",
		"data.json",
		"config.toml",
		"image.png",
		"notes.txt",
	}
	for _, name := range unknown {
		t.Run(name, func(t *testing.T) {
			assert.False(t, pulumiSourceFile(name), "should not match: %s", name)
		})
	}
}
