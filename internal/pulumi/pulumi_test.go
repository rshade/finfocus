package pulumi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner implements CommandRunner for testing.
type mockRunner struct {
	stdout []byte
	stderr []byte
	err    error
	// Captured call arguments for verification.
	lastDir  string
	lastName string
	lastArgs []string
}

func (m *mockRunner) Run(_ context.Context, dir string, name string, args ...string) ([]byte, []byte, error) {
	m.lastDir = dir
	m.lastName = name
	m.lastArgs = args
	return m.stdout, m.stderr, m.err
}

// withMockRunner replaces the package Runner with a mock and restores it on cleanup.
func withMockRunner(t *testing.T, m *mockRunner) {
	t.Helper()
	orig := Runner
	Runner = m
	t.Cleanup(func() { Runner = orig })
}

// --- T002: FindBinary tests ---

func TestFindBinary_Found(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// This test relies on the actual system PATH.
	path, err := FindBinary()
	if errors.Is(err, ErrPulumiNotFound) {
		t.Skip("pulumi not installed, skipping FindBinary found test")
	}
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestFindBinary_NotFound(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("PATH", t.TempDir())

	_, err := FindBinary()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPulumiNotFound)
	assert.Contains(t, err.Error(), pulumiInstallURL)
	assert.Contains(t, err.Error(), "--pulumi-json")
}

// --- T003: FindProject tests ---

func TestFindProject_YamlInCurrentDir(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: test\n"), 0644))

	result, err := FindProject(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, result)
}

func TestFindProject_YmlInCurrentDir(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yml"), []byte("name: test\n"), 0644))

	result, err := FindProject(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, result)
}

func TestFindProject_InParentDir(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	parent := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(parent, "Pulumi.yaml"), []byte("name: test\n"), 0644))

	child := filepath.Join(parent, "src", "infra")
	require.NoError(t, os.MkdirAll(child, 0755))

	result, err := FindProject(child)
	require.NoError(t, err)
	assert.Equal(t, parent, result)
}

func TestFindProject_NotFound(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	dir := t.TempDir()

	_, err := FindProject(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoProject)
}

// --- T033: Error message quality for ErrPulumiNotFound ---

func TestErrPulumiNotFound_ContainsInstallURL(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	assert.Contains(t, ErrPulumiNotFound.Error(), pulumiInstallURL)
	assert.Contains(t, ErrPulumiNotFound.Error(), "--pulumi-json")
}

// --- T034: Error message quality for ErrNoProject ---

func TestErrNoProject_SuggestsPulumiJson(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	assert.Contains(t, ErrNoProject.Error(), "--pulumi-json")
	assert.Contains(t, ErrNoProject.Error(), "no Pulumi project found")
}

func TestFindProject_NotFound_MessageQuality(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	dir := t.TempDir()

	_, err := FindProject(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--pulumi-json")
	assert.Contains(t, err.Error(), "no Pulumi project found")
}

// --- T035: Error message quality for ErrNoCurrentStack ---

func TestNoCurrentStackError_IncludesStackNames(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	err := NoCurrentStackError([]string{"dev", "staging", "prod"})
	assert.ErrorIs(t, err, ErrNoCurrentStack)
	assert.Contains(t, err.Error(), "--stack")
	assert.Contains(t, err.Error(), "dev")
	assert.Contains(t, err.Error(), "staging")
	assert.Contains(t, err.Error(), "prod")
}

func TestNoCurrentStackError_EmptyList(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	err := NoCurrentStackError([]string{})
	assert.ErrorIs(t, err, ErrNoCurrentStack)
	assert.Contains(t, err.Error(), "no stacks found")
}

// --- T036: Error message quality for Preview/Export failures ---

func TestPreviewError_IncludesStderr(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	err := PreviewError("error: missing provider credentials")
	assert.ErrorIs(t, err, ErrPreviewFailed)
	assert.Contains(t, err.Error(), "missing provider credentials")
}

func TestExportError_IncludesStderr(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	err := ExportError("error: stack 'prod' not found")
	assert.ErrorIs(t, err, ErrExportFailed)
	assert.Contains(t, err.Error(), "stack 'prod' not found")
}

func TestFindProject_PrefersYamlOverYml(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: yaml\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yml"), []byte("name: yml\n"), 0644))

	result, err := FindProject(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, result)
}

// --- T004: GetCurrentStack tests ---

func TestGetCurrentStack_CurrentFound(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	stackJSON := `[{"name":"dev","current":true,"url":"file://~"},` +
		`{"name":"prod","current":false,"url":"file://~"}]`
	mock := &mockRunner{
		stdout: []byte(stackJSON),
	}
	withMockRunner(t, mock)

	stack, err := GetCurrentStack(context.Background(), "/project")
	require.NoError(t, err)
	assert.Equal(t, "dev", stack)
	assert.Equal(t, "/project", mock.lastDir)
}

func TestGetCurrentStack_NoCurrent(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	stackJSON := `[{"name":"dev","current":false,"url":"file://~"},` +
		`{"name":"staging","current":false,"url":"file://~"}]`
	mock := &mockRunner{
		stdout: []byte(stackJSON),
	}
	withMockRunner(t, mock)

	_, err := GetCurrentStack(context.Background(), "/project")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoCurrentStack)
	assert.Contains(t, err.Error(), "dev")
	assert.Contains(t, err.Error(), "staging")
	assert.Contains(t, err.Error(), "--stack")
}

func TestGetCurrentStack_EmptyList(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stdout: []byte(`[]`),
	}
	withMockRunner(t, mock)

	_, err := GetCurrentStack(context.Background(), "/project")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoCurrentStack)
	assert.Contains(t, err.Error(), "no stacks found")
}

func TestGetCurrentStack_MalformedJSON(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stdout: []byte(`{not valid json`),
	}
	withMockRunner(t, mock)

	_, err := GetCurrentStack(context.Background(), "/project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing pulumi stack ls output")
}

func TestGetCurrentStack_CommandError(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stderr: []byte("access denied"),
		err:    errors.New("exit status 1"),
	}
	withMockRunner(t, mock)

	_, err := GetCurrentStack(context.Background(), "/project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running pulumi stack ls")
	assert.Contains(t, err.Error(), "access denied")
}

// --- T005: Preview tests ---

func TestPreview_Success(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	expected := `{"steps":[{"op":"create","urn":"urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket"}]}`
	mock := &mockRunner{
		stdout: []byte(expected),
	}
	withMockRunner(t, mock)

	data, err := Preview(context.Background(), PreviewOptions{
		ProjectDir: "/project",
	})
	require.NoError(t, err)
	assert.Equal(t, expected, string(data))
	assert.Contains(t, mock.lastArgs, "--json")
}

func TestPreview_WithStack(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stdout: []byte(`{"steps":[]}`),
	}
	withMockRunner(t, mock)

	_, err := Preview(context.Background(), PreviewOptions{
		ProjectDir: "/project",
		Stack:      "production",
	})
	require.NoError(t, err)
	assert.Contains(t, mock.lastArgs, "--stack")
	assert.Contains(t, mock.lastArgs, "production")
}

func TestPreview_Failure(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stderr: []byte("error: program failed to compile"),
		err:    errors.New("exit status 1"),
	}
	withMockRunner(t, mock)

	_, err := Preview(context.Background(), PreviewOptions{
		ProjectDir: "/project",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPreviewFailed)
	assert.Contains(t, err.Error(), "program failed to compile")
}

func TestPreview_ContextCancellation(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		err: context.DeadlineExceeded,
	}
	withMockRunner(t, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Wait for context to expire.
	time.Sleep(60 * time.Millisecond)

	_, err := Preview(ctx, PreviewOptions{
		ProjectDir: "/project",
		Timeout:    100 * time.Millisecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestPreview_DefaultTimeout(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stdout: []byte(`{"steps":[]}`),
	}
	withMockRunner(t, mock)

	_, err := Preview(context.Background(), PreviewOptions{
		ProjectDir: "/project",
	})
	require.NoError(t, err)
}

// --- T006: StackExport tests ---

func TestStackExport_Success(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	expected := `{"version":3,"deployment":{"manifest":{},"resources":[]}}`
	mock := &mockRunner{
		stdout: []byte(expected),
	}
	withMockRunner(t, mock)

	data, err := StackExport(context.Background(), ExportOptions{
		ProjectDir: "/project",
	})
	require.NoError(t, err)
	assert.Equal(t, expected, string(data))
	assert.Contains(t, mock.lastArgs, "export")
}

func TestStackExport_WithStack(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stdout: []byte(`{"version":3}`),
	}
	withMockRunner(t, mock)

	_, err := StackExport(context.Background(), ExportOptions{
		ProjectDir: "/project",
		Stack:      "staging",
	})
	require.NoError(t, err)
	assert.Contains(t, mock.lastArgs, "--stack")
	assert.Contains(t, mock.lastArgs, "staging")
}

func TestStackExport_Failure(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		stderr: []byte("error: stack not found"),
		err:    errors.New("exit status 1"),
	}
	withMockRunner(t, mock)

	_, err := StackExport(context.Background(), ExportOptions{
		ProjectDir: "/project",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExportFailed)
	assert.Contains(t, err.Error(), "stack not found")
}

func TestStackExport_ContextCancellation(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{
		err: context.DeadlineExceeded,
	}
	withMockRunner(t, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	time.Sleep(60 * time.Millisecond)

	_, err := StackExport(ctx, ExportOptions{
		ProjectDir: "/project",
		Timeout:    100 * time.Millisecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}
