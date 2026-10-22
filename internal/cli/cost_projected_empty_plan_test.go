package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

type emptyPlanDocument struct {
	Finfocus struct {
		EmptyPlan *bool             `json:"emptyPlan"`
		Resources []json.RawMessage `json:"resources"`
	} `json:"finfocus"`
}

func runProjectedPlan(t *testing.T, planPath string, extra ...string) (emptyPlanDocument, string) {
	t.Helper()
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())
	isolateConfig(t)
	isolateFromPulumiProject(t)

	args := append([]string{"cost", "projected", "--pulumi-json", planPath, "--output", "json"}, extra...)
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), args)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)

	var doc emptyPlanDocument
	require.NoError(t, json.Unmarshal(result.Stdout, &doc))
	return doc, string(result.Stderr)
}

//nolint:paralleltest // sets environment variables and changes the working directory
func TestCostProjected_UnchangedPulumiPlanIsFlaggedEmpty(t *testing.T) {
	planPath, err := filepath.Abs("testdata/pulumi/unchanged-stack-preview.json")
	require.NoError(t, err)

	doc, stderr := runProjectedPlan(t, planPath)

	require.NotNil(t, doc.Finfocus.EmptyPlan)
	assert.True(t, *doc.Finfocus.EmptyPlan)
	assert.Empty(t, doc.Finfocus.Resources)
	assert.Contains(t, stderr, "no resources to price")
	assert.Contains(t, stderr, "--show-sames")
}

//nolint:paralleltest // sets environment variables and changes the working directory
func TestCostProjected_PlanWithResourcesIsNotFlaggedEmpty(t *testing.T) {
	planPath, err := filepath.Abs("../../test/fixtures/plans/aws-simple-plan.json")
	require.NoError(t, err)

	doc, stderr := runProjectedPlan(t, planPath)

	assert.Nil(t, doc.Finfocus.EmptyPlan, "emptyPlan is omitted when the plan has resources")
	assert.NotEmpty(t, doc.Finfocus.Resources)
	assert.NotContains(t, stderr, "no resources to price")
}

//nolint:paralleltest // sets environment variables and changes the working directory
func TestCostProjected_FilterThatRemovesEverythingIsNotFlaggedEmpty(t *testing.T) {
	planPath, err := filepath.Abs("../../test/fixtures/plans/aws-simple-plan.json")
	require.NoError(t, err)

	doc, stderr := runProjectedPlan(t, planPath, "--filter", "type=does:not/exist:Type")

	assert.Nil(t, doc.Finfocus.EmptyPlan, "an empty result from --filter is the filter's doing")
	assert.NotContains(t, stderr, "no resources to price")
}
