package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildResourceFromParamsNormalizesProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		want     string
	}{
		{"aws", "aws"},
		{"AWS", "aws"},
		{"aws-native", "aws"},
		{"azure-native", "azure"},
		{"google-native", "gcp"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Parallel()
			resource := buildResourceFromParams(tt.provider, "aws-native:ec2:Instance", "us-east-1")
			assert.Equal(t, tt.want, resource.Provider)
			assert.Equal(t, "aws-native:ec2:Instance", resource.Type)
		})
	}
}
