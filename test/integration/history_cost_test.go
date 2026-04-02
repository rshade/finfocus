package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
)

// TestHistoryCostFlow_FullWriteReadMerge exercises the complete history pipeline:
// 1. Create a BoltStore in a temp directory
// 2. Write history entries for two cloud IDs under the same URN (mid-month replacement)
// 3. Write a history entry for a resource NOT in current state (deletion)
// 4. Call the merge logic with current state containing only the new cloud ID
// 5. Verify the output contains entries for BOTH old and new cloud IDs plus the deleted resource.
func TestHistoryCostFlow_FullWriteReadMerge(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := *logging.FromContext(ctx)
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "testorg",
		Project:      "testproject",
		Stack:        "dev",
	}

	now := time.Now().Unix()

	// Simulate mid-month replacement: web server was replaced, creating a new cloud ID.
	// Old cloud ID: "i-old-web" (active from day 1-15)
	// New cloud ID: "i-new-web" (active from day 15-now)
	webURN := "urn:pulumi:dev::testproject::aws:ec2/instance:Instance::web"

	// Record old incarnation
	oldResources := []history.StateResource{
		{
			URN:      webURN,
			CloudID:  "i-old-web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{"env": "dev"},
		},
	}
	writer.RecordStateSnapshot(stackCtx, oldResources)

	// Record new incarnation (simulating a later state snapshot)
	newResources := []history.StateResource{
		{
			URN:      webURN,
			CloudID:  "i-new-web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{"env": "dev"},
		},
	}
	writer.RecordStateSnapshot(stackCtx, newResources)

	// Record a resource that was deleted from current state
	deletedURN := "urn:pulumi:dev::testproject::aws:s3/bucket:Bucket::logs"
	deletedResources := []history.StateResource{
		{
			URN:      deletedURN,
			CloudID:  "logs-bucket-123",
			Type:     "aws:s3/bucket:Bucket",
			Provider: "aws",
			Tags:     map[string]string{"env": "dev"},
		},
	}
	writer.RecordStateSnapshot(stackCtx, deletedResources)

	// Step 2: Read historical resources for the period
	reader := history.NewReader(store, logger)
	historical, histErr := reader.GetResourcesForPeriod(stackCtx, now-86400, now+3600)
	require.NoError(t, histErr)
	require.NotEmpty(t, historical)

	// Step 3: Current state has only the new web server (no logs bucket)
	currentResources := []engine.ResourceDescriptor{
		{
			ID:       webURN,
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]any{
				"pulumi:cloudId": "i-new-web",
			},
		},
	}

	// Step 4: Merge historical resources into current
	merged := cli.MergeHistoricalResources(currentResources, historical)

	// Step 5: Verify the merged list contains:
	// - The current "i-new-web" (from currentResources)
	// - The old "i-old-web" (from history, added by merge)
	// - The deleted "logs-bucket-123" (from history, added by merge)
	cloudIDs := make(map[string]bool)
	for _, r := range merged {
		if id, ok := r.Properties["pulumi:cloudId"].(string); ok {
			cloudIDs[id] = true
		}
	}

	assert.True(t, cloudIDs["i-new-web"], "current cloud ID should be present")
	assert.True(t, cloudIDs["i-old-web"], "old (replaced) cloud ID should be present")
	assert.True(t, cloudIDs["logs-bucket-123"], "deleted resource cloud ID should be present")
	assert.Len(t, merged, 3, "merged list should have 3 resources (current + old + deleted)")
}

// TestHistoryCostFlow_NoHistoryStore verifies that when history store is nil,
// the merge function returns the current resources unchanged (no regression).
func TestHistoryCostFlow_NoHistoryStore(t *testing.T) {
	currentResources := []engine.ResourceDescriptor{
		{
			ID:       "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]any{
				"pulumi:cloudId": "i-web123",
			},
		},
	}

	// With empty historical resources, merge returns current unchanged
	merged := cli.MergeHistoricalResources(currentResources, nil)
	assert.Equal(t, currentResources, merged)
}

// TestHistoryCostFlow_DuplicateCloudIDNotAdded verifies that a historical
// cloud ID already in the current state is not duplicated.
func TestHistoryCostFlow_DuplicateCloudIDNotAdded(t *testing.T) {
	currentResources := []engine.ResourceDescriptor{
		{
			ID:       "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]any{
				"pulumi:cloudId": "i-web123",
			},
		},
	}

	historical := []history.HistoricalResource{
		{
			URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			CloudIDs: []string{"i-web123"}, // Same as current
		},
	}

	merged := cli.MergeHistoricalResources(currentResources, historical)
	assert.Len(t, merged, 1, "duplicate cloud ID should not create a new entry")
}

// TestHistoryCostFlow_MultiCloudID_MixedDuplicateAndNew verifies that when
// historical CloudIDs contain both a duplicate and new IDs, only the new
// IDs are added as separate descriptors.
func TestHistoryCostFlow_MultiCloudID_MixedDuplicateAndNew(t *testing.T) {
	currentResources := []engine.ResourceDescriptor{
		{
			ID:       "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]any{
				"pulumi:cloudId": "i-web123",
			},
		},
	}

	historical := []history.HistoricalResource{
		{
			URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			CloudIDs: []string{"i-web123", "i-web456", "i-web789"}, // 1 duplicate, 2 new
		},
	}

	merged := cli.MergeHistoricalResources(currentResources, historical)
	assert.Len(t, merged, 3, "should have original + 2 new cloud IDs")

	cloudIDs := make(map[string]bool)
	for _, r := range merged {
		if cid, ok := r.Properties["pulumi:cloudId"].(string); ok {
			cloudIDs[cid] = true
		}
	}
	assert.True(t, cloudIDs["i-web123"], "original cloud ID should be present")
	assert.True(t, cloudIDs["i-web456"], "new historical cloud ID should be added")
	assert.True(t, cloudIDs["i-web789"], "new historical cloud ID should be added")
}
