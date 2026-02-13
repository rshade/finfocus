package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndReadPluginMetadata(t *testing.T) {
	dir := t.TempDir()

	metadata := map[string]string{
		"region": "us-west-2",
		"custom": "value",
	}

	err := WritePluginMetadata(dir, metadata)
	require.NoError(t, err)

	// Verify file exists
	path := filepath.Join(dir, pluginMetadataFile)
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Read it back
	got, err := ReadPluginMetadata(dir)
	require.NoError(t, err)
	assert.Equal(t, metadata, got)
}

func TestReadPluginMetadata_NotFound(t *testing.T) {
	dir := t.TempDir()

	got, err := ReadPluginMetadata(dir)
	assert.ErrorIs(t, err, ErrMetadataNotFound)
	assert.Nil(t, got)
}

func TestReadPluginMetadata_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, pluginMetadataFile)
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0640))

	_, err := ReadPluginMetadata(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing metadata file")
}

func TestParseRegionFromBinaryName(t *testing.T) {
	tests := []struct {
		name       string
		binaryPath string
		wantRegion string
		wantOk     bool
	}{
		{
			name:       "us-east-1 in name",
			binaryPath: "/path/to/finfocus-plugin-aws-public-us-east-1",
			wantRegion: "us-east-1",
			wantOk:     true,
		},
		{
			name:       "us-west-2 in name",
			binaryPath: "/path/to/finfocus-plugin-aws-public-us-west-2",
			wantRegion: "us-west-2",
			wantOk:     true,
		},
		{
			name:       "eu-west-1 in name",
			binaryPath: "/path/to/plugin-eu-west-1",
			wantRegion: "eu-west-1",
			wantOk:     true,
		},
		{
			name:       "ap-southeast-1 in name",
			binaryPath: "/path/to/plugin-ap-southeast-1",
			wantRegion: "ap-southeast-1",
			wantOk:     true,
		},
		{
			name:       "no region in name",
			binaryPath: "/path/to/finfocus-plugin-aws-public",
			wantRegion: "",
			wantOk:     false,
		},
		{
			name:       "just filename",
			binaryPath: "recorder",
			wantRegion: "",
			wantOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region, ok := ParseRegionFromBinaryName(tt.binaryPath)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantRegion, region)
		})
	}
}

func TestPluginInfo_Region(t *testing.T) {
	tests := []struct {
		name   string
		plugin PluginInfo
		want   string
	}{
		{
			name:   "with region",
			plugin: PluginInfo{Metadata: map[string]string{"region": "us-west-2"}},
			want:   "us-west-2",
		},
		{
			name:   "no region",
			plugin: PluginInfo{Metadata: map[string]string{"other": "value"}},
			want:   "",
		},
		{
			name:   "nil metadata",
			plugin: PluginInfo{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.plugin.Region())
		})
	}
}
