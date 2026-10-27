package stackload

import (
	"context"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"
)

func sampleState() []ingest.StackExportResource {
	return []ingest.StackExportResource{
		{
			URN:    "urn:a",
			Type:   "aws:ec2/instance:Instance",
			Custom: true,
			ID:     "i-a",
			Inputs: map[string]any{"nested": map[string]any{"size": "small"}},
		},
		{URN: "urn:b", Type: "aws:s3/bucket:Bucket", Custom: true},
	}
}
func samplePlan() *ingest.PulumiPlan {
	return &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				URN:      "urn:a",
				Type:     "aws:ec2/instance:Instance",
				Op:       "update",
				OldState: &ingest.PulumiState{Inputs: map[string]any{"size": "small"}},
				NewState: &ingest.PulumiState{Inputs: map[string]any{"size": "large"}},
			},
			{URN: "urn:c", Type: "aws:s3/bucket:Bucket", Op: "create"},
		},
		ChangeSummary: map[string]int{"same": 4},
	}
}
func TestDescriptorViews(t *testing.T) {
	t.Parallel()
	s := (&Snapshot{RawState: sampleState(), StateStatus: Status{Loaded: true}}).WithPlan(samplePlan())
	want, err := ingest.MapResources(samplePlan().GetResources())
	require.NoError(t, err)
	got := s.Descriptors()
	require.Len(t, got, 3)
	assert.Equal(t, want, got[:2])
	assert.Equal(t, "urn:b", got[2].ID)
	assert.Equal(t, "same", got[2].Operation)
	assert.Equal(t, engine.OriginState, got[2].Origin)
	deployed, err := ingest.MapStateResources(sampleState())
	require.NoError(t, err)
	assert.Equal(t, deployed, s.DeployedDescriptors())
	got[0].Properties["size"] = "mutated"
	assert.Equal(t, "large", s.Descriptors()[0].Properties["size"])
	d := s.DeployedDescriptors()
	d[0].Properties["nested"].(map[string]any)["size"] = "mutated"
	assert.Equal(t, "small", s.DeployedDescriptors()[0].Properties["nested"].(map[string]any)["size"])
	stateOnly := &Snapshot{RawState: sampleState(), StateStatus: Status{Loaded: true}}
	for _, r := range stateOnly.Descriptors() {
		assert.Equal(t, "same", r.Operation)
		assert.Equal(t, engine.OriginPlan, r.Origin)
	}
	assert.Equal(t, want, (&Snapshot{}).WithPlan(samplePlan()).Descriptors())
}
func TestWithPlanRowsAndMetadata(t *testing.T) {
	t.Parallel()
	s := &Snapshot{
		RawState:    sampleState(),
		State:       convertStateResources(sampleState()),
		StateStatus: Status{Loaded: true},
	}
	next := s.WithPlan(samplePlan())
	assert.Nil(t, s.Plan)
	assert.Equal(t, 4, next.OmittedUnchanged)
	assert.Empty(t, next.Mismatches)
	rows, err := next.Rows(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, engine.StatusUpdating, rows[0].Status)
	assert.Equal(t, engine.StatusCreating, rows[2].Status)
	assert.Equal(t, "size", rows[0].PropertyDiffs[0].Key)
	pending, count := next.PendingChanges(context.Background())
	assert.True(t, pending)
	assert.Equal(t, 2, count)
	rows[0].Properties["nested"].(map[string]any)["size"] = "mutated"
	assert.Equal(t, "small", next.State[0].Properties["nested"].(map[string]any)["size"])
	only, err := s.Rows(context.Background())
	require.NoError(t, err)
	assert.Len(t, only, 2)
}
func TestMismatchesAndOmissions(t *testing.T) {
	t.Parallel()
	p := samplePlan()
	p.Steps = append(
		p.Steps,
		ingest.PulumiStep{URN: "urn:missing", Op: "delete"},
		ingest.PulumiStep{URN: "urn:b", Op: "create"},
		ingest.PulumiStep{URN: "urn:a", Op: "same"},
	)
	s := (&Snapshot{RawState: sampleState(), StateStatus: Status{Loaded: true}}).WithPlan(p)
	assert.Equal(t, []string{"urn:missing", "urn:b"}, s.Mismatches)
	assert.Equal(t, 3, s.OmittedUnchanged)
	assert.Empty(t, (&Snapshot{}).WithPlan(p).Mismatches)
	p.ChangeSummary = nil
	assert.Zero(t, s.WithPlan(p).OmittedUnchanged)
	p.ChangeSummary = map[string]int{"same": 0}
	assert.Zero(t, s.WithPlan(p).OmittedUnchanged)
	assert.Empty(t, s.WithPlan(nil).Mismatches)
}

func TestDescriptorsPreserveExplicitEmptyProperties(t *testing.T) {
	t.Parallel()
	plan := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				URN:      "urn:empty",
				Type:     "aws:s3/bucket:Bucket",
				Op:       "create",
				NewState: &ingest.PulumiState{Inputs: map[string]any{}},
			},
		},
	}
	want, err := ingest.MapResources(plan.GetResources())
	require.NoError(t, err)
	assert.Equal(t, want, (&Snapshot{}).WithPlan(plan).Descriptors())
}

func TestMismatchesIgnoreBookkeeping(t *testing.T) {
	t.Parallel()
	p := samplePlan()
	p.Steps = append(
		p.Steps,
		ingest.PulumiStep{URN: "urn:root", Type: "pulumi:pulumi:Stack", Op: "same"},
		ingest.PulumiStep{URN: "urn:provider", Type: "pulumi:providers:aws", Op: "same"},
	)
	s := (&Snapshot{RawState: sampleState(), StateStatus: Status{Loaded: true}}).WithPlan(p)
	assert.Empty(t, s.Mismatches)
}

func TestDescriptorReferencesAreIndependent(t *testing.T) {
	t.Parallel()
	state := sampleState()
	state[0].PropertyDependencies = map[string][]string{"subnet": {"urn:subnet"}}
	plan := samplePlan()
	plan.Steps[0].NewState.PropertyDependencies = map[string][]string{"subnet": {"urn:planned-subnet"}}
	snapshot := (&Snapshot{RawState: state, StateStatus: Status{Loaded: true}}).WithPlan(plan)
	for _, tc := range []struct {
		name     string
		snapshot *Snapshot
		deployed bool
		want     string
	}{
		{"plan", snapshot, false, "urn:planned-subnet"},
		{"deployed", snapshot, true, "urn:subnet"},
		{"state-only", &Snapshot{RawState: state}, false, "urn:subnet"},
		{"appended-state", (&Snapshot{RawState: state}).WithPlan(&ingest.PulumiPlan{}), false, "urn:subnet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			descriptors := tc.snapshot.Descriptors
			if tc.deployed {
				descriptors = tc.snapshot.DeployedDescriptors
			}
			first := descriptors()
			require.NotEmpty(t, first[0].Refs["subnet"])
			first[0].Refs["subnet"][0] = "mutated"
			first[0].Refs["added"] = []string{"urn:added"}
			second := descriptors()
			assert.Equal(t, []string{tc.want}, second[0].Refs["subnet"])
			assert.NotContains(t, second[0].Refs, "added")
		})
	}
}

func TestActualDescriptors(t *testing.T) {
	t.Parallel()
	deployed, err := ingest.MapStateResources(sampleState())
	require.NoError(t, err)
	planned, err := ingest.MapResources(samplePlan().GetResources())
	require.NoError(t, err)
	withState := (&Snapshot{RawState: sampleState(), StateStatus: Status{Loaded: true}}).WithPlan(samplePlan())
	assert.Equal(t, deployed, withState.ActualDescriptors(), "read state is the deployed inventory")
	savedPreview := (&Snapshot{StateStatus: Status{Reason: ReasonNotRequested}}).WithPlan(samplePlan())
	assert.Equal(
		t,
		planned,
		savedPreview.ActualDescriptors(),
		"without state, a saved preview keeps its v0.5.2 inventory",
	)
	unreadable := (&Snapshot{StateStatus: Status{Reason: ReasonBackendUnreachable}}).WithPlan(samplePlan())
	assert.Equal(t, planned, unreadable.ActualDescriptors())
}
