package history_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func TestHistoryWriter_RecordStateSnapshot_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	resources := []history.StateResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:Instance1",
			CloudID:  "i-10001",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{"env": "test"},
		},
		{
			URN:      "urn:pulumi:aws:s3:bucket:Bucket1",
			CloudID:  "bucket-abc",
			Type:     "aws:s3/bucket:Bucket",
			Provider: "aws",
			Tags:     map[string]string{"app": "myapp"},
		},
		{
			URN:      "urn:pulumi:gcp:compute:instance:GCPInstance",
			CloudID:  "instance-gcp-123",
			Type:     "gcp:compute/instance:Instance",
			Provider: "gcp",
			Tags:     map[string]string{},
		},
	}

	writer.RecordStateSnapshot(stackCtx, resources)

	stackHash := stackCtx.Hash()

	for _, res := range resources {
		urnHash := history.URNHash(res.URN)
		results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
		require.NoError(t, err)
		require.Len(t, results, 1)

		entry := results[0]
		assert.Equal(t, res.URN, entry.URN)
		assert.Equal(t, res.CloudID, entry.CloudID)
		assert.Equal(t, res.Type, entry.Type)
		assert.Equal(t, res.Provider, entry.Provider)
		assert.Equal(t, history.SourceStateSnapshot, entry.Source)
		if len(res.Tags) > 0 {
			assert.Equal(t, res.Tags, entry.Tags)
		}
	}
}

func TestHistoryWriter_RecordStateSnapshot_UpdatesLastSeen(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	firstBatch := []history.StateResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:MyInstance",
			CloudID:  "i-12345",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{},
		},
	}

	writer.RecordStateSnapshot(stackCtx, firstBatch)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(firstBatch[0].URN)

	results1, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results1, 1)
	firstLastSeen := results1[0].LastSeen

	time.Sleep(1 * time.Second)

	secondBatch := []history.StateResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:MyInstance",
			CloudID:  "i-12345",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{},
		},
	}

	writer.RecordStateSnapshot(stackCtx, secondBatch)

	results2, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results2, 1)
	secondLastSeen := results2[0].LastSeen

	assert.Greater(t, secondLastSeen, firstLastSeen, "LastSeen should be updated on subsequent snapshot")
}

func TestHistoryWriter_RecordStateSnapshot_SkipsEmptyCloudID(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	resources := []history.StateResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:Good",
			CloudID:  "i-12345",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{},
		},
		{
			URN:      "urn:pulumi:aws:ec2:instance:NoCloudID",
			CloudID:  "",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{},
		},
		{
			URN:      "urn:pulumi:aws:s3:bucket:AlsoGood",
			CloudID:  "bucket-xyz",
			Type:     "aws:s3/bucket:Bucket",
			Provider: "aws",
			Tags:     map[string]string{},
		},
	}

	writer.RecordStateSnapshot(stackCtx, resources)

	stackHash := stackCtx.Hash()
	allResults, err := store.GetAllForStack(stackHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)

	assert.Len(t, allResults, 2, "only resources with CloudID should be stored")

	cloudIDs := make(map[string]bool)
	for _, entry := range allResults {
		cloudIDs[entry.CloudID] = true
	}
	assert.True(t, cloudIDs["i-12345"])
	assert.True(t, cloudIDs["bucket-xyz"])
	assert.False(t, cloudIDs[""])
}

func TestHistoryWriter_RecordStateSnapshot_FireAndForget(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	disabledStore, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	defer disabledStore.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(disabledStore, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	resources := []history.StateResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:Instance1",
			CloudID:  "i-10001",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{},
		},
	}

	writer.RecordStateSnapshot(stackCtx, resources)
}

func TestHistoryWriter_RecordStateSnapshot_NilStore(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(nil, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	resources := []history.StateResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:Instance1",
			CloudID:  "i-10001",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Tags:     map[string]string{},
		},
	}

	writer.RecordStateSnapshot(stackCtx, resources)
}

// ---------------------------------------------------------------------------
// T030: RecordPlanLineage tests
// ---------------------------------------------------------------------------

func TestHistoryWriter_RecordPlanLineage_ReplaceRecordsBothIDs(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	steps := []history.PlanStep{
		{
			Op:         "replace",
			URN:        "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:       "aws:ec2/instance:Instance",
			Provider:   "aws",
			OldCloudID: "i-old-abc123",
			NewCloudID: "i-new-def456",
			Tags:       map[string]string{"env": "dev"},
		},
	}

	writer.RecordPlanLineage(stackCtx, steps)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(steps[0].URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	assert.Len(t, results, 2, "replace should record both old and new cloud IDs")

	cloudIDs := make(map[string]bool)
	for _, entry := range results {
		cloudIDs[entry.CloudID] = true
		assert.Equal(t, history.SourcePlanLineage, entry.Source)
		assert.Equal(t, "aws:ec2/instance:Instance", entry.Type)
		assert.Equal(t, "aws", entry.Provider)
	}
	assert.True(t, cloudIDs["i-old-abc123"], "old cloud ID should be recorded")
	assert.True(t, cloudIDs["i-new-def456"], "new cloud ID should be recorded")
}

func TestHistoryWriter_RecordPlanLineage_DeleteRecordsOldID(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	steps := []history.PlanStep{
		{
			Op:         "delete",
			URN:        "urn:pulumi:dev::app::aws:s3/bucket:Bucket::old-data",
			Type:       "aws:s3/bucket:Bucket",
			Provider:   "aws",
			OldCloudID: "bucket-to-delete",
			NewCloudID: "",
			Tags:       map[string]string{},
		},
	}

	writer.RecordPlanLineage(stackCtx, steps)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(steps[0].URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results, 1, "delete should record old cloud ID only")
	assert.Equal(t, "bucket-to-delete", results[0].CloudID)
	assert.Equal(t, history.SourcePlanLineage, results[0].Source)
}

func TestHistoryWriter_RecordPlanLineage_CreateRecordsNewID(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	steps := []history.PlanStep{
		{
			Op:         "create",
			URN:        "urn:pulumi:dev::app::aws:ec2/instance:Instance::new-web",
			Type:       "aws:ec2/instance:Instance",
			Provider:   "aws",
			OldCloudID: "",
			NewCloudID: "i-brand-new",
			Tags:       map[string]string{},
		},
	}

	writer.RecordPlanLineage(stackCtx, steps)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(steps[0].URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results, 1, "create should record new cloud ID only")
	assert.Equal(t, "i-brand-new", results[0].CloudID)
	assert.Equal(t, history.SourcePlanLineage, results[0].Source)
}

func TestHistoryWriter_RecordPlanLineage_SkipsEmptyCloudIDs(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	steps := []history.PlanStep{
		{
			Op:         "update",
			URN:        "urn:pulumi:dev::app::aws:ec2/instance:Instance::config-change",
			Type:       "aws:ec2/instance:Instance",
			Provider:   "aws",
			OldCloudID: "",
			NewCloudID: "",
			Tags:       map[string]string{},
		},
	}

	writer.RecordPlanLineage(stackCtx, steps)

	stackHash := stackCtx.Hash()
	allResults, err := store.GetAllForStack(stackHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	assert.Empty(t, allResults, "steps with both empty cloud IDs should be skipped")
}

func TestHistoryWriter_RecordPlanLineage_NilStore(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(nil, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	steps := []history.PlanStep{
		{
			Op:         "replace",
			URN:        "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:       "aws:ec2/instance:Instance",
			Provider:   "aws",
			OldCloudID: "i-old",
			NewCloudID: "i-new",
		},
	}

	writer.RecordPlanLineage(stackCtx, steps)
}

func TestHistoryWriter_RecordPlanLineage_DisabledStore(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	steps := []history.PlanStep{
		{
			Op:         "replace",
			URN:        "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:       "aws:ec2/instance:Instance",
			Provider:   "aws",
			OldCloudID: "i-old",
			NewCloudID: "i-new",
		},
	}

	writer.RecordPlanLineage(stackCtx, steps)
}

// ---------------------------------------------------------------------------
// T031: RecordAnalyzerEvent tests
// ---------------------------------------------------------------------------

func TestHistoryWriter_RecordAnalyzerEvent_RecordsWithCloudID(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "i-deployed-123",
		Properties: map[string]any{
			"instanceType": "t3.micro",
		},
	}

	writer.RecordAnalyzerEvent(stackCtx, event)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(event.URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results, 1)

	entry := results[0]
	assert.Equal(t, event.URN, entry.URN)
	assert.Equal(t, event.CloudID, entry.CloudID)
	assert.Equal(t, event.Type, entry.Type)
	assert.Equal(t, event.Provider, entry.Provider)
	assert.Equal(t, history.SourceAnalyzerEvent, entry.Source)
}

func TestHistoryWriter_RecordAnalyzerEvent_SkipsEmptyCloudID(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::preview-only",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "",
		Properties: map[string]any{
			"instanceType": "t3.micro",
		},
	}

	writer.RecordAnalyzerEvent(stackCtx, event)

	stackHash := stackCtx.Hash()
	allResults, err := store.GetAllForStack(stackHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	assert.Empty(t, allResults, "event without cloud ID should not be stored (DryRun=true)")
}

func TestHistoryWriter_RecordAnalyzerEvent_NilStore(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(nil, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "i-deployed",
	}

	writer.RecordAnalyzerEvent(stackCtx, event)
}

func TestHistoryWriter_RecordAnalyzerEvent_DisabledStore(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "i-deployed",
	}

	writer.RecordAnalyzerEvent(stackCtx, event)
}

// ---------------------------------------------------------------------------
// Tag extraction from analyzer event properties
// ---------------------------------------------------------------------------

func TestHistoryWriter_RecordAnalyzerEvent_ExtractsTagsFromProperties(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "i-tagged-123",
		Properties: map[string]any{
			"instanceType": "t3.micro",
			"tags": map[string]any{
				"Name": "web-server",
				"env":  "prod",
			},
		},
	}

	writer.RecordAnalyzerEvent(stackCtx, event)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(event.URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results, 1)

	entry := results[0]
	assert.Equal(t, "web-server", entry.Tags["Name"])
	assert.Equal(t, "prod", entry.Tags["env"])
	assert.Len(t, entry.Tags, 2)
}

func TestHistoryWriter_RecordAnalyzerEvent_PrefersTagsAllOverTags(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "i-tagsall-456",
		Properties: map[string]any{
			"tags": map[string]any{
				"Name": "from-tags",
			},
			"tagsAll": map[string]any{
				"Name":       "from-tagsAll",
				"managed-by": "pulumi",
			},
		},
	}

	writer.RecordAnalyzerEvent(stackCtx, event)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(event.URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results, 1)

	entry := results[0]
	assert.Equal(t, "from-tagsAll", entry.Tags["Name"], "tagsAll should take precedence over tags")
	assert.Equal(t, "pulumi", entry.Tags["managed-by"])
	assert.Len(t, entry.Tags, 2)
}

func TestHistoryWriter_RecordAnalyzerEvent_EmptyPropertiesNoTags(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::empty",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "i-empty-789",
	}

	writer.RecordAnalyzerEvent(stackCtx, event)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(event.URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].Tags)
}

func TestHistoryWriter_RecordAnalyzerEvent_PropertiesWithoutTagKeys(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter())
	writer := history.NewWriter(store, logger)

	stackCtx := history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}

	event := history.AnalyzerResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::notags",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		CloudID:  "i-notags-000",
		Properties: map[string]any{
			"instanceType":     "t3.micro",
			"availabilityZone": "us-east-1a",
		},
	}

	writer.RecordAnalyzerEvent(stackCtx, event)

	stackHash := stackCtx.Hash()
	urnHash := history.URNHash(event.URN)
	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].Tags)
}
