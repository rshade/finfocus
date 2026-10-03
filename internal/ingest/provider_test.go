package ingest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

func TestMappedProviderIsTheBillingCloud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		{"aws", "aws:ec2/instance:Instance", "aws"},
		{"aws native", "aws-native:ec2:Instance", "aws"},
		{"azure native", "azure-native:compute:VirtualMachine", "azure"},
		{"google native", "google-native:compute/v1:Instance", "gcp"},
		{"kubernetes", "kubernetes:apps/v1:Deployment", "kubernetes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mapped, err := ingest.MapResource(ingest.PulumiResource{
				URN:  "urn:pulumi:dev::app::" + tt.resourceType + "::res",
				Type: tt.resourceType,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, mapped.Provider)
			assert.Equal(t, tt.resourceType, mapped.Type)

			state, err := ingest.MapStateResource(ingest.StackExportResource{
				URN:  "urn:pulumi:dev::app::" + tt.resourceType + "::res",
				Type: tt.resourceType,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, state.Provider)
			assert.Equal(t, tt.resourceType, state.Type)

			plan, err := ingest.ParsePulumiPlan([]byte(`{"steps":[{"op":"create","urn":"urn:pulumi:dev::app::` +
				tt.resourceType + `::res","type":"` + tt.resourceType + `","newState":{"type":"` +
				tt.resourceType + `","inputs":{}}}]}`))
			require.NoError(t, err)
			resources := plan.GetResources()
			require.Len(t, resources, 1)
			assert.Equal(t, tt.want, resources[0].Provider)
		})
	}
}

func TestMapTerraformResourceProviderIsTheBillingCloud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		resourceType string
		want         string
	}{
		{"aws_instance", "aws"},
		{"azurerm_linux_virtual_machine", "azure"},
		{"google_compute_instance", "gcp"},
		{"random_pet", "random"},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			t.Parallel()

			mapped, err := ingest.MapTerraformResource(
				ingest.TerraformStateResource{Mode: "managed", Type: tt.resourceType, Name: "x"},
				ingest.TerraformStateInstance{Attributes: map[string]any{}},
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want, mapped.Provider)
			assert.Equal(t, tt.resourceType, mapped.Type)
		})
	}
}

func TestMapTerraformResourceIsUnchangedInfrastructure(t *testing.T) {
	t.Parallel()

	mapped, err := ingest.MapTerraformResource(
		ingest.TerraformStateResource{Mode: "managed", Type: "aws_instance", Name: "web"},
		ingest.TerraformStateInstance{Attributes: map[string]any{}},
	)
	require.NoError(t, err)
	assert.Equal(t, "same", mapped.Operation)
}
