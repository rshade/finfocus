package stackload

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/sourcepos"
)

const sourceposFixtures = "../../testdata/sourcepos"

func sourceposRuntimes() []string { return []string{"nodejs", "go", "python", "yaml"} }

type expectedLocations struct {
	State struct {
		Resources map[string]*sourcepos.Location `json:"resources"`
	} `json:"state"`
	Preview struct {
		Resources map[string]struct {
			Location *sourcepos.Location `json:"location"`
		} `json:"resources"`
	} `json:"preview"`
}

func loadExpected(t *testing.T, runtime string) expectedLocations {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(sourceposFixtures, runtime, "expected.json"))
	require.NoError(t, err)
	var expected expectedLocations
	require.NoError(t, json.Unmarshal(data, &expected))
	return expected
}

func resourceName(urn string) string {
	return urn[strings.LastIndex(urn, "::")+2:]
}

func TestLoadState_ResolvesRecordedFixtureLocations(t *testing.T) {
	t.Parallel()

	for _, runtime := range sourceposRuntimes() {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(sourceposFixtures, runtime)
			expected := loadExpected(t, runtime)

			snapshot, err := LoadState(context.Background(), Source{
				ProjectDir: filepath.Join(dir, "program"),
				StatePath:  filepath.Join(dir, "state.json"),
			})
			require.NoError(t, err)
			require.NotEmpty(t, snapshot.State)

			for _, res := range snapshot.State {
				want, recorded := expected.State.Resources[resourceName(res.URN)]
				require.True(t, recorded, "no expectation for %s", res.URN)
				assert.Equal(t, want, res.Location, res.URN)
			}
		})
	}
}

func TestLoadState_WithoutProjectDirResolvesNothing(t *testing.T) {
	t.Parallel()

	snapshot, err := LoadState(context.Background(), Source{
		StatePath: filepath.Join(sourceposFixtures, "nodejs", "state.json"),
	})
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.State)
	for _, res := range snapshot.State {
		assert.Nil(t, res.Location, res.URN)
	}
}

func TestLoadPlan_ResolvesRecordedFixtureLocations(t *testing.T) {
	t.Parallel()

	for _, runtime := range sourceposRuntimes() {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(sourceposFixtures, runtime)
			expected := loadExpected(t, runtime)

			snapshot, err := LoadPlan(context.Background(), Source{
				ProjectDir: filepath.Join(dir, "program-edited"),
				PlanPath:   filepath.Join(dir, "preview.json"),
			})
			require.NoError(t, err)
			require.NotEmpty(t, snapshot.Steps)

			for _, step := range snapshot.Steps {
				want, recorded := expected.Preview.Resources[resourceName(step.URN)]
				require.True(t, recorded, "no expectation for %s", step.URN)
				assert.Equal(t, want.Location, step.Location, step.URN)
			}
		})
	}
}

func TestSnapshotRows_PreviewLocationWinsOverMovedStateLocation(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(sourceposFixtures, "go")
	expected := loadExpected(t, "go")
	src := Source{
		ProjectDir: filepath.Join(dir, "program-edited"),
		StatePath:  filepath.Join(dir, "state.json"),
	}
	state, err := LoadState(context.Background(), src)
	require.NoError(t, err)
	plan, err := LoadPlan(
		context.Background(),
		Source{ProjectDir: src.ProjectDir, PlanPath: filepath.Join(dir, "preview.json")},
	)
	require.NoError(t, err)

	rows, err := state.WithPlan(plan.Plan).Rows(context.Background())
	require.NoError(t, err)
	byName := map[string]engine.OverviewRow{}
	for _, row := range rows {
		byName[resourceName(row.URN)] = row
	}

	web := byName["web"]
	require.NotNil(t, web.Location)
	assert.Equal(t, &sourcepos.Location{File: "main.go", Line: 14}, web.Location, "web moved from line 12 to 14")
	assert.Equal(t, expected.Preview.Resources["web"].Location, web.Location)
	assert.Equal(
		t,
		&sourcepos.Location{File: "main.go", Line: 21},
		byName["extra"].Location,
		"a create takes its preview location",
	)
}

func TestConvertStateResources_LocationHandling(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.js"), nil, 0o600))
	resolver := sourcepos.NewResolver(root, "nodejs")

	located := ingest.StackExportResource{
		URN: "urn:a", Type: "aws:ec2/instance:Instance", Custom: true, ID: "i-1",
		Inputs:         map[string]any{"instanceType": "t3.micro"},
		SourcePosition: "project:///index.js#4,1",
		StackTrace:     []ingest.StackFrame{{SourcePosition: "project:///index.js#4,1"}},
	}
	bare := located
	bare.SourcePosition = ""
	bare.StackTrace = nil
	outside := located
	outside.SourcePosition = "project:///../elsewhere.js#4,1"
	outside.StackTrace = nil

	got := convertStateResources([]ingest.StackExportResource{located, bare, outside}, resolver)
	require.Len(t, got, 3)
	assert.Equal(t, &sourcepos.Location{File: "index.js", Line: 4, Column: 1}, got[0].Location)
	assert.Nil(t, got[1].Location)
	assert.Nil(t, got[2].Location)

	assert.Equal(t, got[1].Properties, got[0].Properties, "a position is not a pricing property")
	assert.Equal(t, got[1].ID, got[0].ID)

	unresolved := convertStateResources([]ingest.StackExportResource{located}, nil)
	require.Len(t, unresolved, 1)
	assert.Nil(t, unresolved[0].Location, "no resolver means no location, not a panic")
}

func TestConvertPlanSteps_LocationComesFromTheCurrentDeclarationOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.js"), nil, 0o600))
	resolver := sourcepos.NewResolver(root, "nodejs")

	at := func(line string) *ingest.PulumiState {
		return &ingest.PulumiState{
			Type: "aws:s3/bucket:Bucket", Inputs: map[string]any{"bucket": "b"},
			SourcePosition: "project:///index.js#" + line,
		}
	}
	steps := []ingest.PulumiStep{
		{URN: "urn:create", Op: "create", NewState: at("10,1")},
		{URN: "urn:update", Op: "update", OldState: at("3,1"), NewState: at("12,1")},
		{URN: "urn:same", Op: "same", NewState: at("14,1")},
		{URN: "urn:replace", Op: "replace", OldState: at("3,1"), NewState: at("16,1")},
		{URN: "urn:create-replacement", Op: "create-replacement", NewState: at("16,1")},
		{URN: "urn:delete", Op: "delete", OldState: at("5,1")},
		{URN: "urn:delete-replaced", Op: "delete-replaced", OldState: at("3,1")},
		{
			URN:      "urn:unresolved",
			Op:       "update",
			OldState: at("3,1"),
			NewState: &ingest.PulumiState{Type: "aws:s3/bucket:Bucket"},
		},
	}

	got := convertPlanSteps(steps, resolver)
	want := map[string]*sourcepos.Location{
		"urn:create":             {File: "index.js", Line: 10, Column: 1},
		"urn:update":             {File: "index.js", Line: 12, Column: 1},
		"urn:same":               {File: "index.js", Line: 14, Column: 1},
		"urn:replace":            {File: "index.js", Line: 16, Column: 1},
		"urn:create-replacement": {File: "index.js", Line: 16, Column: 1},
		"urn:delete":             nil,
		"urn:delete-replaced":    nil,
		"urn:unresolved":         nil,
	}
	require.Len(t, got, len(want))
	for _, step := range got {
		assert.Equal(t, want[step.URN], step.Location, step.URN)
	}
}

func TestConvertPlanSteps_LocationLeavesPricingInputsUntouched(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.js"), nil, 0o600))
	resolver := sourcepos.NewResolver(root, "nodejs")

	build := func(position string) []ingest.PulumiStep {
		return []ingest.PulumiStep{{
			URN: "urn:a", Op: "update",
			OldState: &ingest.PulumiState{ID: "i-1", Inputs: map[string]any{"size": "small"}},
			NewState: &ingest.PulumiState{
				ID: "i-1", Inputs: map[string]any{"size": "large"},
				SourcePosition: position,
				StackTrace:     []ingest.StackFrame{{SourcePosition: position}},
			},
		}}
	}

	with := convertPlanSteps(build("project:///index.js#4,1"), resolver)
	without := convertPlanSteps(build(""), resolver)
	require.Len(t, with, 1)
	require.Len(t, without, 1)
	assert.NotNil(t, with[0].Location)
	assert.Nil(t, without[0].Location)

	with[0].Location = nil
	assert.Equal(t, without[0], with[0], "location must not change properties, diffs, or ids")
}

func TestSnapshotRows_UnchangedResourceTakesCurrentLocationAndIsNotOmitted(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(sourceposFixtures, "go")
	src := Source{
		ProjectDir: filepath.Join(dir, "program-edited"),
		StatePath:  filepath.Join(dir, "state.json"),
	}
	state, err := LoadState(context.Background(), src)
	require.NoError(t, err)
	plan, err := LoadPlan(
		context.Background(),
		Source{ProjectDir: src.ProjectDir, PlanPath: filepath.Join(dir, "preview.json")},
	)
	require.NoError(t, err)

	merged := state.WithPlan(plan.Plan)
	var sameSteps int
	for _, step := range merged.Steps {
		if step.Op == "same" {
			sameSteps++
		}
	}
	require.Positive(t, sameSteps, "the fixture lists unchanged resources")
	assert.Zero(t, merged.OmittedUnchanged, "every unchanged resource is listed, so none is omitted")
	assert.Empty(t, merged.Mismatches)
}
