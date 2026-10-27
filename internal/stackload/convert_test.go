package stackload

import (
	"context"

	"github.com/rshade/finfocus/internal/ingest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"
	"time"
)

func TestProjectedPropertiesPreserveNestedState(t *testing.T) {
	t.Parallel()
	old := map[string]any{
		"nested":  map[string]any{"keep": "old", "size": "small"},
		"list":    []any{map[string]any{"keep": "value"}},
		"replace": "scalar",
	}
	step := ingest.PulumiStep{
		OldState: &ingest.PulumiState{Outputs: old},
		NewState: &ingest.PulumiState{
			Inputs: map[string]any{"nested": map[string]any{"size": "large"}, "replace": map[string]any{"x": true}},
		},
	}
	got := projectedPropertiesFromStep(step)
	assert.Equal(t, map[string]any{"keep": "old", "size": "large"}, got["nested"])
	got["list"].([]any)[0].(map[string]any)["keep"] = "mutated"
	assert.Equal(t, "value", old["list"].([]any)[0].(map[string]any)["keep"])
	assert.Equal(
		t,
		map[string]any{"legacy": true},
		projectedPropertiesFromStep(ingest.PulumiStep{Inputs: map[string]any{"legacy": true}}),
	)
	assert.Nil(t, projectedPropertiesFromStep(ingest.PulumiStep{}))
}
func TestInputDiffsPresenceAndFormatting(t *testing.T) {
	t.Parallel()
	assert.Nil(t, diffInputs(nil, nil))
	assert.Nil(t, diffInputs(&ingest.PulumiState{}, &ingest.PulumiState{}))
	diffs := diffInputs(
		&ingest.PulumiState{
			Inputs: map[string]any{
				"__defaults": "skip",
				"same":       true,
				"remove":     nil,
				"bool":       false,
				"map":        map[string]any{"x": 1},
				"bad":        make(chan int),
			},
		},
		&ingest.PulumiState{
			Inputs: map[string]any{
				"__defaults": "changed",
				"same":       true,
				"bool":       true,
				"number":     1,
				"map":        map[string]any{"x": 2},
			},
		},
	)
	require.Len(t, diffs, 5)
	assert.Equal(t, "bad", diffs[0].Key)
	assert.Equal(t, "false", diffs[1].OldValue)
	assert.Equal(t, `{"x":2}`, diffs[2].NewValue)
	assert.Equal(t, "1", diffs[3].NewValue)
	assert.Equal(t, "remove", diffs[4].Key)
}
func TestStateTimestampAndStackIdentity(t *testing.T) {
	t.Parallel()
	now := time.Now()
	state := sampleState()
	state[0].Created = &now
	converted := convertStateResources(state)
	require.NotNil(t, converted[0].CreatedAt)
	assert.Equal(t, now, *converted[0].CreatedAt)
	s := &Snapshot{State: converted}
	rows, err := s.Rows(context.Background())
	require.NoError(t, err)
	*rows[0].CreatedAt = time.Time{}
	assert.Equal(t, now, *s.State[0].CreatedAt)
	for _, tc := range []struct{ raw, manifestTime, stackName string }{
		{
			raw:          `{"deployment":{"manifest":{"time":"today"},"resources":[{"type":"pulumi:pulumi:Stack","urn":"urn:pulumi:dev::project::pulumi:pulumi:Stack::project-dev"}]}}`,
			manifestTime: "today",
			stackName:    "dev",
		},
		{raw: `{"deployment":{"resources":[{"type":"pulumi:pulumi:Stack","urn":"invalid"}]}}`},
	} {
		loaded, loadErr := LoadState(context.Background(), Source{StatePath: inputFile(t, "state.json", tc.raw)})
		require.NoError(t, loadErr)
		assert.Equal(t, tc.manifestTime, loaded.ManifestTime)
		assert.Equal(t, tc.stackName, loaded.StackName)
	}
	_, err = LoadState(context.Background(), Source{})
	require.Error(t, err)
}
func TestDescriptorStateShowSamesParity(t *testing.T) {
	t.Parallel()
	raw := sampleState()[0]
	p := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				URN:      raw.URN,
				Type:     raw.Type,
				Op:       "same",
				NewState: &ingest.PulumiState{Inputs: raw.Inputs, Outputs: raw.Outputs},
			},
		},
	}
	state := (&Snapshot{RawState: []ingest.StackExportResource{raw}}).Descriptors()[0]
	plan := (&Snapshot{}).WithPlan(p).Descriptors()[0]
	for key := range state.Properties {
		if len(key) >= 7 && key[:7] == "pulumi:" {
			delete(state.Properties, key)
		}
	}
	state.Operation = plan.Operation
	assert.Equal(t, plan, state)
}

func TestLoadStateInfersStackFromRootURN(t *testing.T) {
	t.Parallel()
	const state = `{"deployment":{"resources":[{"type":"pulumi:pulumi:Stack","urn":"urn:pulumi:dev::test-stack::pulumi:pulumi:Stack::test-stack-dev"}]}}`
	path := inputFile(t, "stack.json", state)
	snapshot, err := LoadState(context.Background(), Source{StatePath: path})
	require.NoError(t, err)
	assert.Equal(t, "dev", snapshot.StackName)
	explicit, err := LoadState(context.Background(), Source{StatePath: path, Stack: "explicit"})
	require.NoError(t, err)
	assert.Equal(t, "explicit", explicit.StackName)
}
