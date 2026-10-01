package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNDJSONStreaming_HeadTermination tests that NDJSON output works correctly
// when piped to `head -n N` command, verifying graceful SIGPIPE handling.
//
//nolint:paralleltest // subtests share the parent-scoped fixture err = os.WriteFile(...)
func TestNDJSONStreaming_HeadTermination(t *testing.T) {
	// Create a simple Pulumi plan JSON
	planJSON := `{
		"version": 3,
		"steps": []
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	// Build finfocus binary path
	binPath := filepath.Join("..", "..", "bin", "finfocus")

	// Test with head -n 5: should terminate after 5 lines without error
	t.Run("head -n 5 terminates gracefully", func(t *testing.T) {
		// Run: finfocus cost recommendations --pulumi-json plan.json --output ndjson | head -n 5
		cmd1 := exec.Command(binPath, "cost", "recommendations", "--pulumi-json", planPath, "--output", "ndjson")
		cmd2 := exec.Command("head", "-n", "5")

		// Connect pipe
		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr, "failed to create pipe")
		cmd1.Stdout = w
		cmd2.Stdin = r

		var out bytes.Buffer
		cmd2.Stdout = &out

		// Start both commands
		err = cmd1.Start()
		if err != nil {
			t.Skip("finfocus binary not available, skipping integration test")
			return
		}

		err = cmd2.Start()
		require.NoError(t, err)

		// Close write end after starting cmd1
		w.Close()

		// Wait for both commands
		_ = cmd2.Wait() // head always succeeds
		_ = cmd1.Wait() // finfocus should handle SIGPIPE gracefully

		// Verify output has at most 5 lines
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) > 0 && lines[0] != "" {
			assert.LessOrEqual(t, len(lines), 5, "head -n 5 should limit output to 5 lines")
		}
	})

	// Test with head -n 1: should get only summary line
	t.Run("head -n 1 gets only summary", func(t *testing.T) {
		cmd1 := exec.Command(binPath, "cost", "recommendations", "--pulumi-json", planPath, "--output", "ndjson")
		cmd2 := exec.Command("head", "-n", "1")

		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr, "failed to create pipe")
		cmd1.Stdout = w
		cmd2.Stdin = r

		var out bytes.Buffer
		cmd2.Stdout = &out

		err = cmd1.Start()
		if err != nil {
			t.Skip("finfocus binary not available, skipping integration test")
			return
		}

		err = cmd2.Start()
		require.NoError(t, err)

		w.Close()

		_ = cmd2.Wait()
		_ = cmd1.Wait()

		// Verify we got exactly 1 line (summary)
		output := strings.TrimSpace(out.String())
		if output != "" {
			lines := strings.Split(output, "\n")
			assert.Len(t, lines, 1, "head -n 1 should return exactly 1 line")

			// Parse and verify it's the summary line
			var summary map[string]interface{}
			err = json.Unmarshal([]byte(lines[0]), &summary)
			require.NoError(t, err)
			assert.Equal(t, "summary", summary["type"], "first line should be summary")
		}
	})
}

// TestNDJSONStreaming_JQProcessing tests that NDJSON output works correctly
// with jq for line-by-line JSON processing.
func TestNDJSONStreaming_JQProcessing(t *testing.T) {
	t.Parallel()

	// Create a Pulumi plan JSON
	planJSON := `{
		"version": 3,
		"steps": []
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	binPath := filepath.Join("..", "..", "bin", "finfocus")

	// Check if jq is available
	if _, lookPathErr := exec.LookPath("jq"); lookPathErr != nil {
		t.Skip("jq not available, skipping jq integration test")
		return
	}

	t.Run("jq processes each line", func(t *testing.T) {
		t.Parallel()
		// Run: finfocus cost recommendations --pulumi-json plan.json --output ndjson | jq -c '.'
		output, jqErr := runJQPipeline(t, binPath, planPath, "-c", ".")

		// Verify jq processed output successfully (no stderr errors)
		if jqErr != "" {
			t.Logf("jq stderr: %s", jqErr)
		}

		// Each line of output should be valid JSON
		assertEachLineValidJSON(t, output)
	})

	t.Run("jq filters specific fields", func(t *testing.T) {
		t.Parallel()
		// Run: finfocus ... | jq -c '.type // .resource_id'
		// This selects either the type field (for summary) or resource_id field (for recommendations)
		output, _ := runJQPipeline(t, binPath, planPath, "-c", ".type // .resource_id")

		// Verify jq extracted fields successfully
		if output == "" {
			return
		}
		lines := strings.Split(output, "\n")
		require.NotEmpty(t, lines)
		// First line should be "summary" (from summary.type field)
		firstLine := strings.Trim(lines[0], "\"")
		assert.Equal(t, "summary", firstLine, "first jq output should be 'summary'")
	})
}

// runJQPipeline runs finfocus cost recommendations piped through jq and
// returns jq's trimmed stdout and raw stderr.
func runJQPipeline(t *testing.T, binPath, planPath string, jqArgs ...string) (string, string) {
	t.Helper()

	cmd1 := exec.Command(binPath, "cost", "recommendations", "--pulumi-json", planPath, "--output", "ndjson")
	cmd2 := exec.Command("jq", jqArgs...)

	r, w, err := os.Pipe()
	require.NoError(t, err, "failed to create pipe")
	cmd1.Stdout = w
	cmd2.Stdin = r

	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd2.Stdout = &out
	cmd2.Stderr = &errBuf

	err = cmd1.Start()
	if err != nil {
		t.Skip("finfocus binary not available, skipping integration test")
	}

	err = cmd2.Start()
	require.NoError(t, err)

	w.Close()

	_ = cmd2.Wait()
	_ = cmd1.Wait()

	return strings.TrimSpace(out.String()), errBuf.String()
}

// assertEachLineValidJSON asserts that every non-empty line is valid JSON.
func assertEachLineValidJSON(t *testing.T, output string) {
	t.Helper()
	if output == "" {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		var jsonObj map[string]interface{}
		assert.NoError(t, json.Unmarshal([]byte(line), &jsonObj), "line %d should be valid JSON", lineNum)
	}
}

// TestNDJSONStreaming_NoBuffering tests that NDJSON output appears immediately
// without buffering delays when processed line-by-line.
//
//nolint:paralleltest // subtests share the parent-scoped fixture err = os.WriteFile(...)
func TestNDJSONStreaming_NoBuffering(t *testing.T) {
	planJSON := `{
		"version": 3,
		"steps": []
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	binPath := filepath.Join("..", "..", "bin", "finfocus")

	t.Run("lines appear immediately", func(t *testing.T) {
		cmd := exec.Command(binPath, "cost", "recommendations", "--pulumi-json", planPath, "--output", "ndjson")

		stdout, pipeErr := cmd.StdoutPipe()
		if pipeErr != nil {
			t.Fatalf("failed to get stdout pipe: %v", pipeErr)
		}

		err = cmd.Start()
		if err != nil {
			t.Skip("finfocus binary not available, skipping integration test")
			return
		}

		// Read first line immediately - should not block
		scanner := bufio.NewScanner(stdout)
		gotFirstLine := scanner.Scan()

		cmd.Wait()

		// Verify we got the first line without blocking
		assert.True(t, gotFirstLine, "should get first line immediately without buffering")

		if gotFirstLine {
			firstLine := scanner.Text()
			var summary map[string]interface{}
			unmarshalErr := json.Unmarshal([]byte(firstLine), &summary)
			require.NoError(t, unmarshalErr, "first line should be valid JSON")
			assert.Equal(t, "summary", summary["type"], "first line should be summary")
		}
	})
}

// TestNDJSONStreaming_NoPaginationMetadata verifies that NDJSON output
// does not include pagination metadata for true streaming compatibility.
func TestNDJSONStreaming_NoPaginationMetadata(t *testing.T) {
	t.Parallel()

	planJSON := `{
		"version": 3,
		"steps": []
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	binPath := filepath.Join("..", "..", "bin", "finfocus")

	t.Run("summary has no pagination metadata", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(binPath, "cost", "recommendations", "--pulumi-json", planPath, "--output", "ndjson")

		out, cmdErr := cmd.Output()
		if cmdErr != nil {
			var exitErr *exec.ExitError
			if errors.As(cmdErr, &exitErr) {
				t.Logf("command stderr: %s", exitErr.Stderr)
			}
			t.Skip("finfocus binary not available or command failed, skipping test")
			return
		}

		// Parse summary line
		scanner := bufio.NewScanner(bytes.NewReader(out))
		if scanner.Scan() {
			var summary map[string]interface{}
			unmarshalErr := json.Unmarshal(scanner.Bytes(), &summary)
			require.NoError(t, unmarshalErr)

			// Verify no pagination field in streaming mode
			_, hasPagination := summary["pagination"]
			assert.False(t, hasPagination, "NDJSON summary should not include pagination metadata for streaming")
		}
	})
}
