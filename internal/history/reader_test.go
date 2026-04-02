package history_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func TestHistoryReader_GetResourcesForPeriod_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws::instance:123",
			CloudID:   "i-12345",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 3600,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
			Tags: map[string]string{
				"environment": "test",
			},
		},
		{
			URN:       "urn:pulumi:aws::instance:123",
			CloudID:   "i-67890",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 7200,
			LastSeen:  now - 1800,
			Source:    history.SourceStateSnapshot,
			Tags: map[string]string{
				"environment": "test",
			},
		},
	}

	err = store.UpsertBatch(entries)
	require.NoError(t, err)

	logger := zerolog.New(io.Discard)
	reader := history.NewReader(store, logger)

	stack := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	results, err := reader.GetResourcesForPeriod(stack, now-10800, now+3600)
	require.NoError(t, err)
	require.Len(t, results, 1)

	res := results[0]
	assert.Equal(t, "urn:pulumi:aws::instance:123", res.URN)
	assert.Equal(t, "aws:ec2/instance:Instance", res.Type)
	assert.Equal(t, "aws", res.Provider)
	assert.Len(t, res.CloudIDs, 2)
	assert.Contains(t, res.CloudIDs, "i-12345")
	assert.Contains(t, res.CloudIDs, "i-67890")
	assert.Equal(t, "test", res.Tags["environment"])
}

func TestHistoryReader_GetResourcesForPeriod_TimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws::instance:1",
			CloudID:   "i-11111",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 86400,
			LastSeen:  now - 3600,
			Source:    history.SourceStateSnapshot,
			Tags:      map[string]string{},
		},
		{
			URN:       "urn:pulumi:aws::instance:2",
			CloudID:   "i-22222",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 1800,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
			Tags:      map[string]string{},
		},
	}

	err = store.UpsertBatch(entries)
	require.NoError(t, err)

	logger := zerolog.New(io.Discard)
	reader := history.NewReader(store, logger)

	stack := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	// Query with time range that only includes the second entry
	results, err := reader.GetResourcesForPeriod(stack, now-1000, now+1000)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "urn:pulumi:aws::instance:2", results[0].URN)
}

func TestHistoryReader_GetResourcesForPeriod_GroupsCloudIDs(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws::bucket:1",
			CloudID:   "bucket-aaa",
			Type:      "aws:s3/bucket:Bucket",
			Provider:  "aws",
			FirstSeen: now - 10000,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
			Tags: map[string]string{
				"v1": "value1",
			},
		},
		{
			URN:       "urn:pulumi:aws::bucket:1",
			CloudID:   "bucket-bbb",
			Type:      "aws:s3/bucket:Bucket",
			Provider:  "aws",
			FirstSeen: now - 5000,
			LastSeen:  now - 2000,
			Source:    history.SourceStateSnapshot,
			Tags: map[string]string{
				"v2": "value2",
			},
		},
		{
			URN:       "urn:pulumi:aws::bucket:1",
			CloudID:   "bucket-ccc",
			Type:      "aws:s3/bucket:Bucket",
			Provider:  "aws",
			FirstSeen: now - 1000,
			LastSeen:  now - 500,
			Source:    history.SourceStateSnapshot,
			Tags: map[string]string{
				"v3": "value3",
			},
		},
	}

	err = store.UpsertBatch(entries)
	require.NoError(t, err)

	logger := zerolog.New(io.Discard)
	reader := history.NewReader(store, logger)

	stack := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	results, err := reader.GetResourcesForPeriod(stack, now-15000, now+1000)
	require.NoError(t, err)
	require.Len(t, results, 1)

	res := results[0]
	assert.Equal(t, "urn:pulumi:aws::bucket:1", res.URN)
	assert.Len(t, res.CloudIDs, 3)
	assert.Contains(t, res.CloudIDs, "bucket-aaa")
	assert.Contains(t, res.CloudIDs, "bucket-bbb")
	assert.Contains(t, res.CloudIDs, "bucket-ccc")

	// All tags should be present (merged)
	assert.Equal(t, "value1", res.Tags["v1"])
	assert.Equal(t, "value2", res.Tags["v2"])
	assert.Equal(t, "value3", res.Tags["v3"])
}

func TestHistoryReader_GetResourcesForPeriod_EmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(io.Discard)
	reader := history.NewReader(store, logger)

	now := time.Now().Unix()
	stack := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	results, err := reader.GetResourcesForPeriod(stack, now-3600, now)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestHistoryReader_GetResourcesForPeriod_DisabledStore(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(io.Discard)
	reader := history.NewReader(store, logger)

	now := time.Now().Unix()
	stack := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	results, err := reader.GetResourcesForPeriod(stack, now-3600, now)
	require.NoError(t, err)
	assert.Empty(t, results)
}
