package cli_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// TestNDJSONEncoder_LineByLineEncoding tests that NDJSON encoder writes
// each item as a separate line immediately without buffering.
func TestNDJSONEncoder_LineByLineEncoding(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{
			ResourceID:       "r1",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 100.0,
			Currency:         "USD",
			Description:      "Test 1",
		},
		{
			ResourceID:       "r2",
			Type:             "TERMINATE",
			EstimatedSavings: 50.0,
			Currency:         "USD",
			Description:      "Test 2",
		},
		{
			ResourceID:       "r3",
			Type:             "DELETE_UNUSED",
			EstimatedSavings: 25.0,
			Currency:         "USD",
			Description:      "Test 3",
		},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	// Parse output line by line
	scanner := bufio.NewScanner(&buf)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		// Each line should be valid JSON
		var jsonObj map[string]interface{}
		err := json.Unmarshal([]byte(line), &jsonObj)
		require.NoError(t, err, "line %d should be valid JSON: %s", lineCount, line)
	}

	// Should have summary + 3 recommendations = 4 lines
	assert.Equal(t, 4, lineCount, "should have exactly 4 lines (1 summary + 3 items)")
}

// TestNDJSONEncoder_NoBuffering tests that NDJSON output appears immediately
// without buffering delays.
func TestNDJSONEncoder_NoBuffering(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{
			ResourceID:       "r1",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 100.0,
			Currency:         "USD",
			Description:      "Test 1",
		},
	}

	// Use a pipe to test streaming behavior
	pr, pw := io.Pipe()
	defer pr.Close()

	// Write in goroutine
	errCh := make(chan error, 1)
	go func() {
		err := cli.RenderRecommendationsNDJSONForTest(pw, recs)
		pw.Close()
		errCh <- err
	}()

	// Read first line immediately - should not block
	scanner := bufio.NewScanner(pr)
	require.True(t, scanner.Scan(), "should read first line immediately")

	firstLine := scanner.Text()
	var firstObj map[string]interface{}
	err := json.Unmarshal([]byte(firstLine), &firstObj)
	require.NoError(t, err)

	// Verify it's the summary line
	assert.Equal(t, "summary", firstObj["type"])

	// Read second line (the recommendation)
	require.True(t, scanner.Scan(), "should read second line immediately")

	// Wait for writer to complete
	require.NoError(t, <-errCh)
}

// TestNDJSONEncoder_EmptyList tests NDJSON output with zero recommendations.
func TestNDJSONEncoder_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, []cli.TestableRecommendation{})
	require.NoError(t, err)

	// Should have only summary line
	scanner := bufio.NewScanner(&buf)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		var jsonObj map[string]interface{}
		err := json.Unmarshal([]byte(line), &jsonObj)
		require.NoError(t, err)

		// First line should be summary with 0 count
		if lineCount == 1 {
			assert.Equal(t, "summary", jsonObj["type"])
			assert.Equal(t, float64(0), jsonObj["total_count"])
		}
	}

	assert.Equal(t, 1, lineCount, "should have exactly 1 line (summary only)")
}

// TestNDJSONEncoder_NoPaginationMetadata tests that NDJSON output
// does not include pagination metadata (for true streaming).
func TestNDJSONEncoder_NoPaginationMetadata(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{
			ResourceID:       "r1",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 100.0,
			Currency:         "USD",
			Description:      "Test",
		},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	// Parse summary line
	scanner := bufio.NewScanner(&buf)
	require.True(t, scanner.Scan())

	var summary map[string]interface{}
	err = json.Unmarshal(scanner.Bytes(), &summary)
	require.NoError(t, err)

	// Verify no pagination metadata in streaming mode
	_, hasPagination := summary["pagination"]
	assert.False(
		t,
		hasPagination,
		"NDJSON output should not include pagination metadata for streaming",
	)
}

// TestNDJSONEncoder_LargeDataset tests NDJSON output with large dataset
// to verify memory efficiency and no buffering.
func TestNDJSONEncoder_LargeDataset(t *testing.T) {
	// Create 1000 recommendations
	recs := make([]cli.TestableRecommendation, 1000)
	for i := range recs {
		recs[i] = cli.TestableRecommendation{
			ResourceID:       "resource-" + string(rune('A'+i%26)),
			Type:             "RIGHTSIZE",
			EstimatedSavings: float64(i),
			Currency:         "USD",
			Description:      "Test recommendation",
		}
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	// Count lines
	scanner := bufio.NewScanner(&buf)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	// Should have summary + 1000 recommendations = 1001 lines
	assert.Equal(t, 1001, lineCount)
}

// TestNDJSONEncoder_ValidJSONOnEachLine tests that every line
// in NDJSON output is valid, parseable JSON.
func TestNDJSONEncoder_ValidJSONOnEachLine(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{
			ResourceID:       "r1",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 100.5,
			Currency:         "USD",
			Description:      "Test with special chars: \n\t\"quotes\"",
		},
		{
			ResourceID:       "r2",
			Type:             "TERMINATE",
			EstimatedSavings: 0.0,
			Currency:         "USD",
			Description:      "Zero savings",
		},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	scanner := bufio.NewScanner(&buf)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Each line must be valid JSON
		var jsonObj map[string]interface{}
		err := json.Unmarshal([]byte(line), &jsonObj)
		assert.NoError(t, err, "line %d should be valid JSON: %s", lineNum, line)

		// Verify no trailing commas or invalid JSON
		assert.NotEmpty(t, jsonObj, "line %d should parse to non-empty object", lineNum)
	}

	assert.Equal(t, 3, lineNum, "should have 3 lines total")
}

// TestNDJSONEncoder_SummaryStructure tests the structure of the summary line.
func TestNDJSONEncoder_SummaryStructure(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{
			ResourceID:       "r1",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 100.0,
			Currency:         "USD",
			Description:      "Test 1",
		},
		{
			ResourceID:       "r2",
			Type:             "TERMINATE",
			EstimatedSavings: 50.0,
			Currency:         "USD",
			Description:      "Test 2",
		},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	// Parse first line (summary)
	scanner := bufio.NewScanner(&buf)
	require.True(t, scanner.Scan())

	var summary map[string]interface{}
	err = json.Unmarshal(scanner.Bytes(), &summary)
	require.NoError(t, err)

	// Verify summary structure
	assert.Equal(t, "summary", summary["type"])
	assert.Equal(t, float64(2), summary["total_count"])
	assert.Equal(t, float64(150.0), summary["total_savings"])
	assert.Equal(t, "USD", summary["currency"])

	// Verify count_by_action_type exists
	countByAction, ok := summary["count_by_action_type"].(map[string]interface{})
	require.True(t, ok, "count_by_action_type should be a map")
	assert.Equal(t, float64(1), countByAction["RIGHTSIZE"])
	assert.Equal(t, float64(1), countByAction["TERMINATE"])
}

// TestNDJSONEncoder_RecommendationStructure tests the structure of recommendation lines.
func TestNDJSONEncoder_RecommendationStructure(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{
			ResourceID:       "aws:ec2:Instance/i-0abc123",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 87.60,
			Currency:         "USD",
			Description:      "Consider downsizing",
		},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	// Skip summary line, read recommendation line
	scanner := bufio.NewScanner(&buf)
	require.True(t, scanner.Scan()) // summary
	require.True(t, scanner.Scan()) // recommendation

	var rec map[string]interface{}
	err = json.Unmarshal(scanner.Bytes(), &rec)
	require.NoError(t, err)

	// Verify recommendation structure
	assert.Equal(t, "aws:ec2:Instance/i-0abc123", rec["resource_id"])
	assert.Equal(t, "RIGHTSIZE", rec["action_type"])
	assert.Equal(t, "Consider downsizing", rec["description"])
	assert.Equal(t, float64(87.60), rec["estimated_savings"])
	assert.Equal(t, "USD", rec["currency"])

	// Should NOT have a "type" field (that's only for summary)
	_, hasType := rec["type"]
	assert.False(t, hasType, "recommendation should not have 'type' field")
}

// TestNDJSONEncoder_NewlineTermination tests that output ends with newline.
func TestNDJSONEncoder_NewlineTermination(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{
			ResourceID:       "r1",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 100.0,
			Currency:         "USD",
			Description:      "Test",
		},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	output := buf.String()

	// Output should end with exactly one newline
	assert.True(t, strings.HasSuffix(output, "\n"), "output should end with newline")

	// Count trailing newlines (should be exactly 1)
	trimmed := strings.TrimRight(output, "\n")
	trailingNewlines := len(output) - len(trimmed)
	assert.Equal(t, 1, trailingNewlines, "should have exactly one trailing newline")
}
