package router

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestExtractResourceRegion(t *testing.T) {
	tests := []struct {
		name     string
		resource engine.ResourceDescriptor
		want     string
	}{
		{
			name: "region property",
			resource: engine.ResourceDescriptor{
				Type:       "aws:ec2:Instance",
				Properties: map[string]interface{}{"region": "us-west-2"},
			},
			want: "us-west-2",
		},
		{
			name: "availability zone stripped to region",
			resource: engine.ResourceDescriptor{
				Type:       "aws:ec2:Instance",
				Properties: map[string]interface{}{"availabilityZone": "us-west-2a"},
			},
			want: "us-west-2",
		},
		{
			name: "availability_zone underscore",
			resource: engine.ResourceDescriptor{
				Type:       "aws:ec2:Instance",
				Properties: map[string]interface{}{"availability_zone": "eu-west-1b"},
			},
			want: "eu-west-1",
		},
		{
			name: "location property for Azure",
			resource: engine.ResourceDescriptor{
				Type:       "azure:compute:VM",
				Properties: map[string]interface{}{"location": "eastus"},
			},
			want: "eastus",
		},
		{
			name: "ARN-based extraction",
			resource: engine.ResourceDescriptor{
				Type: "aws:eks:Cluster",
				Properties: map[string]interface{}{
					"arn": "arn:aws:eks:us-west-2:123456789:cluster/my-cluster",
				},
			},
			want: "us-west-2",
		},
		{
			name: "no region info",
			resource: engine.ResourceDescriptor{
				Type:       "aws:ec2:Instance",
				Properties: map[string]interface{}{"instanceType": "t3.micro"},
			},
			want: "",
		},
		{
			name: "nil properties",
			resource: engine.ResourceDescriptor{
				Type: "aws:ec2:Instance",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractResourceRegion(tt.resource)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeToRegion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"us-west-2a", "us-west-2"},
		{"us-west-2", "us-west-2"},
		{"eu-central-1c", "eu-central-1"},
		{"eastus", "eastus"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeToRegion(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractRegionFromARN(t *testing.T) {
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{
			name: "standard ARN",
			arn:  "arn:aws:ec2:us-east-1:123456789:instance/i-1234",
			want: "us-east-1",
		},
		{
			name: "EKS ARN",
			arn:  "arn:aws:eks:us-west-2:123456789:cluster/my-cluster",
			want: "us-west-2",
		},
		{
			name: "global ARN",
			arn:  "arn:aws:iam::123456789:role/my-role",
			want: "",
		},
		{
			name: "not an ARN",
			arn:  "i-1234567890abcdef0",
			want: "",
		},
		{
			name: "empty string",
			arn:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRegionFromARN(tt.arn)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRegionMatches(t *testing.T) {
	tests := []struct {
		name           string
		pluginRegion   string
		resourceRegion string
		want           bool
	}{
		{name: "both match", pluginRegion: "us-west-2", resourceRegion: "us-west-2", want: true},
		{name: "mismatch", pluginRegion: "us-east-1", resourceRegion: "us-west-2", want: false},
		{name: "universal plugin", pluginRegion: "", resourceRegion: "us-west-2", want: true},
		{name: "no resource region", pluginRegion: "us-east-1", resourceRegion: "", want: true},
		{name: "both empty", pluginRegion: "", resourceRegion: "", want: true},
		{name: "case insensitive", pluginRegion: "US-WEST-2", resourceRegion: "us-west-2", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RegionMatches(tt.pluginRegion, tt.resourceRegion)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPluginRegion(t *testing.T) {
	tests := []struct {
		name   string
		client *pluginhost.Client
		want   string
	}{
		{name: "nil client", client: nil, want: ""},
		{name: "nil metadata", client: &pluginhost.Client{}, want: ""},
		{name: "nil metadata map", client: &pluginhost.Client{
			Metadata: &proto.PluginMetadata{},
		}, want: ""},
		{name: "with region", client: &pluginhost.Client{
			Metadata: &proto.PluginMetadata{
				Metadata: map[string]string{"region": "us-west-2"},
			},
		}, want: "us-west-2"},
		{name: "no region key", client: &pluginhost.Client{
			Metadata: &proto.PluginMetadata{
				Metadata: map[string]string{"other": "value"},
			},
		}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PluginRegion(tt.client)
			assert.Equal(t, tt.want, got)
		})
	}
}
