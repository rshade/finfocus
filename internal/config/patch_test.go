package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchKeepsCommentsAndUnknownKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hujson")
	original := []byte("{\n" +
		"  // keep me\n" +
		"  \"output\": { \"precision\": 2, \"default_format\": \"table\" },\n" +
		"  \"custom_unknown\": true\n" +
		"}\n")
	require.NoError(t, os.WriteFile(path, original, 0o600))
	version, err := FileVersion(path)
	require.NoError(t, err)

	preview, err := PatchFile(path, TierProject, version, []Change{{
		Key: "output.precision", Value: strPtr("4"),
	}}, true)
	require.NoError(t, err)
	assert.Equal(t, version, preview.Version)
	require.Len(t, preview.Diff, 1)
	assert.Equal(t, "2", preview.Diff[0].Before)
	assert.Equal(t, "4", preview.Diff[0].After)
	unchanged, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(unchanged))

	saved, err := PatchFile(path, TierProject, version, []Change{{
		Key: "output.precision", Value: strPtr("4"),
	}}, false)
	require.NoError(t, err)
	assert.NotEqual(t, version, saved.Version)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "// keep me")
	assert.Contains(t, string(data), "custom_unknown")
	assert.Contains(t, string(data), "default_format")
	assert.Contains(t, string(data), "4")
	again, err := FileVersion(path)
	require.NoError(t, err)
	assert.Equal(t, saved.Version, again)
	matches, err := filepath.Glob(filepath.Join(dir, ".config-*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestPatchRejectsInvalidAndStaleVersions(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	original := []byte("{\n  // keep me\n  \"output\": { \"precision\": 2 }\n}\n")
	require.NoError(t, os.WriteFile(path, original, 0o600))
	version, err := FileVersion(path)
	require.NoError(t, err)

	_, err = PatchFile(path, TierGlobal, version, []Change{{
		Key: "output.precision", Value: strPtr("11"),
	}}, false)
	var invalid *TierValidationError
	require.ErrorAs(t, err, &invalid)
	assert.False(t, invalid.Result.Valid)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(data))

	_, err = PatchFile(path, TierGlobal, "stale", []Change{{
		Key: "output.precision", Value: strPtr("3"),
	}}, false)
	require.ErrorIs(t, err, ErrStaleConfig)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(data))

	_, err = PatchFile(path, TierGlobal, version, []Change{{Key: "not.a.setting", Value: strPtr("x")}}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration section")
}

func TestPatchSecretsAndNoop(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	original := []byte(
		"{\n  \"plugins\": { \"aws\": { \"secret_key\": \"stored-secret\", \"region\": \"us-east-1\" } }\n}\n",
	)
	require.NoError(t, os.WriteFile(path, original, 0o600))
	version, err := FileVersion(path)
	require.NoError(t, err)

	redacted := RedactedValue
	result, err := PatchFile(path, TierGlobal, version, []Change{{
		Key: "plugins.aws.secret_key", Value: &redacted,
	}}, false)
	require.NoError(t, err)
	assert.Empty(t, result.Diff)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "stored-secret")
	assert.NotContains(t, string(data), RedactedValue)

	empty := ""
	_, err = PatchFile(path, TierGlobal, version, []Change{{
		Key: "plugins.aws.secret_key", Value: &empty,
	}}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")

	same := "us-east-1"
	result, err = PatchFile(path, TierGlobal, version, []Change{{
		Key: "plugins.aws.region", Value: &same,
	}}, false)
	require.NoError(t, err)
	assert.Empty(t, result.Diff)

	result, err = PatchFile(path, TierGlobal, version, []Change{{
		Key: "plugins.aws.secret_key", Value: strPtr("next-secret"),
	}}, true)
	require.NoError(t, err)
	require.Len(t, result.Diff, 1)
	assert.Equal(t, "set", result.Diff[0].Before)
	assert.Equal(t, "replaced", result.Diff[0].After)
	assert.NotContains(t, result.Diff[0].Before+result.Diff[0].After, "stored-secret")
	assert.NotContains(t, result.Diff[0].Before+result.Diff[0].After, "next-secret")
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(data))
}

func TestPatchClearsAndMigratesYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("output:\n  precision: 2\n  default_format: table\n"), 0o600))
	hujsonPath := filepath.Join(dir, "config.hujson")
	version, err := FileVersion(PreferredConfigPath(hujsonPath))
	require.NoError(t, err)

	result, err := PatchFile(hujsonPath, TierGlobal, version, []Change{{
		Key: "output.precision", Value: strPtr("5"),
	}}, false)
	require.NoError(t, err)
	assert.NotEqual(t, AbsentVersion, result.Version)
	data, err := os.ReadFile(hujsonPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "5")
	assert.Contains(t, string(data), "table")

	cleared, err := PatchFile(hujsonPath, TierGlobal, result.Version, []Change{{
		Key: "output.precision", Clear: true,
	}}, false)
	require.NoError(t, err)
	require.Len(t, cleared.Diff, 1)
	assert.Equal(t, "not set", cleared.Diff[0].After)
	data, err = os.ReadFile(hujsonPath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "precision")
}

func TestPatchCreatesNestedBudgetAndSkipsMissingProject(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	result, err := PatchFile(path, TierGlobal, AbsentVersion, []Change{
		{Key: "cost.budgets.global.currency", Value: strPtr("USD")},
		{Key: "cost.budgets.amount", Value: strPtr("25")},
	}, false)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Regexp(t, `"amount":\s*25\b`, string(data))
	assert.NotContains(t, string(data), `"25"`)
	assert.Contains(t, string(data), "USD")
	assert.Equal(t, result.Version, mustVersion(t, path))

	_, err = PatchFile("", TierProject, AbsentVersion, nil, false)
	require.ErrorIs(t, err, ErrProjectUnavailable)
}

func TestPatchOverlappingSaves(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	first, err := PatchFile(path, TierGlobal, AbsentVersion, []Change{{
		Key: "output.default_format", Value: strPtr("json"),
	}}, false)
	require.NoError(t, err)
	second, err := PatchFile(path, TierGlobal, first.Version, []Change{{
		Key: "output.precision", Value: strPtr("3"),
	}}, false)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "json")
	assert.Contains(t, string(data), "3")

	var stale error
	var wrote int
	var mu sync.Mutex
	base := second.Version
	var group sync.WaitGroup
	for _, value := range []string{"4", "6"} {
		group.Add(1)
		go func() {
			defer group.Done()
			_, patchErr := PatchFile(path, TierGlobal, base, []Change{{
				Key: "output.precision", Value: strPtr(value),
			}}, false)
			mu.Lock()
			defer mu.Unlock()
			if patchErr == nil {
				wrote++
				return
			}
			stale = patchErr
		}()
	}
	group.Wait()
	assert.Equal(t, 1, wrote)
	require.ErrorIs(t, stale, ErrStaleConfig)
	final, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(final), "json")
}

func TestPatchPrecisionMustBeANumber(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	_, err := PatchFile(path, TierGlobal, AbsentVersion, []Change{{
		Key: "output.precision", Value: strPtr("nope"),
	}}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "precision must be a number")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func strPtr(value string) *string { return &value }

func mustVersion(t *testing.T, path string) string {
	t.Helper()
	version, err := FileVersion(path)
	require.NoError(t, err)
	return version
}
