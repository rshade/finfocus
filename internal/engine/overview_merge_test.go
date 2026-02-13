package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// MapOperationToStatus
// ---------------------------------------------------------------------------

func TestMapOperationToStatus(t *testing.T) {
	tests := []struct {
		op     string
		expect ResourceStatus
	}{
		{"create", StatusCreating},
		{"update", StatusUpdating},
		{"delete", StatusDeleting},
		{"replace", StatusReplacing},
		{"create-replacement", StatusReplacing},
		{"delete-replaced", StatusReplacing},
		{"same", StatusActive},
		{"refresh", StatusActive},
		{"", StatusActive},
		{"unknown-op", StatusActive},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			assert.Equal(t, tt.expect, MapOperationToStatus(tt.op))
		})
	}
}

// ---------------------------------------------------------------------------
// MergeResourcesForOverview
// ---------------------------------------------------------------------------

func TestMergeResourcesForOverview(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		stateResources []StateResource
		planSteps      []PlanStep
		wantLen        int
		wantURNs       []string
		wantStatuses   []ResourceStatus
	}{
		{
			name:         "empty state and plan",
			wantLen:      0,
			wantURNs:     []string{},
			wantStatuses: []ResourceStatus{},
		},
		{
			name: "state only no plan changes",
			stateResources: []StateResource{
				{
					URN:    "urn:pulumi:stack::proj::aws:ec2:Instance::web",
					Type:   "aws:ec2:Instance",
					ID:     "i-123",
					Custom: true,
				},
				{
					URN:    "urn:pulumi:stack::proj::aws:s3:Bucket::data",
					Type:   "aws:s3:Bucket",
					ID:     "data-bucket",
					Custom: true,
				},
			},
			wantLen: 2,
			wantURNs: []string{
				"urn:pulumi:stack::proj::aws:ec2:Instance::web",
				"urn:pulumi:stack::proj::aws:s3:Bucket::data",
			},
			wantStatuses: []ResourceStatus{StatusActive, StatusActive},
		},
		{
			name: "filters out non-custom resources",
			stateResources: []StateResource{
				{
					URN:    "urn:pulumi:stack::proj::pulumi:providers:aws::default",
					Type:   "pulumi:providers:aws",
					Custom: false,
				},
				{
					URN:    "urn:pulumi:stack::proj::aws:ec2:Instance::web",
					Type:   "aws:ec2:Instance",
					ID:     "i-123",
					Custom: true,
				},
			},
			wantLen:      1,
			wantURNs:     []string{"urn:pulumi:stack::proj::aws:ec2:Instance::web"},
			wantStatuses: []ResourceStatus{StatusActive},
		},
		{
			name: "state resource with matching plan update",
			stateResources: []StateResource{
				{
					URN:    "urn:pulumi:stack::proj::aws:ec2:Instance::web",
					Type:   "aws:ec2:Instance",
					ID:     "i-123",
					Custom: true,
				},
			},
			planSteps: []PlanStep{
				{URN: "urn:pulumi:stack::proj::aws:ec2:Instance::web", Op: "update", Type: "aws:ec2:Instance"},
			},
			wantLen:      1,
			wantURNs:     []string{"urn:pulumi:stack::proj::aws:ec2:Instance::web"},
			wantStatuses: []ResourceStatus{StatusUpdating},
		},
		{
			name: "state resource with matching plan delete",
			stateResources: []StateResource{
				{
					URN:    "urn:pulumi:stack::proj::aws:ec2:Instance::web",
					Type:   "aws:ec2:Instance",
					ID:     "i-123",
					Custom: true,
				},
			},
			planSteps: []PlanStep{
				{URN: "urn:pulumi:stack::proj::aws:ec2:Instance::web", Op: "delete", Type: "aws:ec2:Instance"},
			},
			wantLen:      1,
			wantURNs:     []string{"urn:pulumi:stack::proj::aws:ec2:Instance::web"},
			wantStatuses: []ResourceStatus{StatusDeleting},
		},
		{
			name: "state resource with matching plan replace",
			stateResources: []StateResource{
				{
					URN:    "urn:pulumi:stack::proj::aws:ec2:Instance::web",
					Type:   "aws:ec2:Instance",
					ID:     "i-123",
					Custom: true,
				},
			},
			planSteps: []PlanStep{
				{URN: "urn:pulumi:stack::proj::aws:ec2:Instance::web", Op: "replace", Type: "aws:ec2:Instance"},
			},
			wantLen:      1,
			wantURNs:     []string{"urn:pulumi:stack::proj::aws:ec2:Instance::web"},
			wantStatuses: []ResourceStatus{StatusReplacing},
		},
		{
			name: "new resource in plan only",
			stateResources: []StateResource{
				{
					URN:    "urn:pulumi:stack::proj::aws:ec2:Instance::web",
					Type:   "aws:ec2:Instance",
					ID:     "i-123",
					Custom: true,
				},
			},
			planSteps: []PlanStep{
				{URN: "urn:pulumi:stack::proj::aws:s3:Bucket::new-bucket", Op: "create", Type: "aws:s3:Bucket"},
			},
			wantLen: 2,
			wantURNs: []string{
				"urn:pulumi:stack::proj::aws:ec2:Instance::web",
				"urn:pulumi:stack::proj::aws:s3:Bucket::new-bucket",
			},
			wantStatuses: []ResourceStatus{StatusActive, StatusCreating},
		},
		{
			name: "plan delete for non-state resource is ignored",
			planSteps: []PlanStep{
				{URN: "urn:pulumi:stack::proj::aws:ec2:Instance::ghost", Op: "delete", Type: "aws:ec2:Instance"},
			},
			wantLen:      0,
			wantURNs:     []string{},
			wantStatuses: []ResourceStatus{},
		},
		{
			name: "mixed scenario preserves state order",
			stateResources: []StateResource{
				{URN: "urn:a", Type: "aws:ec2:Instance", ID: "i-a", Custom: true},
				{URN: "urn:b", Type: "aws:s3:Bucket", ID: "b-b", Custom: true},
				{URN: "urn:c", Type: "aws:rds:Instance", ID: "db-c", Custom: true},
			},
			planSteps: []PlanStep{
				{URN: "urn:b", Op: "update", Type: "aws:s3:Bucket"},
				{URN: "urn:c", Op: "delete", Type: "aws:rds:Instance"},
				{URN: "urn:d", Op: "create", Type: "aws:lambda:Function"},
			},
			wantLen:  4,
			wantURNs: []string{"urn:a", "urn:b", "urn:c", "urn:d"},
			wantStatuses: []ResourceStatus{
				StatusActive,   // urn:a - no plan entry
				StatusUpdating, // urn:b - plan update
				StatusDeleting, // urn:c - plan delete
				StatusCreating, // urn:d - new in plan
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := MergeResourcesForOverview(ctx, tt.stateResources, tt.planSteps)
			require.NoError(t, err)
			require.Len(t, rows, tt.wantLen)

			require.Len(t, tt.wantURNs, tt.wantLen, "test setup: wantURNs length must match wantLen")
			require.Len(t, tt.wantStatuses, tt.wantLen, "test setup: wantStatuses length must match wantLen")

			for i, row := range rows {
				assert.Equal(t, tt.wantURNs[i], row.URN, "URN mismatch at index %d", i)
				assert.Equal(
					t,
					tt.wantStatuses[i],
					row.Status,
					"Status mismatch at index %d for URN %s",
					i,
					row.URN,
				)
			}
		})
	}
}

func TestMergeResourcesForOverview_SkeletonRowsHaveNilCosts(t *testing.T) {
	ctx := context.Background()

	rows, err := MergeResourcesForOverview(ctx,
		[]StateResource{
			{URN: "urn:a", Type: "aws:ec2:Instance", ID: "i-a", Custom: true},
		},
		[]PlanStep{
			{URN: "urn:b", Op: "create", Type: "aws:s3:Bucket"},
		},
	)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	for _, row := range rows {
		assert.Nil(t, row.ActualCost, "skeleton rows should not have ActualCost")
		assert.Nil(t, row.ProjectedCost, "skeleton rows should not have ProjectedCost")
		assert.Nil(t, row.CostDrift, "skeleton rows should not have CostDrift")
		assert.Nil(t, row.Error, "skeleton rows should not have Error")
		assert.Empty(t, row.Recommendations, "skeleton rows should not have Recommendations")
	}
}

func TestMergeResourcesForOverview_ResourceIDPopulated(t *testing.T) {
	ctx := context.Background()

	rows, err := MergeResourcesForOverview(ctx,
		[]StateResource{
			{URN: "urn:a", Type: "aws:ec2:Instance", ID: "i-abc123", Custom: true},
		},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "i-abc123", rows[0].ResourceID)
}

// ---------------------------------------------------------------------------
// DetectPendingChanges
// ---------------------------------------------------------------------------

func TestDetectPendingChanges(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		steps     []PlanStep
		wantHas   bool
		wantCount int
	}{
		{
			name:      "empty plan",
			steps:     nil,
			wantHas:   false,
			wantCount: 0,
		},
		{
			name: "no mutating ops",
			steps: []PlanStep{
				{URN: "urn:a", Op: "same"},
				{URN: "urn:b", Op: "refresh"},
			},
			wantHas:   false,
			wantCount: 0,
		},
		{
			name: "single create",
			steps: []PlanStep{
				{URN: "urn:a", Op: "create"},
			},
			wantHas:   true,
			wantCount: 1,
		},
		{
			name: "all mutating operation types",
			steps: []PlanStep{
				{URN: "urn:a", Op: "create"},
				{URN: "urn:b", Op: "update"},
				{URN: "urn:c", Op: "delete"},
				{URN: "urn:d", Op: "replace"},
				{URN: "urn:e", Op: "create-replacement"},
				{URN: "urn:f", Op: "delete-replaced"},
			},
			wantHas:   true,
			wantCount: 6,
		},
		{
			name: "mixed mutating and non-mutating",
			steps: []PlanStep{
				{URN: "urn:a", Op: "same"},
				{URN: "urn:b", Op: "update"},
				{URN: "urn:c", Op: "same"},
				{URN: "urn:d", Op: "create"},
			},
			wantHas:   true,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has, count := DetectPendingChanges(ctx, tt.steps)
			assert.Equal(t, tt.wantHas, has)
			assert.Equal(t, tt.wantCount, count)
		})
	}
}
