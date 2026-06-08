package analyzer

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPolicyPackDir_Pass(t *testing.T) {
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)

	ppDir := filepath.Join(finfocusHome, policyPackDirName)
	require.NoError(t, os.MkdirAll(ppDir, 0o755))

	result, resolvedDir := checkPolicyPackDir()

	assert.Equal(t, "policy_pack_dir", result.Name)
	assert.Equal(t, "pass", result.Status)
	assert.Contains(t, result.Message, ppDir)
	assert.Empty(t, result.Remediation)
	assert.Equal(t, ppDir, resolvedDir)
}

func TestCheckPolicyPackDir_Fail(t *testing.T) {
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)

	result, resolvedDir := checkPolicyPackDir()

	assert.Equal(t, "policy_pack_dir", result.Name)
	assert.Equal(t, "fail", result.Status)
	assert.Contains(t, result.Message, "not found")
	assert.Contains(t, result.Remediation, "finfocus analyzer install")
	assert.Empty(t, resolvedDir)
}

func TestCheckPulumiPolicyYAML_Pass(t *testing.T) {
	ppDir := t.TempDir()
	require.NoError(t, WritePulumiPolicyYAML(ppDir))

	result := checkPulumiPolicyYAML(ppDir)

	assert.Equal(t, "pulumi_policy_yaml", result.Name)
	assert.Equal(t, "pass", result.Status)
	assert.Contains(t, result.Message, pulumiPolicyFilename)
	assert.Empty(t, result.Remediation)
}

func TestCheckPulumiPolicyYAML_Fail(t *testing.T) {
	ppDir := t.TempDir()
	content := []byte("name: finfocus\nruntime: wrong-runtime\ndescription: test\n")
	require.NoError(t, os.WriteFile(filepath.Join(ppDir, pulumiPolicyFilename), content, 0o600))

	result := checkPulumiPolicyYAML(ppDir)

	assert.Equal(t, "pulumi_policy_yaml", result.Name)
	assert.Equal(t, "fail", result.Status)
	assert.Contains(t, result.Message, "runtime")
	assert.Contains(t, result.Remediation, "runtime: finfocus")
}

func TestCheckBinaryInPATH_Fail(t *testing.T) {
	emptyPath := t.TempDir()
	t.Setenv("PATH", emptyPath)
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	result := checkBinaryInPATH()

	assert.Equal(t, "binary_in_path", result.Name)
	assert.Equal(t, "fail", result.Status)
	assert.Contains(t, result.Message, policyPackBinaryName)
	// Remediation is shell-specific: PowerShell syntax on Windows, POSIX export elsewhere
	if runtime.GOOS == "windows" {
		assert.Contains(t, result.Remediation, "$env:PATH")
	} else {
		assert.Contains(t, result.Remediation, "export PATH=")
	}
}

func TestCheckBinaryInPATH_Pass(t *testing.T) {
	binDir := t.TempDir()
	// exec.LookPath on Windows only resolves files with a PATHEXT extension,
	// so the fixture needs an .exe suffix there
	binaryName := policyPackBinaryName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binPath := filepath.Join(binDir, binaryName)
	require.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\necho ok"), 0o755))
	t.Setenv("PATH", binDir)

	result := checkBinaryInPATH()

	assert.Equal(t, "binary_in_path", result.Name)
	assert.Equal(t, "pass", result.Status)
	assert.Contains(t, result.Message, binPath)
	assert.Empty(t, result.Remediation)
}

func TestRunChecks_SkipCascade(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())

	report, err := RunChecks(context.Background())
	require.NoError(t, err)
	require.NotNil(t, report)

	require.Len(t, report.Checks, 4)
	assert.False(t, report.AllPass)
	assert.Equal(t, "policy_pack_dir", report.Checks[0].Name)
	assert.Equal(t, "fail", report.Checks[0].Status)
	assert.Equal(t, "skip", report.Checks[1].Status)
	assert.Equal(t, "skip", report.Checks[2].Status)
	assert.Equal(t, "skip", report.Checks[3].Status)
}

func TestRunChecks_SkipAfterYAMLFailure(t *testing.T) {
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	t.Setenv("PATH", t.TempDir())

	ppDir := filepath.Join(finfocusHome, policyPackDirName)
	require.NoError(t, os.MkdirAll(ppDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ppDir, pulumiPolicyFilename),
		[]byte("name: finfocus\nruntime: invalid\n"),
		0o600,
	))

	report, err := RunChecks(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Checks, 4)

	assert.False(t, report.AllPass)
	assert.Equal(t, "pass", report.Checks[0].Status)
	assert.Equal(t, "fail", report.Checks[1].Status)
	assert.Equal(t, "skip", report.Checks[2].Status)
	assert.Equal(t, "skip", report.Checks[3].Status)
}

func TestRunChecks_SkipAfterBinaryFailure(t *testing.T) {
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	t.Setenv("PATH", t.TempDir())

	ppDir := filepath.Join(finfocusHome, policyPackDirName)
	require.NoError(t, os.MkdirAll(ppDir, 0o755))
	require.NoError(t, WritePulumiPolicyYAML(ppDir))

	report, err := RunChecks(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Checks, 4)

	assert.False(t, report.AllPass)
	assert.Equal(t, "pass", report.Checks[0].Status)
	assert.Equal(t, "pass", report.Checks[1].Status)
	assert.Equal(t, "fail", report.Checks[2].Status)
	assert.Equal(t, "skip", report.Checks[3].Status)
}

func TestReadServePort(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		port, err := readServePort(context.Background(), strings.NewReader("43210\n"))
		require.NoError(t, err)
		assert.Equal(t, 43210, port)
	})

	t.Run("invalid output", func(t *testing.T) {
		_, err := readServePort(context.Background(), strings.NewReader("not-a-port\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected port output")
	})

	t.Run("timeout", func(t *testing.T) {
		pipeReader, pipeWriter := io.Pipe()
		defer func() {
			_ = pipeWriter.Close()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := readServePort(ctx, pipeReader)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timed out")
	})
}

func TestFirstLine(t *testing.T) {
	assert.Equal(t, "line1", firstLine("line1\nline2"))
	assert.Equal(t, "single-line", firstLine("single-line"))
}

func TestCheckGRPCSmokeTest_FailsInTestBinary(t *testing.T) {
	result := checkGRPCSmokeTest(context.Background())

	assert.Equal(t, "grpc_smoke_test", result.Name)
	assert.Equal(t, "fail", result.Status)
	assert.NotEmpty(t, result.Message)
}
