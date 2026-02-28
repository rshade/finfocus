package engine

import (
	"context"
	"testing"
	"time"

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

func TestMergeResourcesForOverview_PropertiesPreserved(t *testing.T) {
	ctx := context.Background()

	props := map[string]interface{}{
		"instanceType":     "t3.micro",
		"availabilityZone": "us-east-1a",
	}

	rows, err := MergeResourcesForOverview(ctx,
		[]StateResource{
			{
				URN:        "urn:a",
				Type:       "aws:ec2:Instance",
				ID:         "i-abc123",
				Custom:     true,
				Properties: props,
			},
		},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, props, rows[0].Properties)
}

func TestMergeResourcesForOverview_NilPropertiesOK(t *testing.T) {
	ctx := context.Background()

	rows, err := MergeResourcesForOverview(ctx,
		[]StateResource{
			{URN: "urn:a", Type: "aws:ec2:Instance", ID: "i-123", Custom: true},
		},
		[]PlanStep{
			{URN: "urn:b", Op: "create", Type: "aws:s3:Bucket"},
		},
	)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Nil(t, rows[0].Properties, "state resource without properties should have nil")
	assert.Nil(t, rows[1].Properties, "plan-only resource should have nil properties")
}

// ---------------------------------------------------------------------------
// NewRowsFromState
// ---------------------------------------------------------------------------

func TestNewRowsFromState(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          []StateResource
		expectLen      int
		expectURNOrder []string
		allActive      bool
		nilCosts       bool
	}{
		{
			name: "only custom resources",
			input: []StateResource{
				{URN: "urn:provider", Type: "pulumi:providers:aws", Custom: false},
				{URN: "urn:ec2", Type: "aws:ec2:Instance", ID: "i-123", Custom: true},
				{URN: "urn:s3", Type: "aws:s3:Bucket", ID: "bucket-abc", Custom: true},
			},
			expectLen:      2,
			expectURNOrder: []string{"urn:ec2", "urn:s3"},
			allActive:      true,
		},
		{
			name: "preserves state order",
			input: []StateResource{
				{URN: "urn:a", Type: "aws:ec2:Instance", Custom: true},
				{URN: "urn:b", Type: "aws:s3:Bucket", Custom: true},
				{URN: "urn:c", Type: "aws:rds:Instance", Custom: true},
			},
			expectLen:      3,
			expectURNOrder: []string{"urn:a", "urn:b", "urn:c"},
			allActive:      true,
		},
		{
			name: "all skeleton rows have StatusActive",
			input: []StateResource{
				{URN: "urn:a", Type: "aws:ec2:Instance", Custom: true},
				{URN: "urn:b", Type: "aws:s3:Bucket", Custom: true},
			},
			expectLen: 2,
			allActive: true,
		},
		{
			name: "skeleton rows have nil cost fields",
			input: []StateResource{
				{URN: "urn:a", Type: "aws:ec2:Instance", Custom: true},
			},
			expectLen: 1,
			nilCosts:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := NewRowsFromState(ctx, tt.input)
			require.Len(t, rows, tt.expectLen)
			for i, urn := range tt.expectURNOrder {
				assert.Equal(t, urn, rows[i].URN)
			}
			if tt.allActive {
				for _, row := range rows {
					assert.Equal(t, StatusActive, row.Status)
				}
			}
			if tt.nilCosts && len(rows) > 0 {
				assert.Nil(t, rows[0].ActualCost)
				assert.Nil(t, rows[0].ProjectedCost)
				assert.Nil(t, rows[0].CostDrift)
				assert.Nil(t, rows[0].Error)
				assert.Empty(t, rows[0].Recommendations)
			}
		})
	}
}

func TestMergeResourcesForOverview_CreatedAtPreserved(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2025, 2, 13, 10, 0, 0, 0, time.UTC)

	rows, err := MergeResourcesForOverview(ctx,
		[]StateResource{
			{
				URN:       "urn:a",
				Type:      "aws:ec2:Instance",
				ID:        "i-abc123",
				Custom:    true,
				CreatedAt: &createdAt,
			},
		},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].CreatedAt)
	assert.Equal(t, createdAt, *rows[0].CreatedAt)
}

func TestMergeResourcesForOverview_NilCreatedAtOK(t *testing.T) {
	ctx := context.Background()

	rows, err := MergeResourcesForOverview(ctx,
		[]StateResource{
			{URN: "urn:a", Type: "aws:ec2:Instance", Custom: true, CreatedAt: nil},
		},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].CreatedAt)
}

func TestNewRowsFromState_CreatedAtPreserved(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)

	rows := NewRowsFromState(ctx, []StateResource{
		{URN: "urn:a", Type: "aws:ec2:Instance", Custom: true, CreatedAt: &createdAt},
		{URN: "urn:b", Type: "aws:s3:Bucket", Custom: true, CreatedAt: nil},
	})
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].CreatedAt)
	assert.Equal(t, createdAt, *rows[0].CreatedAt)
	assert.Nil(t, rows[1].CreatedAt)
}

// ---------------------------------------------------------------------------
// ApplyChangesToRows
// ---------------------------------------------------------------------------

func TestApplyChangesToRows(t *testing.T) {
	t.Run("updates matching URNs", func(t *testing.T) {
		rows := []OverviewRow{
			{URN: "urn:a", Status: StatusActive},
			{URN: "urn:b", Status: StatusActive},
		}
		ApplyChangesToRows(rows, map[string]ResourceStatus{"urn:a": StatusUpdating})
		assert.Equal(t, StatusUpdating, rows[0].Status)
		assert.Equal(t, StatusActive, rows[1].Status)
	})

	t.Run("preserves unmatched rows", func(t *testing.T) {
		rows := []OverviewRow{
			{URN: "urn:a", Status: StatusActive},
			{URN: "urn:b", Status: StatusActive},
		}
		ApplyChangesToRows(rows, map[string]ResourceStatus{"urn:c": StatusCreating})
		assert.Equal(t, StatusActive, rows[0].Status)
		assert.Equal(t, StatusActive, rows[1].Status)
	})

	t.Run("empty map no-op", func(t *testing.T) {
		rows := []OverviewRow{
			{URN: "urn:a", Status: StatusDeleting},
			{URN: "urn:b", Status: StatusUpdating},
		}
		ApplyChangesToRows(rows, map[string]ResourceStatus{})
		assert.Equal(t, StatusDeleting, rows[0].Status)
		assert.Equal(t, StatusUpdating, rows[1].Status)
	})

	t.Run("nil map no-op", func(t *testing.T) {
		rows := []OverviewRow{
			{URN: "urn:a", Status: StatusDeleting},
			{URN: "urn:b", Status: StatusUpdating},
		}
		ApplyChangesToRows(rows, nil)
		assert.Equal(t, StatusDeleting, rows[0].Status, "nil map should leave statuses unchanged")
		assert.Equal(t, StatusUpdating, rows[1].Status, "nil map should leave statuses unchanged")
	})

	t.Run("nil rows is a no-op", func(t *testing.T) {
		assert.NotPanics(t, func() {
			ApplyChangesToRows(nil, map[string]ResourceStatus{})
		}, "ApplyChangesToRows should be a no-op on nil rows input")
	})
}

// ---------------------------------------------------------------------------
// BuildStatusByURN
// ---------------------------------------------------------------------------

func TestBuildStatusByURN(t *testing.T) {
	tests := []struct {
		name    string
		steps   []PlanStep
		wantMap map[string]ResourceStatus
	}{
		{
			name:    "empty steps returns empty map",
			steps:   nil,
			wantMap: map[string]ResourceStatus{},
		},
		{
			name: "single step per URN",
			steps: []PlanStep{
				{URN: "urn:a", Op: "create", Type: "aws:ec2:Instance"},
				{URN: "urn:b", Op: "update", Type: "aws:s3:Bucket"},
				{URN: "urn:c", Op: "delete", Type: "aws:rds:Instance"},
			},
			wantMap: map[string]ResourceStatus{
				"urn:a": StatusCreating,
				"urn:b": StatusUpdating,
				"urn:c": StatusDeleting,
			},
		},
		{
			name: "delete-replaced wins over create-replacement for same URN",
			steps: []PlanStep{
				{URN: "urn:a", Op: "create-replacement", Type: "aws:ec2:Instance"},
				{URN: "urn:a", Op: "delete-replaced", Type: "aws:ec2:Instance"},
			},
			wantMap: map[string]ResourceStatus{
				"urn:a": StatusReplacing,
			},
		},
		{
			name: "delete wins over create for same URN (highest precedence)",
			steps: []PlanStep{
				{URN: "urn:a", Op: "create", Type: "aws:ec2:Instance"},
				{URN: "urn:a", Op: "delete", Type: "aws:ec2:Instance"},
			},
			wantMap: map[string]ResourceStatus{
				"urn:a": StatusDeleting,
			},
		},
		{
			name: "replace-family steps result in StatusReplacing",
			steps: []PlanStep{
				{URN: "urn:a", Op: "replace", Type: "aws:ec2:Instance"},
			},
			wantMap: map[string]ResourceStatus{
				"urn:a": StatusReplacing,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildStatusByURN(tt.steps)
			require.Len(t, got, len(tt.wantMap), "map length mismatch")
			for urn, wantStatus := range tt.wantMap {
				assert.Equal(t, wantStatus, got[urn], "status mismatch for URN %s", urn)
			}
		})
	}
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
