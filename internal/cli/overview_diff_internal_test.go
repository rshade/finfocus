package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

// ---------------------------------------------------------------------------
// convertPlanSteps — property diff extraction
// ---------------------------------------------------------------------------

func TestConvertPlanSteps_UpdateExtractsDiffs(t *testing.T) {
	steps := []ingest.PulumiStep{
		{
			URN:  "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Op:   "update",
			Type: "aws:ec2/instance:Instance",
			OldState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"instanceType": "t3.medium",
					"ami":          "ami-123",
				},
			},
			NewState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"instanceType": "t3.large",
					"ami":          "ami-123",
				},
			},
		},
	}

	result := convertPlanSteps(steps)
	require.Len(t, result, 1)
	require.Len(t, result[0].PropertyDiffs, 1)
	assert.Equal(t, "instanceType", result[0].PropertyDiffs[0].Key)
	assert.Equal(t, "t3.medium", result[0].PropertyDiffs[0].OldValue)
	assert.Equal(t, "t3.large", result[0].PropertyDiffs[0].NewValue)
}

func TestConvertPlanSteps_ReplaceExtractsDiffs(t *testing.T) {
	steps := []ingest.PulumiStep{
		{
			URN:  "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Op:   "replace",
			Type: "aws:ec2/instance:Instance",
			OldState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"ami": "ami-old",
				},
			},
			NewState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"ami": "ami-new",
				},
			},
		},
	}

	result := convertPlanSteps(steps)
	require.Len(t, result, 1)
	require.Len(t, result[0].PropertyDiffs, 1)
	assert.Equal(t, "ami", result[0].PropertyDiffs[0].Key)
}

func TestConvertPlanSteps_CreateReplacementExtractsDiffs(t *testing.T) {
	steps := []ingest.PulumiStep{
		{
			URN:  "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Op:   "create-replacement",
			Type: "aws:ec2/instance:Instance",
			OldState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"subnetId": "subnet-aaa",
				},
			},
			NewState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"subnetId": "subnet-bbb",
				},
			},
		},
	}

	result := convertPlanSteps(steps)
	require.Len(t, result, 1)
	require.Len(t, result[0].PropertyDiffs, 1)
	assert.Equal(t, "subnetId", result[0].PropertyDiffs[0].Key)
}

func TestConvertPlanSteps_CreateNoDiffs(t *testing.T) {
	steps := []ingest.PulumiStep{
		{
			URN:  "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Op:   "create",
			Type: "aws:ec2/instance:Instance",
			NewState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"instanceType": "t3.large",
				},
			},
		},
	}

	result := convertPlanSteps(steps)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].PropertyDiffs)
}

func TestConvertPlanSteps_DeleteNoDiffs(t *testing.T) {
	steps := []ingest.PulumiStep{
		{
			URN:  "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Op:   "delete",
			Type: "aws:ec2/instance:Instance",
			OldState: &ingest.PulumiState{
				Inputs: map[string]interface{}{
					"instanceType": "t3.large",
				},
			},
		},
	}

	result := convertPlanSteps(steps)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].PropertyDiffs)
}

func TestConvertPlanSteps_SameNoDiffs(t *testing.T) {
	steps := []ingest.PulumiStep{
		{
			URN:  "urn:pulumi:stack::proj::aws:ec2/instance:Instance::web",
			Op:   "same",
			Type: "aws:ec2/instance:Instance",
		},
	}

	result := convertPlanSteps(steps)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].PropertyDiffs)
}

// ---------------------------------------------------------------------------
// diffInputs
// ---------------------------------------------------------------------------

func TestDiffInputs_NilStates(t *testing.T) {
	assert.Nil(t, diffInputs(nil, nil))
	assert.Nil(t, diffInputs(&ingest.PulumiState{}, nil))
	assert.Nil(t, diffInputs(nil, &ingest.PulumiState{}))
}

func TestDiffInputs_EmptyInputs(t *testing.T) {
	oldS := &ingest.PulumiState{Inputs: map[string]interface{}{}}
	newS := &ingest.PulumiState{Inputs: map[string]interface{}{}}
	assert.Nil(t, diffInputs(oldS, newS))
}

func TestDiffInputs_IdenticalInputs(t *testing.T) {
	oldS := &ingest.PulumiState{Inputs: map[string]interface{}{"key": "value"}}
	newS := &ingest.PulumiState{Inputs: map[string]interface{}{"key": "value"}}
	assert.Empty(t, diffInputs(oldS, newS))
}

func TestDiffInputs_AddedKey(t *testing.T) {
	oldS := &ingest.PulumiState{Inputs: map[string]interface{}{}}
	newS := &ingest.PulumiState{Inputs: map[string]interface{}{"key": "value"}}
	diffs := diffInputs(oldS, newS)
	require.Len(t, diffs, 1)
	assert.Equal(t, "key", diffs[0].Key)
	assert.Equal(t, "", diffs[0].OldValue)
	assert.Equal(t, "value", diffs[0].NewValue)
}

func TestDiffInputs_RemovedKey(t *testing.T) {
	oldS := &ingest.PulumiState{Inputs: map[string]interface{}{"key": "value"}}
	newS := &ingest.PulumiState{Inputs: map[string]interface{}{}}
	diffs := diffInputs(oldS, newS)
	require.Len(t, diffs, 1)
	assert.Equal(t, "key", diffs[0].Key)
	assert.Equal(t, "value", diffs[0].OldValue)
	assert.Equal(t, "", diffs[0].NewValue)
}

func TestDiffInputs_SortedByKey(t *testing.T) {
	oldS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"zebra": "a",
			"alpha": "b",
		},
	}
	newS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"zebra": "x",
			"alpha": "y",
		},
	}
	diffs := diffInputs(oldS, newS)
	require.Len(t, diffs, 2)
	assert.Equal(t, "alpha", diffs[0].Key)
	assert.Equal(t, "zebra", diffs[1].Key)
}

func TestDiffInputs_ComplexValues(t *testing.T) {
	oldS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"tags": map[string]interface{}{"env": "staging"},
		},
	}
	newS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"tags": map[string]interface{}{"env": "prod"},
		},
	}
	diffs := diffInputs(oldS, newS)
	require.Len(t, diffs, 1)
	assert.Equal(t, "tags", diffs[0].Key)
	assert.Contains(t, diffs[0].OldValue, "staging")
	assert.Contains(t, diffs[0].NewValue, "prod")
}

func TestDiffInputs_TypeOnlyChange(t *testing.T) {
	// Regression: type-only changes (e.g., string "1" → float64 1) must be
	// detected as diffs, even though formatDiffValue produces the same string.
	oldS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"count":   "1",
			"enabled": "true",
		},
	}
	newS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"count":   float64(1),
			"enabled": true,
		},
	}
	diffs := diffInputs(oldS, newS)
	require.Len(t, diffs, 2)
	// Sorted by key: "count" then "enabled".
	assert.Equal(t, "count", diffs[0].Key)
	assert.Equal(t, "1", diffs[0].OldValue)
	assert.Equal(t, "1", diffs[0].NewValue)
	assert.Equal(t, "enabled", diffs[1].Key)
	assert.Equal(t, "true", diffs[1].OldValue)
	assert.Equal(t, "true", diffs[1].NewValue)
}

func TestDiffInputs_SkipsInternalKeys(t *testing.T) {
	oldS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"__defaults":   []interface{}{"a", "b"},
			"instanceType": "t3.medium",
		},
	}
	newS := &ingest.PulumiState{
		Inputs: map[string]interface{}{
			"__defaults":   []interface{}{"a"},
			"instanceType": "t3.large",
		},
	}
	diffs := diffInputs(oldS, newS)
	require.Len(t, diffs, 1)
	assert.Equal(t, "instanceType", diffs[0].Key, "__defaults should be filtered out")
}

// ---------------------------------------------------------------------------
// formatDiffValue
// ---------------------------------------------------------------------------

func TestFormatDiffValue(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"float64", float64(42.5), "42.5"},
		{"int", 7, "7"},
		{"map", map[string]interface{}{"a": "b"}, `{"a":"b"}`},
		{"slice", []interface{}{"x", "y"}, `["x","y"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatDiffValue(tt.val))
		})
	}
}
