package stackload

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/proto"
)

const (
	testProviderURN  = "urn:pulumi:dev::app::pulumi:providers:aws::default_us-east-1"
	testProviderID   = "provider-id-123"
	testInstanceURN  = "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"
	testProviderRef  = testProviderURN + "::" + testProviderID
	testInstanceType = "aws:ec2/instance:Instance"
)

func awsProviderResource(inputs map[string]any) ingest.StackExportResource {
	return ingest.StackExportResource{
		URN:    testProviderURN,
		Type:   "pulumi:providers:aws",
		Custom: true,
		ID:     testProviderID,
		Inputs: inputs,
	}
}

func regionlessInstance(provider string, inputs map[string]any) ingest.StackExportResource {
	props := map[string]any{"instanceType": "t3.micro", "subnetId": "subnet-1"}
	for k, v := range inputs {
		props[k] = v
	}
	return ingest.StackExportResource{
		URN:      testInstanceURN,
		Type:     testInstanceType,
		Custom:   true,
		ID:       "i-1",
		Provider: provider,
		Inputs:   props,
	}
}

func instanceDescriptor(t *testing.T, descriptors []engine.ResourceDescriptor) engine.ResourceDescriptor {
	t.Helper()
	return descriptorFor(t, descriptors, testInstanceURN)
}

func descriptorFor(t *testing.T, descriptors []engine.ResourceDescriptor, urn string) engine.ResourceDescriptor {
	t.Helper()
	for _, d := range descriptors {
		if d.ID == urn {
			return d
		}
	}
	require.Failf(t, "descriptor missing", "no descriptor for %s", urn)
	return engine.ResourceDescriptor{}
}

func resolvedRegion(d engine.ResourceDescriptor) string {
	props := make(map[string]string, len(d.Properties))
	for k, v := range d.Properties {
		if s, ok := v.(string); ok {
			props[k] = s
		}
	}
	_, region := proto.ResolveSKUAndRegion(context.Background(), d.Provider, d.Type, props)
	return region
}

func previewWithProvider(providerRegion string) *ingest.PulumiPlan {
	return &ingest.PulumiPlan{Steps: []ingest.PulumiStep{
		{
			Op:   "create",
			URN:  testProviderURN,
			Type: "pulumi:providers:aws",
			NewState: &ingest.PulumiState{
				Type:   "pulumi:providers:aws",
				URN:    testProviderURN,
				Inputs: map[string]any{"region": providerRegion},
			},
		},
		{
			Op:   "create",
			URN:  testInstanceURN,
			Type: testInstanceType,
			NewState: &ingest.PulumiState{
				Type:     testInstanceType,
				URN:      testInstanceURN,
				Provider: testProviderURN + "::" + proto.UnknownPulumiValue,
				Inputs:   map[string]any{"instanceType": "t3.micro", "subnetId": "subnet-1"},
			},
		},
	}}
}

func TestProviderRegionFromStateOverridesEnvironment(t *testing.T) {
	state := []ingest.StackExportResource{
		awsProviderResource(map[string]any{"region": "us-east-1"}),
		regionlessInstance(testProviderRef, nil),
	}
	snapshot := &Snapshot{RawState: state, StateStatus: Status{Loaded: true}}

	for name, descriptors := range map[string][]engine.ResourceDescriptor{
		"deployed":  snapshot.DeployedDescriptors(),
		"inventory": snapshot.Descriptors(),
		"actual":    snapshot.ActualDescriptors(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("AWS_REGION", "us-west-2")
			desc := instanceDescriptor(t, descriptors)
			assert.Equal(t, "us-east-1", desc.Properties[proto.ProviderRegionProperty])
			assert.Equal(t, "us-east-1", resolvedRegion(desc))
		})
	}
}

func TestProviderRegionResolutionPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		inputs     map[string]any
		wantRegion string
	}{
		{name: "provider region", provider: testProviderRef, wantRegion: "us-east-1"},
		{
			name:       "explicit region wins",
			provider:   testProviderRef,
			inputs:     map[string]any{"region": "us-west-2"},
			wantRegion: "us-west-2",
		},
		{
			name:       "availability zone wins",
			provider:   testProviderRef,
			inputs:     map[string]any{"availabilityZone": "us-west-2a"},
			wantRegion: "us-west-2",
		},
		{
			name:       "arn wins",
			provider:   testProviderRef,
			inputs:     map[string]any{"arn": "arn:aws:ec2:us-west-2:123456789012:instance/i-1"},
			wantRegion: "us-west-2",
		},
		{
			name:       "missing provider resource falls back to environment",
			provider:   "urn:pulumi:dev::app::pulumi:providers:aws::gone::x",
			wantRegion: "us-west-2",
		},
		{name: "no provider reference falls back to environment", wantRegion: "us-west-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWS_REGION", "us-west-2")
			state := []ingest.StackExportResource{
				awsProviderResource(map[string]any{"region": "us-east-1"}),
				regionlessInstance(tc.provider, tc.inputs),
			}
			desc := instanceDescriptor(t, (&Snapshot{RawState: state}).DeployedDescriptors())
			assert.Equal(t, tc.wantRegion, resolvedRegion(desc))
		})
	}
}

func TestProviderRegionFromPreviewOnlyCreate(t *testing.T) {
	t.Parallel()
	snapshot := (&Snapshot{}).WithPlan(previewWithProvider("us-east-1"))

	desc := instanceDescriptor(t, snapshot.Descriptors())
	assert.Equal(t, "us-east-1", desc.Properties[proto.ProviderRegionProperty])
	require.Len(t, snapshot.Steps, 2)
	assert.Equal(t, "us-east-1", snapshot.Steps[1].ProjectedProperties[proto.ProviderRegionProperty])
}

func TestProviderRegionScope(t *testing.T) {
	t.Parallel()
	secret := map[string]any{"4dabf18193072939515e22adb298388d": "1", "ciphertext": "cipher"}
	azureURN := "urn:pulumi:dev::app::azure-native:compute:VirtualMachine::vm"
	cases := []struct {
		name      string
		providers []ingest.StackExportResource
		resource  ingest.StackExportResource
		wantSet   bool
	}{
		{
			name:      "aws region from the provider",
			providers: []ingest.StackExportResource{awsProviderResource(map[string]any{"region": "us-east-1"})},
			resource:  regionlessInstance(testProviderRef, nil),
			wantSet:   true,
		},
		{
			name: "provider output region is used when its inputs name none",
			providers: []ingest.StackExportResource{{
				URN: testProviderURN, Type: "pulumi:providers:aws", Custom: true, ID: testProviderID,
				Outputs: map[string]any{"region": "us-east-1"},
			}},
			resource: regionlessInstance(testProviderRef, nil),
			wantSet:  true,
		},
		{
			name:      "secret provider region is ignored",
			providers: []ingest.StackExportResource{awsProviderResource(map[string]any{"region": secret})},
			resource:  regionlessInstance(testProviderRef, nil),
		},
		{
			name: "unknown provider region is ignored",
			providers: []ingest.StackExportResource{
				awsProviderResource(map[string]any{"region": proto.UnknownPulumiValue}),
			},
			resource: regionlessInstance(testProviderRef, nil),
		},
		{
			name: "azure provider region does not reach an aws resource",
			providers: []ingest.StackExportResource{{
				URN: testProviderURN, Type: "pulumi:providers:azure-native", Custom: true, ID: testProviderID,
				Inputs: map[string]any{"region": "eastus"},
			}},
			resource: regionlessInstance(testProviderRef, nil),
		},
		{
			name:      "non-aws resource is unaffected",
			providers: []ingest.StackExportResource{awsProviderResource(map[string]any{"region": "us-east-1"})},
			resource: ingest.StackExportResource{
				URN: azureURN, Type: "azure-native:compute:VirtualMachine", Custom: true, ID: "vm-1",
				Provider: testProviderRef, Inputs: map[string]any{"vmSize": "Standard_B1s"},
			},
			wantSet: false,
		},
		{
			name:      "aws resource naming its own region is still annotated, and the adapter prefers its region",
			providers: []ingest.StackExportResource{awsProviderResource(map[string]any{"region": "us-east-1"})},
			resource:  regionlessInstance(testProviderRef, map[string]any{"region": "us-west-2"}),
			wantSet:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := append(append([]ingest.StackExportResource{}, tc.providers...), tc.resource)
			desc := descriptorFor(t, (&Snapshot{RawState: state}).DeployedDescriptors(), tc.resource.URN)
			_, isSet := desc.Properties[proto.ProviderRegionProperty]
			assert.Equal(t, tc.wantSet, isSet)
		})
	}
}

func TestProviderRegionChangesProjectedCacheKey(t *testing.T) {
	t.Parallel()
	keyFor := func(region string) string {
		state := []ingest.StackExportResource{
			awsProviderResource(map[string]any{"region": region}),
			regionlessInstance(testProviderRef, nil),
		}
		desc := instanceDescriptor(t, (&Snapshot{RawState: state}).DeployedDescriptors())
		key, err := engine.ProjectedResourceCacheKey(desc)
		require.NoError(t, err)
		return key
	}
	east := keyFor("us-east-1")
	assert.Equal(t, east, keyFor("us-east-1"))
	assert.NotEqual(t, east, keyFor("us-west-2"))
}

func TestProviderRegionIsNotShownAsAnInput(t *testing.T) {
	t.Parallel()
	state := []ingest.StackExportResource{
		awsProviderResource(map[string]any{"region": "us-east-1"}),
		regionlessInstance(testProviderRef, nil),
	}
	desc := instanceDescriptor(t, (&Snapshot{RawState: state}).DeployedDescriptors())
	require.Contains(t, desc.Properties, proto.ProviderRegionProperty)

	attrs := engine.BuildAttributes(context.Background(), desc.Properties)
	assert.NotContains(t, attrs.AsMap(), proto.ProviderRegionProperty)
	assert.NotContains(t, attrs.AsMap(), "region")
}

func TestProviderRegionDeployedStateIgnoresPreviewProviderChange(t *testing.T) {
	t.Parallel()
	state := []ingest.StackExportResource{
		awsProviderResource(map[string]any{"region": "us-east-1"}),
		regionlessInstance(testProviderRef, nil),
	}
	plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{{
		Op:   "update",
		URN:  testProviderURN,
		Type: "pulumi:providers:aws",
		NewState: &ingest.PulumiState{
			Type: "pulumi:providers:aws", URN: testProviderURN,
			Inputs: map[string]any{"region": "us-west-2"},
		},
	}}}
	snapshot := (&Snapshot{RawState: state, StateStatus: Status{Loaded: true}}).WithPlan(plan)

	assert.Equal(
		t,
		"us-east-1",
		instanceDescriptor(t, snapshot.DeployedDescriptors()).Properties[proto.ProviderRegionProperty],
	)
	assert.Equal(t, "us-west-2", instanceDescriptor(t, snapshot.Descriptors()).Properties[proto.ProviderRegionProperty])
	require.Len(t, snapshot.State, 2)
	assert.Equal(t, "us-west-2", snapshot.State[1].Properties[proto.ProviderRegionProperty])
}

func TestProviderRegionSkipsDeleteSteps(t *testing.T) {
	t.Parallel()
	state := []ingest.StackExportResource{
		awsProviderResource(map[string]any{"region": "us-east-1"}),
		regionlessInstance(testProviderRef, nil),
	}
	plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{{
		Op:   "delete",
		URN:  testInstanceURN,
		Type: testInstanceType,
		OldState: &ingest.PulumiState{
			Type:     testInstanceType,
			URN:      testInstanceURN,
			Provider: testProviderRef,
			Inputs:   map[string]any{"instanceType": "t3.micro"},
		},
	}}}
	snapshot := (&Snapshot{RawState: state, StateStatus: Status{Loaded: true}}).WithPlan(plan)

	require.Len(t, snapshot.Steps, 1)
	assert.Nil(t, snapshot.Steps[0].ProjectedProperties)
}
