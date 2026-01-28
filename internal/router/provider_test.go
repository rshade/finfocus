package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractProviderFromType(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		// Standard Pulumi resource types
		{
			name:         "AWS EC2 instance",
			resourceType: "aws:ec2/instance:Instance",
			want:         "aws",
		},
		{
			name:         "GCP compute instance",
			resourceType: "gcp:compute:Instance",
			want:         "gcp",
		},
		{
			name:         "Azure VM",
			resourceType: "azure:compute/vm:VM",
			want:         "azure",
		},
		{
			name:         "Kubernetes pod",
			resourceType: "kubernetes:core/v1:Pod",
			want:         "kubernetes",
		},
		// Native providers
		{
			name:         "AWS native provider",
			resourceType: "aws-native:ec2:Instance",
			want:         "aws-native",
		},
		// Pulumi provider reference
		{
			name:         "Pulumi provider reference",
			resourceType: "pulumi:providers:aws",
			want:         "pulumi",
		},
		// Edge cases
		{
			name:         "empty string",
			resourceType: "",
			want:         ProviderUnknown,
		},
		{
			name:         "no colon separator",
			resourceType: "aws-ec2-instance",
			want:         "aws-ec2-instance",
		},
		{
			name:         "single colon",
			resourceType: "aws:ec2",
			want:         "aws",
		},
		{
			name:         "starts with colon",
			resourceType: ":ec2:Instance",
			want:         ProviderUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractProviderFromType(tt.resourceType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsGlobalProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     bool
	}{
		{"empty string is global", "", true},
		{"wildcard is global", "*", true},
		{"aws is not global", "aws", false},
		{"gcp is not global", "gcp", false},
		{"unknown is not global", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGlobalProvider(tt.provider)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{"lowercase stays lowercase", "aws", "aws"},
		{"uppercase becomes lowercase", "AWS", "aws"},
		{"mixed case normalized", "GcP", "gcp"},
		{"whitespace trimmed", "  azure  ", "azure"},
		{"empty stays empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeProvider(tt.provider)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProviderMatches(t *testing.T) {
	tests := []struct {
		name              string
		resourceProvider  string
		supportedProvider string
		want              bool
	}{
		// Global provider matches everything
		{"empty supported matches aws", "aws", "", true},
		{"wildcard matches aws", "aws", "*", true},
		{"empty supported matches gcp", "gcp", "", true},
		{"wildcard matches gcp", "gcp", "*", true},

		// Exact matches (case-insensitive)
		{"aws matches aws", "aws", "aws", true},
		{"AWS matches aws", "AWS", "aws", true},
		{"aws matches AWS", "aws", "AWS", true},

		// Non-matches
		{"aws does not match gcp", "aws", "gcp", false},
		{"gcp does not match azure", "gcp", "azure", false},

		// Edge cases
		{"unknown matches unknown", "unknown", "unknown", true},
		{"empty resource matches empty supported", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProviderMatches(tt.resourceProvider, tt.supportedProvider)
			assert.Equal(t, tt.want, got)
		})
	}
}
