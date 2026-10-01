// Package fixtures provides utilities for loading test data files.
package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadJSON loads and unmarshals a JSON fixture file into the provided target.
// The path should be relative to the test/fixtures directory.
func LoadJSON(filename string, target any) error {
	data, err := Load(filename)
	if err != nil {
		return err
	}

	if unmarshalErr := json.Unmarshal(data, target); unmarshalErr != nil {
		return fmt.Errorf("failed to unmarshal JSON from %s: %w", filename, unmarshalErr)
	}

	return nil
}

// LoadYAML loads and unmarshals a YAML fixture file into the provided target.
// The path should be relative to the test/fixtures directory.
func LoadYAML(filename string, target any) error {
	data, err := Load(filename)
	if err != nil {
		return err
	}

	if unmarshalErr := yaml.Unmarshal(data, target); unmarshalErr != nil {
		return fmt.Errorf("failed to unmarshal YAML from %s: %w", filename, unmarshalErr)
	}

	return nil
}

// Load reads a fixture file and returns its contents as bytes.
// The path should be relative to the test/fixtures directory.
// Use LoadJSON or LoadYAML for structured data.
func Load(filename string) ([]byte, error) {
	// Build absolute path to fixture
	fixturesDir := GetFixturesDir()
	fullPath := filepath.Join(fixturesDir, filename)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixture %s: %w", filename, err)
	}

	return data, nil
}

// LoadString loads a fixture file and returns its contents as a string.
// Useful for loading template files, example output, etc.
func LoadString(filename string) (string, error) {
	data, err := Load(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Exists checks if a fixture file exists.
// The path should be relative to the test/fixtures directory.
func Exists(filename string) bool {
	fixturesDir := GetFixturesDir()
	fullPath := filepath.Join(fixturesDir, filename)

	_, err := os.Stat(fullPath)
	return err == nil
}

// GetFixturesDir returns the absolute path to the test/fixtures directory.
// This works regardless of where tests are run from.
func GetFixturesDir() string {
	// Try to find the fixtures directory by walking up from current directory
	cwd, err := os.Getwd()
	if err != nil {
		// Fallback to relative path if we can't get cwd
		return "test/fixtures"
	}

	// Walk up the directory tree looking for test/fixtures
	dir := cwd
	for {
		fixturesPath := filepath.Join(dir, "test", "fixtures")
		if _, statErr := os.Stat(fixturesPath); statErr == nil {
			return fixturesPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding fixtures, fallback to relative path
			return "test/fixtures"
		}
		dir = parent
	}
}

// List returns a list of all files in a fixture subdirectory.
// For example, List("plans") returns all files in test/fixtures/plans/.
func List(subdir string) ([]string, error) {
	fixturesDir := GetFixturesDir()
	fullPath := filepath.Join(fixturesDir, subdir)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list fixtures in %s: %w", subdir, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// LoadPlan loads a Pulumi plan JSON file from test/fixtures/plans/.
// This is a convenience wrapper around LoadJSON.
func LoadPlan(filename string) (map[string]any, error) {
	var plan map[string]any
	if err := LoadJSON(filepath.Join("plans", filename), &plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// LoadSpec loads a pricing spec YAML file from test/fixtures/specs/.
// This is a convenience wrapper around LoadYAML.
func LoadSpec(filename string) (map[string]any, error) {
	var spec map[string]any
	if err := LoadYAML(filepath.Join("specs", filename), &spec); err != nil {
		return nil, err
	}
	return spec, nil
}

// LoadResponse loads a mock response JSON file from test/fixtures/responses/.
// This is a convenience wrapper around LoadJSON.
func LoadResponse(filename string) (map[string]any, error) {
	var response map[string]any
	if err := LoadJSON(filepath.Join("responses", filename), &response); err != nil {
		return nil, err
	}
	return response, nil
}

// MustLoad loads a fixture and panics if it fails.
// Use this in test setup when fixture loading must succeed.
func MustLoad(filename string) []byte {
	data, err := Load(filename)
	if err != nil {
		panic(fmt.Sprintf("failed to load required fixture %s: %v", filename, err))
	}
	return data
}

// MustLoadJSON loads a JSON fixture and panics if it fails.
// Use this in test setup when fixture loading must succeed.
func MustLoadJSON(filename string, target any) {
	if err := LoadJSON(filename, target); err != nil {
		panic(fmt.Sprintf("failed to load required JSON fixture %s: %v", filename, err))
	}
}

// MustLoadYAML loads a YAML fixture and panics if it fails.
// Use this in test setup when fixture loading must succeed.
func MustLoadYAML(filename string, target any) {
	if err := LoadYAML(filename, target); err != nil {
		panic(fmt.Sprintf("failed to load required YAML fixture %s: %v", filename, err))
	}
}

// LoadRecommendations loads a recommendations JSON file from test/fixtures/recommendations/.
// This is a convenience wrapper around LoadJSON for recommendation test fixtures.
func LoadRecommendations(filename string) (map[string]any, error) {
	var recommendations map[string]any
	if err := LoadJSON(filepath.Join("recommendations", filename), &recommendations); err != nil {
		return nil, err
	}
	return recommendations, nil
}
