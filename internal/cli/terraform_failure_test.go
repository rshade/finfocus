package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

func TestRunnerPlanSizeHint(t *testing.T) {
	t.Parallel()
	_, err := descriptorsFromPlan(context.Background(), nil, make([]byte, ingest.TerraformPlanMaxBytes+1), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenTofu plan failed.")
	assert.Contains(t, err.Error(), "32 MiB")
}

func TestTerraformPlanFileSkippedProviderNote(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	resources, _, err := loadTerraformPlanFile(context.Background(), cmd, "../ingest/testdata/terraform/plan/show.json")
	require.NoError(t, err)
	assert.NotEmpty(t, resources)
	assert.Empty(t, stdout.String())
	for _, provider := range []string{"random", "null", "helm", "time", "tls", "local", "archive"} {
		assert.Contains(t, stderr.String(), " "+provider+" resources: no cost.")
	}
}

func TestForecastTerraformNoOpSkipsPricing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "no-op.json")
	require.NoError(
		t,
		os.WriteFile(
			path,
			[]byte(
				`{"format_version":"1.2","resource_changes":[{"address":"aws_instance.same","mode":"managed","type":"aws_instance","change":{"actions":["no-op"],"after":{"instance_type":"t3.micro"}}}]}`,
			),
			0o600,
		),
	)
	cmd := NewCostForecastCmd()
	require.NoError(t, cmd.Flags().Set("terraform-plan", path))
	costs, err := forecastCosts(context.Background(), cmd, forecastDeps{}, nil)
	require.NoError(t, err)
	assert.Empty(t, costs)
}
