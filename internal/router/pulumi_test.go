package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInternalPulumiType(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         bool
	}{
		{
			name:         "pulumi stack resource",
			resourceType: "pulumi:pulumi:Stack",
			want:         true,
		},
		{
			name:         "pulumi AWS provider",
			resourceType: "pulumi:providers:aws",
			want:         true,
		},
		{
			name:         "pulumi Azure provider",
			resourceType: "pulumi:providers:azure",
			want:         true,
		},
		{
			name:         "pulumi GCP provider",
			resourceType: "pulumi:providers:gcp",
			want:         true,
		},
		{
			name:         "AWS EC2 instance",
			resourceType: "aws:ec2/instance:Instance",
			want:         false,
		},
		{
			name:         "Azure VM",
			resourceType: "azure:compute/virtualMachine:VirtualMachine",
			want:         false,
		},
		{
			name:         "GCP compute instance",
			resourceType: "gcp:compute/instance:Instance",
			want:         false,
		},
		{
			name:         "empty string",
			resourceType: "",
			want:         false,
		},
		{
			name:         "partial prefix",
			resourceType: "pulumi",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsInternalPulumiType(tt.resourceType))
		})
	}
}

func TestIsPulumiProviderResource(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         bool
	}{
		{
			name:         "AWS provider",
			resourceType: "pulumi:providers:aws",
			want:         true,
		},
		{
			name:         "Azure provider",
			resourceType: "pulumi:providers:azure",
			want:         true,
		},
		{
			name:         "GCP provider",
			resourceType: "pulumi:providers:gcp",
			want:         true,
		},
		{
			name:         "pulumi stack is not a provider",
			resourceType: "pulumi:pulumi:Stack",
			want:         false,
		},
		{
			name:         "AWS EC2 instance is not a provider",
			resourceType: "aws:ec2/instance:Instance",
			want:         false,
		},
		{
			name:         "empty string",
			resourceType: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsPulumiProviderResource(tt.resourceType))
		})
	}
}

func TestIsPulumiComponentResource(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         bool
	}{
		{
			name:         "pulumi stack",
			resourceType: "pulumi:pulumi:Stack",
			want:         true,
		},
		{
			name:         "pulumi state",
			resourceType: "pulumi:pulumi:State",
			want:         true,
		},
		{
			name:         "provider is not a component",
			resourceType: "pulumi:providers:aws",
			want:         false,
		},
		{
			name:         "AWS EC2 instance is not a component",
			resourceType: "aws:ec2/instance:Instance",
			want:         false,
		},
		{
			name:         "empty string",
			resourceType: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsPulumiComponentResource(tt.resourceType))
		})
	}
}
