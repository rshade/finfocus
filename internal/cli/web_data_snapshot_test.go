package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/stackload"
)

// Test sources enter through the same pipeline publication used by the web session.
func publishWebTestState(t *testing.T, data *webData, state []ingest.StackExportResource) {
	t.Helper()
	snapshot := &stackload.Snapshot{RawState: state, StateStatus: stackload.Status{Loaded: true}}
	require.NoError(t, data.setPipelineSource(context.Background(), OverviewPipelineResult{Snapshot: snapshot}))
}

func publishWebTestPlan(ctx context.Context, t *testing.T, data *webData, plan *ingest.PulumiPlan) {
	t.Helper()
	base := data.snapshot
	if base == nil {
		base = &stackload.Snapshot{}
	}
	err := data.setPipelineSource(ctx, OverviewPipelineResult{Snapshot: base.WithPlan(plan)})
	if ctx.Err() != nil {
		require.ErrorIs(t, err, ctx.Err())
		return
	}
	require.NoError(t, err)
}

// Replacement preview steps describe one planned resource, regardless of whether
// Pulumi creates or deletes the old instance first.
func TestWebSnapshotLiveInventoryCollapsesReplacementSteps(t *testing.T) {
	t.Parallel()
	const (
		replaced     = "urn:pulumi:dev::test::aws:ec2/instance:Instance::replaced"
		unchanged    = "urn:pulumi:dev::test::aws:ec2/instance:Instance::unchanged"
		deleted      = "urn:pulumi:dev::test::aws:ec2/instance:Instance::deleted"
		resourceType = "aws:ec2/instance:Instance"
	)
	orders := [][]string{
		{"create-replacement", "replace", "delete-replaced"},
		{"create-replacement", "delete-replaced", "replace"},
		{"replace", "create-replacement", "delete-replaced"},
		{"replace", "delete-replaced", "create-replacement"},
		{"delete-replaced", "create-replacement", "replace"},
		{"delete-replaced", "replace", "create-replacement"},
	}
	for _, order := range orders {
		t.Run(strings.Join(order, "/"), func(t *testing.T) {
			t.Parallel()
			data := newWebData(&cobra.Command{}, overviewParams{cfg: config.New()}, engine.DateRange{}, nil)
			publishWebTestState(t, data, []ingest.StackExportResource{
				{URN: replaced, Type: resourceType, Inputs: map[string]any{"instanceType": "t3.small"}},
				{URN: unchanged, Type: resourceType, Inputs: map[string]any{"instanceType": "t3.micro"}},
				{URN: deleted, Type: resourceType, Inputs: map[string]any{"instanceType": "t3.nano"}},
			})
			plan := &ingest.PulumiPlan{}
			for _, op := range order {
				step := ingest.PulumiStep{Op: op, URN: replaced, Type: resourceType,
					OldState: &ingest.PulumiState{Inputs: map[string]any{"instanceType": "t3.small"}},
				}
				if op == "create-replacement" {
					step.NewState = &ingest.PulumiState{Inputs: map[string]any{"instanceType": "m5.large"}}
				}
				plan.Steps = append(plan.Steps, step)
			}
			plan.Steps = append(plan.Steps, ingest.PulumiStep{Op: "delete", URN: deleted, Type: resourceType})
			publishWebTestPlan(context.Background(), t, data, plan)

			resources, err := data.estimateResources(context.Background())
			require.NoError(t, err)
			require.Len(t, resources, 2)
			byID := resourcesByID(resources)
			require.Contains(t, byID, replaced)
			require.Contains(t, byID, unchanged)
			assert.Equal(t, "m5.large", byID[replaced].Properties["instanceType"])
			assert.Equal(t, "t3.micro", byID[unchanged].Properties["instanceType"])
			assert.NotContains(t, byID, deleted)

			// Returned property maps belong to the query, not the shared source.
			byID[replaced].Properties["instanceType"] = "mutated"
			byID[unchanged].Properties["instanceType"] = "mutated"
			next, err := data.liveResources()
			require.NoError(t, err)
			assert.Equal(t, "m5.large", resourcesByID(next)[replaced].Properties["instanceType"])
			assert.Equal(t, "t3.micro", resourcesByID(next)[unchanged].Properties["instanceType"])
			assert.Len(t, data.snapshot.Descriptors(), 5, "projected pricing keeps all preview steps")
			actual, err := data.loadResources(context.Background())
			require.NoError(t, err)
			require.Len(t, actual, 3)
			assert.Equal(t, "t3.small", resourcesByID(actual)[replaced].Properties["instanceType"])
			assert.Contains(t, resourcesByID(actual), deleted)
		})
	}
}

// A delete-replaced step with no partner is the old resource, which the preview
// removes; the live inventory must not offer it for estimate or recommendations.
func TestWebSnapshotLiveInventoryDropsUnpairedDeleteReplaced(t *testing.T) {
	t.Parallel()
	const (
		orphan       = "urn:pulumi:dev::test::aws:ec2/instance:Instance::orphan"
		kept         = "urn:pulumi:dev::test::aws:ec2/instance:Instance::kept"
		resourceType = "aws:ec2/instance:Instance"
	)
	data := newWebData(&cobra.Command{}, overviewParams{cfg: config.New()}, engine.DateRange{}, nil)
	publishWebTestState(t, data, []ingest.StackExportResource{
		{URN: orphan, Type: resourceType, Inputs: map[string]any{"instanceType": "t3.small"}},
		{URN: kept, Type: resourceType, Inputs: map[string]any{"instanceType": "t3.micro"}},
	})
	publishWebTestPlan(context.Background(), t, data, &ingest.PulumiPlan{Steps: []ingest.PulumiStep{{
		Op: "delete-replaced", URN: orphan, Type: resourceType,
		OldState: &ingest.PulumiState{Inputs: map[string]any{"instanceType": "t3.small"}},
	}}})

	resources, err := data.liveResources()
	require.NoError(t, err)
	byID := resourcesByID(resources)
	assert.NotContains(t, byID, orphan)
	assert.Contains(t, byID, kept)
}

type snapshotHistoryClient struct {
	webActualClient

	mu  sync.Mutex
	ids []string
}

func (c *snapshotHistoryClient) GetActualCost(
	ctx context.Context,
	request *proto.GetActualCostRequest,
	opts ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	c.mu.Lock()
	c.ids = append(c.ids, request.ResourceIDs...)
	c.mu.Unlock()
	return c.webActualClient.GetActualCost(ctx, request, opts...)
}

// Historical resources remain in actual queries even though the current shared
// snapshot contains only deployed resources.
func TestWebSnapshotActualRetainsHistoryEnrichment(t *testing.T) {
	ctx := context.Background()
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: test\nruntime: yaml\n"), 0600))
	t.Chdir(project)
	installFakePulumi(t)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{}))
	store, err := history.NewBoltStore(ctx, t.TempDir(), true, 30)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	now := time.Now()
	dates := engine.DateRange{Start: now.Add(-time.Hour), End: now.Add(time.Hour)}
	stack, ok := detectHistoryStackContext(ctx)
	require.True(t, ok)
	const deleted = "urn:pulumi:dev::test::aws:ec2/instance:Instance::deleted"
	require.NoError(t, store.Upsert(stack.Hash(), history.ResourceHistoryEntry{
		URN: deleted, CloudID: "i-deleted", Type: "aws:ec2/instance:Instance", Provider: "aws",
		FirstSeen: now.Add(-time.Minute).Unix(), LastSeen: now.Unix(), Source: history.SourceStateSnapshot,
	}))
	data := newWebData(&cobra.Command{}, overviewParams{cfg: &config.Config{}}, dates, nil)
	data.history = store
	client := &snapshotHistoryClient{}
	data.eng = engine.New([]*pluginhost.Client{{Name: "fixture", API: client}}, nil)
	publishWebTestState(t, data, nil)
	_, err = data.actualCosts(ctx, "", nil)
	require.NoError(t, err)
	assert.Contains(t, client.ids, deleted)
	deployed, err := data.loadResources(ctx)
	require.NoError(t, err)
	assert.Empty(t, deployed)
}
