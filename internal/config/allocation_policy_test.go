package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePolicy(t *testing.T, dir, body string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o750))
	p := filepath.Join(dir, AllocationPolicyFile)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func isolatePolicyDirs(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	project := filepath.Join(t.TempDir(), ".finfocus")
	SetResolvedProjectDir("")
	t.Cleanup(func() { SetResolvedProjectDir("") })
	return home, project
}

//nolint:paralleltest // isolatePolicyDirs uses t.Setenv and the process-wide resolved project dir
func TestResolveAllocationPolicy_Precedence(t *testing.T) {
	home, project := isolatePolicyDirs(t)
	ctx := context.Background()

	got, err := ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Nil(t, got.JSON, "no file → plugin defaults")
	assert.Empty(t, got.Source)

	globalPath := writePolicy(t, home, `{ "version": 1, /* global */ }`)
	got, err = ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, globalPath, got.Source)
	assert.JSONEq(t, `{"version":1}`, string(got.JSON), "hujson comments and trailing commas standardized")

	projectPath := writePolicy(t, project, `{"idle": "separate"}`)
	SetResolvedProjectDir(project)
	got, err = ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, projectPath, got.Source)
	assert.JSONEq(t, `{"idle":"separate"}`, string(got.JSON), "project replaces global, no merge")

	flagPath := writePolicy(t, t.TempDir(), `{"version": 1}`)
	got, err = ResolveAllocationPolicy(ctx, flagPath)
	require.NoError(t, err)
	assert.Equal(t, flagPath, got.Source)
}

//nolint:paralleltest // isolatePolicyDirs uses t.Setenv and the process-wide resolved project dir
func TestResolveAllocationPolicy_NullDocument(t *testing.T) {
	home, _ := isolatePolicyDirs(t)

	path := writePolicy(t, home, "null\n")
	got, err := ResolveAllocationPolicy(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, path, got.Source)
	assert.Nil(t, got.JSON, "a null document is equivalent to no policy")
}

//nolint:paralleltest // isolatePolicyDirs uses t.Setenv and the process-wide resolved project dir
func TestResolveAllocationPolicy_Errors(t *testing.T) {
	home, _ := isolatePolicyDirs(t)
	ctx := context.Background()

	_, err := ResolveAllocationPolicy(ctx, filepath.Join(t.TempDir(), "missing.hujson"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.hujson")

	broken := writePolicy(t, home, `{"version": }`)
	_, err = ResolveAllocationPolicy(ctx, "")
	require.Error(t, err, "a discovered broken file never falls back to defaults")
	assert.Contains(t, err.Error(), broken)

	commentOnly := writePolicy(t, t.TempDir(), "// nothing yet\n")
	_, err = ResolveAllocationPolicy(ctx, commentOnly)
	require.Error(t, err, "a comment-only file is a parse error, not an empty policy")
	assert.Contains(t, err.Error(), commentOnly)
}

//nolint:paralleltest // isolatePolicyDirs uses t.Setenv and the process-wide resolved project dir
func TestResolveAllocationPolicy_ParseErrorNamesFile(t *testing.T) {
	tests := []struct {
		name string
		// place writes body and returns its path and the --policy value.
		place      func(t *testing.T, home, project, body string) (string, string)
		body       string
		wantLine   int
		wantColumn int
		wantReason string
		hidden     string
	}{
		{
			name: "truncated --policy file",
			place: func(t *testing.T, _, _, body string) (string, string) {
				t.Helper()
				p := writePolicy(t, t.TempDir(), body)
				return p, p
			},
			body:       `{ "idle": `,
			wantLine:   1,
			wantColumn: 11,
			wantReason: "unexpected EOF",
		},
		{
			name: "malformed discovered project file",
			place: func(t *testing.T, _, project, body string) (string, string) {
				t.Helper()
				SetResolvedProjectDir(project)
				return writePolicy(t, project, body), ""
			},
			body:       "{\n  \"idle\": \"share\"\n  \"version\": 1\n}\n",
			wantLine:   3,
			wantColumn: 3,
			wantReason: `invalid character '"'`,
		},
		{
			name: "malformed discovered global file",
			place: func(t *testing.T, home, _, body string) (string, string) {
				t.Helper()
				return writePolicy(t, home, body), ""
			},
			body:       "{\n  \"idle\": sekrit-value\n}\n",
			wantLine:   2,
			wantColumn: 11,
			wantReason: "invalid literal",
			hidden:     "sekrit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, project := isolatePolicyDirs(t)
			path, flag := tt.place(t, home, project, tt.body)

			_, err := ResolveAllocationPolicy(context.Background(), flag)
			require.Error(t, err)
			axErr, ok := errors.AsType[*ax.Error](err)
			require.True(t, ok, "want an *ax.Error, got %T", err)
			assert.Equal(t, "config_invalid", axErr.ErrorCode)
			assert.Equal(t, ax.ExitValidation, axErr.ExitCode())
			assert.Contains(t, axErr.Message, "allocation policy "+path)
			assert.Contains(t, axErr.Message, fmt.Sprintf("line %d, column %d", tt.wantLine, tt.wantColumn))
			assert.Contains(t, axErr.Message, tt.wantReason)
			assert.NotContains(t, axErr.Message, "config is not valid")
			assert.Contains(t, axErr.ActionableFix, path)
			assert.NotContains(t, axErr.ActionableFix, "config syntax")
			assert.Equal(t, path, axErr.Context["path"])
			assert.Equal(t, tt.wantLine, axErr.Context["line"])
			assert.Equal(t, tt.wantColumn, axErr.Context["column"])
			if tt.hidden != "" {
				assert.NotContains(t, axErr.Message, tt.hidden, "file contents never reach the message")
				assert.NotContains(t, axErr.ActionableFix, tt.hidden)
			}
		})
	}
}

//nolint:paralleltest // isolatePolicyDirs uses t.Setenv and the process-wide resolved project dir
func TestResolveAllocationPolicy_TooLargeNamesFile(t *testing.T) {
	isolatePolicyDirs(t)
	path := writePolicy(t, t.TempDir(), `{"idle": "`+strings.Repeat("x", int(ax.DefaultMaxConfigBytes))+`"}`)

	_, err := ResolveAllocationPolicy(context.Background(), path)
	require.Error(t, err)
	axErr, ok := errors.AsType[*ax.Error](err)
	require.True(t, ok, "want an *ax.Error, got %T", err)
	assert.Equal(t, "config_too_large", axErr.ErrorCode)
	assert.Equal(t, ax.ExitValidation, axErr.ExitCode())
	assert.Contains(t, axErr.Message, "allocation policy "+path)
	assert.Contains(t, axErr.ActionableFix, path)
	assert.NotContains(t, axErr.ActionableFix, "WithMaxConfigBytes")
	assert.Equal(t, path, axErr.Context["path"])
}

//nolint:paralleltest // isolatePolicyDirs uses t.Setenv and the process-wide resolved project dir
func TestResolveAllocationPolicy_UnreadableNamesFile(t *testing.T) {
	isolatePolicyDirs(t)
	ctx := context.Background()

	dir := t.TempDir()
	_, err := ResolveAllocationPolicy(ctx, dir)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "cannot read allocation policy "+dir+": "), err.Error())
	assert.Equal(t, 1, strings.Count(err.Error(), dir), "the path is named once: %s", err)

	missing := filepath.Join(t.TempDir(), "missing.hujson")
	_, err = ResolveAllocationPolicy(ctx, missing)
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
	assert.True(t, strings.HasPrefix(err.Error(), "cannot read allocation policy "+missing+": "), err.Error())
	assert.Equal(t, 1, strings.Count(err.Error(), missing), "the path is named once: %s", err)

	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file mode bits do not deny reads here")
	}
	locked := writePolicy(t, t.TempDir(), `{"idle": "share"}`)
	require.NoError(t, os.Chmod(locked, 0))
	_, err = ResolveAllocationPolicy(ctx, locked)
	require.Error(t, err)
	assert.Equal(t, "cannot read allocation policy "+locked+": permission denied", err.Error())
}
