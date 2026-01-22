package recorder

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func silentLogger() zerolog.Logger {
	return zerolog.New(os.Stderr).Level(zerolog.Disabled)
}

// T013: Unit test for Recorder.Record().
func TestRecorder_RecordRequest(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: "aws:ec2:Instance",
			Provider:     "aws",
			Sku:          "t3.medium",
			Region:       "us-east-1",
		},
	}

	err := recorder.RecordRequest("GetProjectedCost", req)
	require.NoError(t, err)

	// Verify file was created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	// Verify file name format
	filename := files[0].Name()
	assert.True(t, strings.HasSuffix(filename, ".json"))
	assert.Contains(t, filename, "GetProjectedCost")
	assert.Contains(t, filename, "_")

	// Verify file content is valid JSON
	content, err := os.ReadFile(filepath.Join(tmpDir, filename))
	require.NoError(t, err)

	var recorded RecordedRequest
	err = json.Unmarshal(content, &recorded)
	require.NoError(t, err)

	assert.Equal(t, "GetProjectedCost", recorded.Method)
	assert.NotEmpty(t, recorded.RequestID)
	assert.NotEmpty(t, recorded.Timestamp)
	assert.NotNil(t, recorded.Request)
}

// T014: Unit test for generateFilename() ULID format.
func TestRecorder_GenerateFilename(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	filename := recorder.generateFilename("TestMethod")

	// Should match format: <timestamp>_<method>_<ulid>.json
	parts := strings.Split(strings.TrimSuffix(filename, ".json"), "_")
	require.Len(t, parts, 3, "filename should have 3 parts: timestamp_method_ulid")

	// Verify timestamp format (20060102T150405Z)
	timestamp := parts[0]
	_, err := time.Parse("20060102T150405Z", timestamp)
	assert.NoError(t, err, "timestamp should be valid ISO8601 compact format")

	// Verify method name
	assert.Equal(t, "TestMethod", parts[1])

	// Verify ULID format (26 characters, uppercase alphanumeric)
	ulid := parts[2]
	assert.Len(t, ulid, 26, "ULID should be 26 characters")
	for _, c := range ulid {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z'),
			"ULID should contain only uppercase alphanumeric characters")
	}
}

// T015: Unit test for directory creation.
func TestRecorder_DirectoryCreation(t *testing.T) {
	// Create a path that doesn't exist
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "recording", "dir")

	recorder := NewRecorder(nestedDir, silentLogger())

	// Directory should be created
	info, err := os.Stat(nestedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Should be able to record
	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: "test:type",
		},
	}
	err = recorder.RecordRequest("Test", req)
	assert.NoError(t, err)
	assert.False(t, recorder.disabled)
}

// T016a: Unit test for malformed request handling.
func TestRecorder_MalformedRequestHandling(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	// Record a nil request (edge case)
	// This should still create a file with the nil serialized
	err := recorder.RecordRequest("NilRequest", nil)
	// protojson.Marshal handles nil gracefully
	require.NoError(t, err)

	// Verify file was created even for nil request
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

// T025: Handle edge case: non-writable directory.
func TestRecorder_NonWritableDirectory(t *testing.T) {
	// Skip on Windows where permission model is different
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err := os.MkdirAll(readOnlyDir, 0555) // Read + execute only
	require.NoError(t, err)

	recorder := NewRecorder(readOnlyDir, silentLogger())

	// Recorder should be disabled due to non-writable directory
	assert.True(t, recorder.disabled)

	// Recording should fail with error
	req := &pbc.GetProjectedCostRequest{}
	err = recorder.RecordRequest("Test", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

// T047: Ensure thread-safety with sync.Mutex.
func TestRecorder_ThreadSafety(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	// Run concurrent recordings
	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &pbc.GetProjectedCostRequest{
				Resource: &pbc.ResourceDescriptor{
					ResourceType: "test:type",
					Provider:     "test",
				},
			}
			_ = recorder.RecordRequest("ConcurrentTest", req)
		}()
	}

	wg.Wait()

	// Verify all files were created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, concurrency, "all concurrent recordings should succeed")

	// Verify no duplicate filenames (ULIDs should be unique)
	filenames := make(map[string]bool)
	for _, f := range files {
		assert.False(t, filenames[f.Name()], "duplicate filename found: %s", f.Name())
		filenames[f.Name()] = true
	}
}

func TestRecorder_SerializeRequest(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: "aws:ec2:Instance",
			Provider:     "aws",
			Sku:          "t3.medium",
			Region:       "us-east-1",
			Tags: map[string]string{
				"environment": "production",
			},
		},
	}

	data, err := recorder.serializeRequest(req)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Verify resource fields are present
	resource, ok := parsed["resource"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "aws:ec2:Instance", resource["resourceType"])
}

func TestRecorder_Close(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	// Should not panic
	recorder.Close()
}

// T058a: Benchmark test validating <10ms recording overhead.
func BenchmarkRecorder_RecordRequest(b *testing.B) {
	tmpDir := b.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: "aws:ec2:Instance",
			Provider:     "aws",
			Sku:          "t3.medium",
			Region:       "us-east-1",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = recorder.RecordRequest("Benchmark", req)
	}
}

// T041a: Concurrent request stress test (100+ parallel requests).
func TestRecorder_StressTest(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewRecorder(tmpDir, silentLogger())

	// Run 100+ concurrent recordings
	var wg sync.WaitGroup
	concurrency := 150

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &pbc.GetProjectedCostRequest{
				Resource: &pbc.ResourceDescriptor{
					ResourceType: "aws:ec2:Instance",
					Provider:     "aws",
				},
			}
			_ = recorder.RecordRequest("StressTest", req)
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	// Verify all files were created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, concurrency)

	// Log performance metrics
	t.Logf("Stress test: %d requests in %v (%.2f req/s)",
		concurrency, duration, float64(concurrency)/duration.Seconds())
}

// Test isDiskFullError function.
func TestIsDiskFullError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "no space left on device",
			err:      errors.New("no space left on device"),
			expected: true,
		},
		{
			name:     "disk quota exceeded",
			err:      errors.New("disk quota exceeded"),
			expected: true,
		},
		{
			name:     "ENOSPC error",
			err:      errors.New("write failed: ENOSPC"),
			expected: true,
		},
		{
			name:     "permission denied",
			err:      os.ErrPermission,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDiskFullError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
