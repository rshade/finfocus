package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// pluginMetadataFile is the filename for user-supplied plugin metadata.
	pluginMetadataFile = "plugin.metadata.json"
)

// WritePluginMetadata writes the provided metadata map as indented JSON to a
// file named plugin.metadata.json inside dir. The file is written with
// permission mode 0600 and a trailing newline is appended.
//
// dir is the target directory for the metadata file. metadata is the key/value
// map to encode.
//
// It returns an error if the metadata cannot be marshaled to JSON or if the
// file cannot be written.
func WritePluginMetadata(dir string, metadata map[string]string) error {
	data, marshalErr := json.MarshalIndent(metadata, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("marshaling metadata: %w", marshalErr)
	}
	path := filepath.Join(dir, pluginMetadataFile)
	if writeErr := os.WriteFile(path, append(data, '\n'), 0600); writeErr != nil {
		return fmt.Errorf("writing metadata file: %w", writeErr)
	}
	return nil
}

// ErrMetadataNotFound is returned when plugin.metadata.json does not exist.
var ErrMetadataNotFound = errors.New("metadata file not found")

// ReadPluginMetadata reads plugin.metadata.json from the given directory.
// ReadPluginMetadata reads the plugin.metadata.json file located in dir and parses it into a map[string]string.
// dir is the directory containing the metadata file.
// If the metadata file does not exist, ErrMetadataNotFound is returned.
// If the file cannot be read or the JSON cannot be parsed, an error describing the failure is returned.
// On success the parsed metadata map and a nil error are returned.
func ReadPluginMetadata(dir string) (map[string]string, error) {
	path := filepath.Join(dir, pluginMetadataFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrMetadataNotFound
		}
		return nil, fmt.Errorf("reading metadata file: %w", err)
	}

	var metadata map[string]string
	if unmarshalErr := json.Unmarshal(data, &metadata); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing metadata file: %w", unmarshalErr)
	}
	return metadata, nil
}

// ParseRegionFromBinaryName extracts a region string from a binary filename.
// It looks for common AWS region patterns like "us-east-1", "eu-west-1", etc.
// ParseRegionFromBinaryName returns the AWS region found in the base filename of binaryPath.
// It examines the filename (not the full path) for known AWS region substrings and returns
// the matched region and true if one is found.
// The first return is the region string (e.g. "us-west-2"); the second is true when a region
// was detected, or an empty string and false otherwise.
func ParseRegionFromBinaryName(binaryPath string) (string, bool) {
	name := filepath.Base(binaryPath)
	// Look for region patterns in the filename
	// AWS regions: us-east-1, us-west-2, eu-west-1, ap-southeast-1, etc.
	regionParts := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
		"ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ap-south-1", "ap-east-1",
		"sa-east-1",
		"ca-central-1",
		"me-south-1",
		"af-south-1",
	}
	for _, region := range regionParts {
		if strings.Contains(name, region) {
			return region, true
		}
	}
	return "", false
}