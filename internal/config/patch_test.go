package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tailscale/hujson"
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

	result, err = PatchFile(path, TierGlobal, version, []Change{{
		Key: "plugins.aws.region", Value: strPtr("eu-west-1"), Secret: true,
	}}, true)
	require.NoError(t, err)
	require.Len(t, result.Diff, 1)
	assert.Equal(t, "set", result.Diff[0].Before)
	assert.Equal(t, "replaced", result.Diff[0].After)
	assert.NotContains(t, result.Diff[0].Before+result.Diff[0].After, "us-east-1")
	assert.NotContains(t, result.Diff[0].Before+result.Diff[0].After, "eu-west-1")
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

func TestPatchPreviewRedactsNestedCredentialsInBefore(t *testing.T) {
	t.Parallel()
	const secret = "stored-secret"
	const endpoint = "https://example.test"
	const stored = `{"plugins":{"aws":{` +
		`"auth":{"profile":"default","token":"` + secret + `"},` +
		`"endpoints":[{"url":"` + endpoint + `","api_key":"` + secret + `"}]}}}`
	tests := []struct {
		name   string
		key    string
		change Change
		keep   string
	}{
		{"remove object", "plugins.aws.auth", Change{Key: "plugins.aws.auth", Clear: true}, "default"},
		{"remove array", "plugins.aws.endpoints", Change{Key: "plugins.aws.endpoints", Clear: true}, endpoint},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.hujson")
			require.NoError(t, os.WriteFile(path, []byte(stored), 0o600))
			version, err := FileVersion(path)
			require.NoError(t, err)

			result, err := PatchFile(path, TierGlobal, version, []Change{tt.change}, true)
			require.NoError(t, err)
			require.Len(t, result.Diff, 1)
			assert.Equal(t, tt.key, result.Diff[0].Key)
			assert.NotContains(t, result.Diff[0].Before, secret)
			assert.Contains(t, result.Diff[0].Before, tt.keep)
		})
	}
}

func readStandardJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	standard, err := hujson.Standardize(data)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(standard, &decoded))
	return decoded
}

func TestPatchKeepsStoredPluginScalarTypes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	require.NoError(t, os.WriteFile(path, []byte(
		`{"plugins":{"aws":{"max_retries":3,"enabled":true,"region":"us-east-1"}}}`), 0o600))
	version, err := FileVersion(path)
	require.NoError(t, err)

	_, err = PatchFile(path, TierGlobal, version, []Change{
		{Key: "plugins.aws.max_retries", Value: strPtr("5")},
		{Key: "plugins.aws.enabled", Value: strPtr("false")},
		{Key: "plugins.aws.region", Value: strPtr("eu-west-1")},
	}, false)
	require.NoError(t, err)

	aws := readStandardJSON(t, path)["plugins"].(map[string]any)["aws"].(map[string]any)
	assert.InDelta(t, 5, aws["max_retries"], 0)
	assert.Equal(t, false, aws["enabled"])
	assert.Equal(t, "eu-west-1", aws["region"])

	version, err = FileVersion(path)
	require.NoError(t, err)
	_, err = PatchFile(path, TierGlobal, version, []Change{
		{Key: "plugins.aws.max_retries", Value: strPtr("many")},
	}, false)
	var invalid *TierValidationError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "plugins.aws.max_retries", invalid.Result.Errors[0].Path)
}

func TestPatchRejectsReplacingNestedPluginValues(t *testing.T) {
	t.Parallel()
	const stored = `{"plugins":{"aws":{` +
		`"auth":{"profile":"default","token":"stored-secret"},` +
		`"endpoints":["https://example.test"]}}}`
	for _, key := range []string{"plugins.aws.auth", "plugins.aws.endpoints"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.hujson")
			require.NoError(t, os.WriteFile(path, []byte(stored), 0o600))
			version, err := FileVersion(path)
			require.NoError(t, err)

			_, err = PatchFile(path, TierGlobal, version, []Change{{Key: key, Value: strPtr("x")}}, false)
			var invalid *InvalidChangeError
			require.ErrorAs(t, err, &invalid)
			assert.Contains(t, invalid.Error(), key)
			onDisk, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.JSONEq(t, stored, string(onDisk))
		})
	}
}

func TestPatchWritesThroughSymlinkedConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "config.hujson")
	link := filepath.Join(dir, "home", "config.hujson")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
	require.NoError(t, os.WriteFile(target, []byte(`{"output":{"precision":2}}`), 0o600))
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	version, err := FileVersion(link)
	require.NoError(t, err)

	_, err = PatchFile(link, TierGlobal, version, []Change{{Key: "output.precision", Value: strPtr("4")}}, false)
	require.NoError(t, err)

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the link must survive the save")
	output := readStandardJSON(t, target)["output"].(map[string]any)
	assert.InDelta(t, 4, output["precision"], 0)
}

func TestPatchRefusesSymlinkedProjectConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside", "notes.json")
	link := filepath.Join(dir, "project", "config.hujson")
	require.NoError(t, os.MkdirAll(filepath.Dir(outside), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
	const original = `{"output":{"precision":2}}`
	require.NoError(t, os.WriteFile(outside, []byte(original), 0o600))
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	version, err := FileVersion(link)
	require.NoError(t, err)

	for _, dryRun := range []bool{true, false} {
		result, patchErr := PatchFile(link, TierProject, version, []Change{
			{Key: "output.precision", Value: strPtr("4")},
		}, dryRun)
		var invalid *InvalidChangeError
		require.ErrorAs(t, patchErr, &invalid)
		assert.Empty(t, result.Diff, "nothing read through the link is returned")
	}

	untouched, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.JSONEq(t, original, string(untouched))
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the link is left alone")
}

func TestPatchSecretScalarFailuresDoNotEchoTheValue(t *testing.T) {
	t.Parallel()
	const submitted = "replacement-secret"
	for name, stored := range map[string]string{"number": "123456", "boolean": "true"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.hujson")
			require.NoError(t, os.WriteFile(path, []byte(`{"plugins":{"aws":{"api_key":`+stored+`}}}`), 0o600))
			version, err := FileVersion(path)
			require.NoError(t, err)

			result, err := PatchFile(path, TierGlobal, version, []Change{{
				Key: "plugins.aws.api_key", Value: strPtr(submitted),
			}}, true)
			var invalid *TierValidationError
			require.ErrorAs(t, err, &invalid)
			assert.NotContains(t, err.Error(), submitted)
			for _, validation := range append(invalid.Result.Errors, result.Validation.Errors...) {
				assert.NotContains(t, validation.Message, submitted)
			}
		})
	}
}

func TestPatchKeepsLargePluginIntegersExact(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	require.NoError(t, os.WriteFile(path, []byte(`{"plugins":{"aws":{"max_retries":3}}}`), 0o600))
	version, err := FileVersion(path)
	require.NoError(t, err)

	_, err = PatchFile(path, TierGlobal, version, []Change{{
		Key: "plugins.aws.max_retries", Value: strPtr("9007199254740993"),
	}}, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "9007199254740993")

	version, err = FileVersion(path)
	require.NoError(t, err)
	for _, bad := range []string{"NaN", "0x10", "1e", "+5", " 5"} {
		_, err = PatchFile(path, TierGlobal, version, []Change{{
			Key: "plugins.aws.max_retries", Value: strPtr(bad),
		}}, true)
		var invalid *TierValidationError
		require.ErrorAs(t, err, &invalid, bad)
	}
}

func TestPatchSerializesSavesThroughASymlinkAndItsTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "config.hujson")
	link := filepath.Join(dir, "home", "config.hujson")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
	require.NoError(t, os.WriteFile(target, []byte(`{"output":{"precision":2}}`), 0o600))
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	base, err := FileVersion(target)
	require.NoError(t, err)

	var stale error
	var wrote int
	var mu sync.Mutex
	var group sync.WaitGroup
	for _, entry := range []struct{ path, value string }{{link, "4"}, {target, "6"}} {
		group.Add(1)
		go func() {
			defer group.Done()
			_, patchErr := PatchFile(entry.path, TierGlobal, base, []Change{{
				Key: "output.precision", Value: strPtr(entry.value),
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
	assert.Equal(t, 1, wrote, "the second save must see the first and report a conflict")
	require.ErrorIs(t, stale, ErrStaleConfig)
}
