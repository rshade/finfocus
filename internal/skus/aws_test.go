package skus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAWSSKU(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		expected     string
	}{
		{
			name:         "EKS cluster",
			resourceType: "aws:eks/cluster:Cluster",
			expected:     "cluster",
		},
		{
			name:         "EKS addon",
			resourceType: "aws:eks/addon:Addon",
			expected:     "addon",
		},
		{
			name:         "EC2 instance (not in well-known map)",
			resourceType: "aws:ec2/instance:Instance",
			expected:     "",
		},
		{
			name:         "S3 bucket (not in well-known map)",
			resourceType: "aws:s3/bucket:Bucket",
			expected:     "",
		},
		{
			name:         "empty type",
			resourceType: "",
			expected:     "",
		},
		{
			name:         "malformed type (no colons)",
			resourceType: "eks-cluster",
			expected:     "",
		},
		{
			name:         "partial type (one colon)",
			resourceType: "aws:eks/cluster",
			expected:     "cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveAWSSKU(tt.resourceType, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPulumiSegment(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		expected     string
	}{
		{
			name:         "standard three-part type",
			resourceType: "aws:eks/cluster:Cluster",
			expected:     "eks/cluster",
		},
		{
			name:         "two-part type",
			resourceType: "aws:ec2/instance",
			expected:     "ec2/instance",
		},
		{
			name:         "no colon",
			resourceType: "nocolon",
			expected:     "",
		},
		{
			name:         "empty string",
			resourceType: "",
			expected:     "",
		},
		{
			name:         "uppercase segment is lowercased",
			resourceType: "aws:EKS/Cluster:Cluster",
			expected:     "eks/cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPulumiSegment(tt.resourceType)
			assert.Equal(t, tt.expected, result)
		})
	}
}
