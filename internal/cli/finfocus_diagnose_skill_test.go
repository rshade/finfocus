package cli

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/constants"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/registry"
)

func TestFinfocusDiagnoseSkillDocumentsLiveContract(t *testing.T) {
	t.Parallel()

	docs := readDiagnoseSkill(t)
	assertDiagnoseTriggers(t, diagnoseSkillDir(t))
	assertDiagnoseCommands(t, docs)
	assertDiagnoseSentinels(t, docs)
	assertDiagnoseCacheKeys(t, docs)

	for _, name := range []string{
		config.CacheEnvTTLSeconds,
		config.CacheEnvTTLSecondsLegacy,
		config.CacheEnvEnabled,
		config.CacheEnvDir,
		config.CacheEnvMaxSize,
		constants.EnvAnalyzerMode,
		pluginsdk.EnvLogLevel,
		pluginsdk.EnvLogFormat,
		pluginsdk.EnvLogFile,
		pluginsdk.EnvTraceID,
		pluginsdk.EnvPort,
	} {
		assert.Contains(t, docs, name)
	}
	assert.Contains(t, docs, strconv.Itoa(config.CacheDefaultTTLSeconds))
	assert.Contains(t, docs, strconv.Itoa(config.CacheDefaultMaxSizeMB))
	assert.Contains(t, docs, strconv.Itoa(cache.MinTTLSeconds))
	assert.Contains(t, docs, strconv.Itoa(cache.MaxTTLSeconds))
	assert.Contains(t, docs, cache.ErrInvalidTTL.Error())
	assert.Contains(t, docs, pluginhost.TraceIDMetadataKey)
	assert.Contains(t, docs, pluginsdk.DefaultSupportsNotImplementedReason)
	assert.Contains(t, docs, " (cached)")
	assert.Contains(t, docs, "The config file is `config.hujson`.")
	assert.Contains(t, docs, "A legacy `config.yaml` is migrated on read.")
	assert.Contains(t, docs, "There is no `finfocus config path`.")
	assert.Contains(t, docs, "FINFOCUS_CONFIG_STRICT` is `true` or `1`.")
	assert.Contains(t, docs, "FINFOCUS_ANALYZER_MODE` is read only when the value is `true`.")
	assert.NotContains(t, docs, "export FINFOCUS_BUDGET_AMOUNT")
}

func TestFinfocusDiagnoseSkillArchive(t *testing.T) {
	t.Parallel()

	archive, err := zip.OpenReader(filepath.Join(diagnoseSkillDir(t), "finfocus-diagnose.skill"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, archive.Close())
	})

	dir := diagnoseSkillDir(t)
	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
		assert.NotContains(t, file.Name, ".skill")
		assert.Positive(t, file.UncompressedSize64)
		rel := strings.TrimPrefix(file.Name, "finfocus-diagnose/")
		packed, readErr := file.Open()
		require.NoError(t, readErr)
		got, readErr := io.ReadAll(packed)
		require.NoError(t, packed.Close())
		require.NoError(t, readErr)
		want, readErr := os.ReadFile(filepath.Join(dir, rel))
		require.NoError(t, readErr)
		assert.Equal(t, string(want), string(got), file.Name)
	}
	for _, name := range []string{
		"finfocus-diagnose/SKILL.md",
		"finfocus-diagnose/references/diagnostic-commands.md",
		"finfocus-diagnose/references/error-codes.md",
		"finfocus-diagnose/references/decision-trees.md",
	} {
		assert.Truef(t, names[name], "archive missing %s", name)
	}
}

func assertDiagnoseTriggers(t *testing.T, dir string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	require.NoError(t, err)
	parts := strings.Split(string(body), "---")
	require.GreaterOrEqual(t, len(parts), 3)
	front := parts[1]
	for _, phrase := range []string{
		"diagnose",
		"debug",
		"connectivity",
		"zero cost",
		"cache issue",
		"plugin error",
		"config resolution",
		"PLUGIN_ERROR",
		"VALIDATION_ERROR",
		"TIMEOUT_ERROR",
		"NO_COST_DATA",
	} {
		assert.Contains(t, front, phrase)
	}
}

func assertDiagnoseCommands(t *testing.T, docs string) {
	t.Helper()
	list := NewPluginListCmd()
	verbose := list.Flags().Lookup("verbose")
	require.NotNil(t, verbose)
	assert.Equal(t, "false", verbose.DefValue)
	assert.Contains(t, docs, verbose.Usage)
	assert.Contains(t, docs, "finfocus plugin "+list.Name())

	validate := NewPluginValidateCmd()
	pluginFlag := validate.Flags().Lookup("plugin")
	require.NotNil(t, pluginFlag)
	assert.Empty(t, pluginFlag.DefValue)
	assert.Contains(t, docs, "--"+pluginFlag.Name)
	assert.Contains(t, docs, "finfocus plugin "+validate.Name())

	routes := NewConfigRoutesCmd()
	require.Equal(t, "routes", routes.Name())
	var listCmdName, testUse string
	for _, sub := range routes.Commands() {
		switch sub.Name() {
		case "list":
			listCmdName = sub.Name()
		case "test":
			testUse = sub.Use
		}
	}
	require.NotEmpty(t, listCmdName)
	require.NotEmpty(t, testUse)
	assert.Contains(t, docs, "finfocus config routes "+listCmdName)
	assert.Contains(t, docs, "finfocus config routes "+testUse)
	assert.Contains(t, docs, "No routing configured (automatic mode)")

	cost := newCostCmd()
	var sawProjected, sawActual bool
	for _, sub := range cost.Commands() {
		switch sub.Name() {
		case "projected":
			sawProjected = true
			flag := sub.Flags().Lookup("pulumi-json")
			require.NotNil(t, flag)
			assert.Contains(t, docs, "--"+flag.Name)
			assert.Contains(t, docs, "finfocus cost projected")
		case "actual":
			sawActual = true
			flag := sub.Flags().Lookup("fallback-estimate")
			require.NotNil(t, flag)
			assert.Equal(t, "false", flag.DefValue)
			assert.Contains(t, docs, "--"+flag.Name)
		}
	}
	assert.True(t, sawProjected, "cost projected is a real subcommand")
	assert.True(t, sawActual, "cost actual is a real subcommand")
}

func assertDiagnoseSentinels(t *testing.T, docs string) {
	t.Helper()
	for _, code := range []string{
		engine.ErrCodePluginError,
		engine.ErrCodeValidationError,
		engine.ErrCodeTimeoutError,
		engine.ErrCodeNoCostData,
	} {
		assert.Contains(t, docs, code)
	}
	for _, sentinel := range []error{
		engine.ErrNoCostData,
		engine.ErrMixedCurrencies,
		engine.ErrInvalidGroupBy,
		engine.ErrEmptyResults,
		engine.ErrInvalidDateRange,
		engine.ErrResourceValidation,
		config.ErrConfigCorrupted,
		pluginhost.ErrPluginIncompatible,
		cache.ErrCacheNotFound,
		cache.ErrCacheExpired,
		cache.ErrInvalidCacheKey,
		cache.ErrInvalidCacheTTL,
		cache.ErrCacheDisabled,
		cache.ErrCacheLocked,
		ErrPluginValidationFailed,
	} {
		assert.Contains(t, docs, sentinel.Error())
	}

	missing := filepath.Join(t.TempDir(), "missing-bin")
	err := ValidatePlugin(context.Background(), registry.PluginInfo{Path: missing, Name: "demo", Version: "1.0.0"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin binary not found:")
	assert.Contains(t, docs, "plugin binary not found:")

	dir := t.TempDir()
	bin := filepath.Join(dir, "demo")
	require.NoError(t, os.WriteFile(bin, []byte("not-a-script"), 0o644))
	err = ValidatePlugin(context.Background(), registry.PluginInfo{Path: bin, Name: "demo", Version: "1.0.0"})
	require.EqualError(t, err, "plugin binary is not executable")
	assert.Contains(t, docs, err.Error())

	require.NoError(t, os.Chmod(bin, 0o755))
	manifest := []byte(`{"name":"other","version":"1.0.0"}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.manifest.json"), manifest, 0o644))
	err = ValidatePlugin(context.Background(), registry.PluginInfo{Path: bin, Name: "demo", Version: "1.0.0"})
	require.EqualError(t, err, "manifest name mismatch: expected demo, got other")
	assert.Contains(t, docs, err.Error())

	manifest = []byte(`{"name":"demo","version":"9.9.9"}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.manifest.json"), manifest, 0o644))
	err = ValidatePlugin(context.Background(), registry.PluginInfo{Path: bin, Name: "demo", Version: "1.0.0"})
	require.EqualError(t, err, "manifest version mismatch: expected 1.0.0, got 9.9.9")
	assert.Contains(t, docs, err.Error())

	err = ValidatePlugin(context.Background(), registry.PluginInfo{Path: t.TempDir(), Name: "demo", Version: "1.0.0"})
	require.EqualError(t, err, "plugin path is a directory, not a binary")
	assert.Contains(t, docs, err.Error())
	assert.Contains(t, docs, "plugin binary is not executable (Windows requires .exe extension)")
}

func assertDiagnoseCacheKeys(t *testing.T, docs string) {
	t.Helper()
	for _, bucket := range []string{
		cache.BucketProjected,
		cache.BucketActual,
		cache.BucketRecommendations,
		cache.BucketResolveTypes,
		cache.BucketScores,
	} {
		assert.Contains(t, docs, bucket)
	}
	assert.Contains(t, docs, cache.BuildProjectedKey("aws", "aws:ec2/instance:Instance", "us-east-1", "t3.micro"))
	assert.Contains(t, docs, cache.BuildProjectedKey("", "", "", ""))
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	assert.Contains(t, docs, cache.BuildActualKey("aws", []string{"ec2", "s3"}, from, to, nil))
	assert.Contains(t, docs, cache.BuildRecommendationsKey([]string{"b", "a"}, "abc"))
}

func readDiagnoseSkill(t *testing.T) string {
	t.Helper()
	return readMarkdownTree(t, diagnoseSkillDir(t))
}

func diagnoseSkillDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "agent-skills", "finfocus-diagnose")
}

func readMarkdownTree(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		b.Write(body)
		b.WriteByte('\n')
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, b.String())
	return b.String()
}
