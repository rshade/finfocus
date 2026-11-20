package pluginname_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginname"
)

func TestValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "aws", true},
		{"hyphenated", "aws-plugin", true},
		{"with digits", "aws-plugin-v2", true},
		{"too short", "a", false},
		{"leading hyphen", "-aws", false},
		{"trailing hyphen", "aws-", false},
		{"uppercase", "AWS", false},
		{"underscore", "aws_plugin", false},
		{"space", "aws plugin", false},
		{"dot", "aws.plugin", false},
		{"too long", "this-is-a-very-long-plugin-name-that-exceeds-the-maximum-allowed-length", false},
		{"exactly fifty", "a234567890123456789012345678901234567890123456789b", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, pluginname.Valid(tc.input))
		})
	}
}

func TestCanonical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"registry name", "aws-public", ""},
		{"short name", "aws-ce", ""},
		{"letters and digits", "k8s", ""},
		{"binary prefix", "finfocus-plugin-aws-public", "finfocus-plugin-"},
		{"uppercase", "AWS-Public", "lowercase"},
		{"doubled hyphen", "aws--public", "consecutive hyphens"},
		{"leading hyphen", "-aws", "lowercase"},
		{"empty", "", "empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := pluginname.Canonical(tc.input)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestFromBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		region string
		want   string
		wantOK bool
	}{
		{"plain binary", "/plugins/aws-public/v0.2.5/finfocus-plugin-aws-public", "", "aws-public", true},
		{"windows binary", `C:\plugins\finfocus-plugin-aws-public.exe`, "", "aws-public", true},
		{"region binary", "finfocus-plugin-aws-public-us-east-1", "us-east-1", "aws-public", true},
		{"region not a suffix", "finfocus-plugin-aws-public", "us-east-1", "aws-public", true},
		{"no prefix", "/opt/bin/my-plugin", "", "", false},
		{"prefix only", "finfocus-plugin-", "", "", false},
		{"region is the whole name", "finfocus-plugin-us-east-1", "us-east-1", "us-east-1", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := pluginname.FromBinary(tc.path, tc.region)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
