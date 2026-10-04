package ingest_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/proto"
)

const pulumiUnknown = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

func TestPropertyDependenciesMissingNullAndEmpty(t *testing.T) {
	t.Parallel()

	plan, err := ingest.ParsePulumiPlan([]byte(`{
		"steps": [
			{"op": "create", "urn": "urn:missing", "type": "azure:example:Child"},
			{"op": "create", "urn": "urn:null", "type": "azure:example:Child",
				"newState": {"type": "azure:example:Child", "urn": "urn:null",
					"parent": null, "dependencies": null, "propertyDependencies": null}},
			{"op": "create", "urn": "urn:empty", "type": "azure:example:Child",
				"newState": {"type": "azure:example:Child", "urn": "urn:empty",
					"dependencies": [],
					"propertyDependencies": {"serverId": [], "name": null}}}
		]
	}`))
	require.NoError(t, err)

	resources := plan.GetResources()
	require.Len(t, resources, 3)

	assert.Nil(t, resources[0].PropertyDependencies)
	assert.Empty(t, resources[0].Parent)
	assert.Nil(t, resources[0].Dependencies)

	assert.Nil(t, resources[1].PropertyDependencies)
	assert.Empty(t, resources[1].Parent)
	assert.Nil(t, resources[1].Dependencies)

	assert.NotNil(t, resources[2].PropertyDependencies["serverId"])
	assert.Empty(t, resources[2].PropertyDependencies["serverId"])
	assert.Nil(t, resources[2].PropertyDependencies["name"])
	assert.NotNil(t, resources[2].Dependencies)
	assert.Empty(t, resources[2].Dependencies)

	mapped, err := ingest.MapResources(resources)
	require.NoError(t, err)
	for _, desc := range mapped {
		assert.Nil(t, desc.Refs)
	}
}

func TestAzurePropertyDependenciesPreview(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/azure_property_dependencies.json")
	require.NoError(t, err)
	plan, err := ingest.ParsePulumiPlan(data)
	require.NoError(t, err)

	pulumiResources := plan.GetResources()
	byPulumi := make(map[string]ingest.PulumiResource, len(pulumiResources))
	for _, resource := range pulumiResources {
		byPulumi[resourceName(resource.URN)] = resource
	}

	sqlDB := byPulumi["sqlDb"]
	assert.Equal(t, "urn:pulumi:dev::demo::pulumi:pulumi:Stack::demo-dev", sqlDB.Parent)
	serverURN := "urn:pulumi:dev::demo::azure:mssql/server:Server::sqlServer"
	assert.Equal(t, []string{serverURN}, sqlDB.Dependencies)
	assert.Equal(t, []string{serverURN}, sqlDB.PropertyDependencies["serverId"])
	assert.Empty(t, sqlDB.PropertyDependencies["name"])

	deleted := byPulumi["deletedDb"]
	assert.Equal(t, []string{serverURN}, deleted.PropertyDependencies["serverId"])

	descriptors, err := ingest.MapResources(pulumiResources)
	require.NoError(t, err)
	engine.ApplyCrossResourceRefs(context.Background(), descriptors)

	prepared := make(map[string]*preparedResource, len(descriptors))
	for _, desc := range descriptors {
		request := proto.PrepareProjectedDescriptor(
			context.Background(), desc.ID, desc.Provider, desc.Type, engine.ConvertToProto(desc.Properties), nil,
		)
		prepared[resourceName(desc.ID)] = &preparedResource{desc: desc, request: request}
	}

	assertNoSentinel(t, prepared)

	sql := prepared["sqlDb"]
	assert.Equal(t, "westeurope", sql.request.GetRegion())
	assert.Empty(t, sql.request.GetSku())
	assert.Equal(t, "westeurope", sql.request.GetTags()["ref.serverId.region"])
	assert.Equal(t, "azure:mssql/server:Server", sql.request.GetTags()["ref.serverId.type"])
	assert.NotContains(t, sql.request.GetTags(), "serverId")
	assert.NotContains(t, sql.desc.Refs, "name")

	for _, name := range []string{"linuxApp", "windowsApp", "linuxFunc", "windowsFunc"} {
		app := prepared[name]
		assert.Equal(t, "S1", app.request.GetTags()["ref.servicePlanId.sku"], name)
		assert.Equal(t, "eastus", app.request.GetRegion(), name)
		assert.Empty(t, app.request.GetSku(), name)
		assert.NotContains(t, app.request.GetTags(), "servicePlanId")
	}

	for _, name := range []string{"nativeAppA", "nativeAppB"} {
		app := prepared[name]
		assert.Equal(t, "P1v3", app.request.GetTags()["ref.serverFarmId.sku"], name)
		assert.Empty(t, app.request.GetSku(), name)
		assert.Equal(t, "westeurope", app.request.GetRegion(), name)
	}
	assert.Equal(t, "P1v3", prepared["nativePlan"].request.GetSku())
	assert.Empty(t, prepared["classicPlan"].request.GetSku())

	assert.Equal(t, "westeurope", prepared["classicPool"].request.GetRegion())
	assert.Equal(t, "Standard_D2s_v3", prepared["classicPool"].request.GetSku())
	assert.Equal(t, "westeurope", prepared["classicPool"].request.GetTags()["ref.kubernetesClusterId.region"])

	assert.Equal(t, "westeurope", prepared["nativePool"].request.GetRegion())
	assert.Equal(t, "Standard_D4s_v3", prepared["nativePool"].request.GetSku())
	assert.Equal(t, "westeurope", prepared["nativePool"].request.GetTags()["ref.resourceName.region"])

	assert.Equal(t, "westeurope", prepared["cosmosDb"].request.GetRegion())
	assert.Equal(t, "westeurope", prepared["cosmosDb"].request.GetTags()["ref.accountName.region"])
	assert.Equal(t, "cosmos-probe", prepared["cosmosDb"].request.GetTags()["accountName"])

	assert.Equal(t, "westeurope", prepared["cosmosContainer"].request.GetRegion())
	assert.NotContains(t, prepared["cosmosContainer"].request.GetTags(), "ref.databaseName.urn")
	assert.Contains(t, prepared["cosmosContainer"].request.GetTags(), "ref.accountName.urn")

	assert.Equal(t, "northeurope", prepared["nativeCosmosDb"].request.GetRegion())
	assert.Equal(t, "westeurope", prepared["nativeCosmosDb"].request.GetTags()["ref.accountName.region"])
	assert.NotContains(t, prepared["nativeCosmosDb"].request.GetTags(), "accountName")

	assert.Empty(t, prepared["nativeCosmosContainer"].request.GetRegion())
	assert.Equal(t, "westeurope", prepared["nativeCosmosContainer"].request.GetTags()["ref.accountName.region"])
	assert.Equal(t, "northeurope", prepared["nativeCosmosContainer"].request.GetTags()["ref.databaseName.region"])

	assert.Equal(t, "northeurope", prepared["nativeSqlDb"].request.GetRegion())
	assert.Equal(t, "westeurope", prepared["nativeSqlDb"].request.GetTags()["ref.serverName.region"])

	negative := prepared["negative"]
	assert.Empty(t, negative.request.GetRegion())
	assert.Empty(t, negative.request.GetSku())
	for key := range negative.request.GetTags() {
		assert.NotContains(t, key, "ref.")
	}
	assert.NotContains(t, negative.desc.Refs, "name")
	assert.Contains(t, negative.desc.Refs, "gone")
	assert.Contains(t, negative.desc.Refs, "multi")

	assert.Equal(t, "westeurope", prepared["deletedDb"].request.GetRegion())
	assert.Contains(t, prepared["deletedDb"].request.GetTags(), "ref.serverId.region")
}

func TestAzurePropertyDependenciesState(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/azure_property_dependencies_state.json")
	require.NoError(t, err)
	state, err := ingest.ParseStackExport(data)
	require.NoError(t, err)

	descriptors, err := ingest.MapStateResources(state.GetCustomResources())
	require.NoError(t, err)
	engine.ApplyCrossResourceRefs(context.Background(), descriptors)

	byName := map[string]engine.ResourceDescriptor{}
	for _, desc := range descriptors {
		byName[resourceName(desc.ID)] = desc
	}

	sqlDB := byName["sqlDb"]
	assert.Equal(t, []string{"urn:pulumi:dev::demo::azure:mssql/server:Server::sqlServer"}, sqlDB.Refs["serverId"])
	assert.Equal(t, "westeurope", sqlDB.Properties["ref.serverId.region"])

	idOnly := byName["idOnly"]
	assert.Nil(t, idOnly.Refs)
	assert.NotContains(t, idOnly.Properties, "ref.serverId.region")
	assert.Equal(t, byName["sqlServer"].Properties["pulumi:cloudId"], idOnly.Properties["serverId"])
}

type preparedResource struct {
	desc    engine.ResourceDescriptor
	request interface {
		GetRegion() string
		GetSku() string
		GetTags() map[string]string
	}
}

func assertNoSentinel(t *testing.T, prepared map[string]*preparedResource) {
	t.Helper()
	for name, item := range prepared {
		assert.NotEqual(t, pulumiUnknown, item.request.GetRegion(), name)
		assert.NotEqual(t, pulumiUnknown, item.request.GetSku(), name)
		for key, value := range item.request.GetTags() {
			assert.NotContains(t, value, pulumiUnknown, "%s tag %s", name, key)
		}
	}
}

func resourceName(urn string) string {
	if i := strings.LastIndex(urn, "::"); i >= 0 {
		return urn[i+2:]
	}
	return urn
}
