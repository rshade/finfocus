package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	if metadata == nil {
		metadata = map[string]string{}
	}
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

// regionPattern matches cloud provider region strings at the end of a filename.
// Covers AWS, Azure, GCP region naming conventions such as us-east-1, eu-west-3,
// ap-southeast-2, etc.
var regionPattern = regexp.MustCompile(`(?:us|eu|ap|sa|ca|me|af|il|mx)-[a-z]+-\d$`)

// ParseRegionFromBinaryName extracts a region string from a binary filename.
// It looks for common cloud region patterns like "us-east-1", "eu-west-1", etc.
// Returns the region and true if found, or empty string and false otherwise.
func ParseRegionFromBinaryName(binaryPath string) (string, bool) {
	name := filepath.Base(binaryPath)
	region := regionPattern.FindString(name)
	if region == "" {
		return "", false
	}
	return region, true
}
