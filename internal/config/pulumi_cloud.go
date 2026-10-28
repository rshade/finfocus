package config

import (
	"encoding/json"
	"errors"

	"gopkg.in/yaml.v3"
)

const cloudModeFalse = "false"

// PulumiCloudMode controls opt-in historical identity imports.
type PulumiCloudMode string

const (
	// Off is the disabled default, including when the setting is absent.
	Off PulumiCloudMode = ""
	// Auto imports available history and falls back on recoverable failures.
	Auto PulumiCloudMode = "auto"
	// Required fails the command when history cannot be imported completely.
	Required PulumiCloudMode = envBoolTrue
)

// PulumiCloudHistory configures historical Pulumi Cloud identity imports.
type PulumiCloudHistory struct {
	Enabled PulumiCloudMode `json:"enabled" yaml:"enabled"`
}

// String returns the configuration spelling of the mode.
func (m PulumiCloudMode) String() string {
	if m == Off {
		return cloudModeFalse
	}
	return string(m)
}

// Validate rejects modes outside the supported enum.
func (m PulumiCloudMode) Validate() error {
	if m != Off && m != Auto && m != Required {
		return errors.New("pulumi_cloud.enabled must be one of false, auto, true")
	}
	return nil
}

func parseCloudMode(raw string) (PulumiCloudMode, error) {
	switch raw {
	case cloudModeFalse:
		return Off, nil
	case "auto":
		return Auto, nil
	case envBoolTrue:
		return Required, nil
	}
	return Off, errors.New("pulumi_cloud.enabled must be one of false, auto, true")
}

// UnmarshalJSON accepts boolean and string spellings of the supported values.
func (m *PulumiCloudMode) UnmarshalJSON(data []byte) error {
	var raw string
	if string(data) == envBoolTrue || string(data) == cloudModeFalse {
		raw = string(data)
	} else if err := json.Unmarshal(data, &raw); err != nil {
		return errors.New("pulumi_cloud.enabled must be one of false, auto, true")
	}
	value, err := parseCloudMode(raw)
	if err != nil {
		return err
	}
	*m = value
	return nil
}

// UnmarshalYAML accepts boolean and string spellings of the supported values.
func (m *PulumiCloudMode) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag != "!!str" && node.Tag != "!!bool" {
		return errors.New("pulumi_cloud.enabled must be one of false, auto, true")
	}
	value, err := parseCloudMode(node.Value)
	if err != nil {
		return err
	}
	*m = value
	return nil
}

// MarshalJSON writes the mode as a string for editable configuration.
func (m PulumiCloudMode) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

// MarshalYAML writes the mode as a string for editable configuration.
func (m PulumiCloudMode) MarshalYAML() (any, error) { return m.String(), nil }
