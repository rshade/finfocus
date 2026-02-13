package awsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegionFromARN(t *testing.T) {
	tests := []struct {
		name       string
		arn        string
		wantRegion string
	}{
		{name: "valid ARN", arn: "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0", wantRegion: "us-east-1"},
		{name: "valid ARN eu-west-1", arn: "arn:aws:s3:eu-west-1:123456789012:bucket/my-bucket", wantRegion: "eu-west-1"},
		{name: "global service empty region", arn: "arn:aws:iam::123456789012:user/admin", wantRegion: ""},
		{name: "too few segments", arn: "arn:aws:ec2", wantRegion: ""},
		{name: "empty string", arn: "", wantRegion: ""},
		{name: "not an ARN", arn: "not-an-arn", wantRegion: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantRegion, RegionFromARN(tt.arn))
		})
	}
}
