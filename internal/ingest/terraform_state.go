package ingest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// TerraformStateVersion is the only Terraform state format version supported.
const TerraformStateVersion = 4

// ErrUnsupportedStateVersion is returned when a state file is not version 4.
var ErrUnsupportedStateVersion = errors.New("unsupported terraform state version")

// ErrEncryptedState is returned when a state file is an encrypted envelope
// (e.g. OpenTofu state encryption). The user must decrypt it first.
var ErrEncryptedState = errors.New("terraform state is encrypted")

// TerraformState represents the top-level structure of a Terraform state v4 file.
type TerraformState struct {
	Version          int                      `json:"version"`
	TerraformVersion string                   `json:"terraform_version"`
	Serial           int                      `json:"serial"`
	Lineage          string                   `json:"lineage"`
	Resources        []TerraformStateResource `json:"resources"`
}

// TerraformStateResource represents a single resource block in a Terraform state file.
type TerraformStateResource struct {
	Module    string                   `json:"module,omitempty"`
	Mode      string                   `json:"mode"`
	Type      string                   `json:"type"`
	Name      string                   `json:"name"`
	Each      string                   `json:"each,omitempty"`
	Provider  string                   `json:"provider"`
	Instances []TerraformStateInstance `json:"instances"`
}

// TerraformStateInstance represents a single instance of a Terraform resource.
// A resource with count/for_each has one instance per key.
type TerraformStateInstance struct {
	IndexKey            interface{}            `json:"index_key,omitempty"`
	Attributes          map[string]interface{} `json:"attributes"`
	SensitiveAttributes []interface{}          `json:"sensitive_attributes,omitempty"`
	Dependencies        []string               `json:"dependencies,omitempty"`
	// Deposed is set on create-before-destroy leftover instances; they must
	// not be priced alongside the replacement instance.
	Deposed string `json:"deposed,omitempty"`
	// Status is "tainted" for instances marked for recreation on next apply.
	Status string `json:"status,omitempty"`
}

// ParseTerraformState parses Terraform state v4 JSON. It returns
// ErrEncryptedState when the input is an encrypted state envelope and
// ErrUnsupportedStateVersion when the version is not 4.
func ParseTerraformState(data []byte) (*TerraformState, error) {
	var probe struct {
		Version           int    `json:"version"`
		EncryptionVersion string `json:"encryption_version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parsing terraform state: %w", err)
	}
	if probe.EncryptionVersion != "" {
		return nil, fmt.Errorf(
			"%w: encryption_version %q present; decrypt the state first (e.g. 'tofu state pull')",
			ErrEncryptedState, probe.EncryptionVersion,
		)
	}
	if probe.Version != TerraformStateVersion {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrUnsupportedStateVersion, probe.Version, TerraformStateVersion)
	}

	var state TerraformState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing terraform state: %w", err)
	}
	return &state, nil
}

// LoadTerraformState reads and parses a Terraform state file from path.
func LoadTerraformState(path string) (*TerraformState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading terraform state file: %w", err)
	}
	return ParseTerraformState(data)
}

// GetManagedResources returns managed resources (mode == "managed"), skipping
// data sources. Instances that are deposed (create-before-destroy leftovers)
// or tainted are dropped so they are not double-counted; resources left with
// no active instances are omitted entirely.
func (s *TerraformState) GetManagedResources() []TerraformStateResource {
	var out []TerraformStateResource
	for _, r := range s.Resources {
		if r.Mode != "managed" {
			continue
		}
		r.Instances = activeInstances(r.Instances)
		if len(r.Instances) == 0 {
			continue
		}
		out = append(out, r)
	}
	return out
}

func activeInstances(instances []TerraformStateInstance) []TerraformStateInstance {
	var out []TerraformStateInstance
	for _, inst := range instances {
		if inst.Deposed != "" || inst.Status == "tainted" {
			continue
		}
		out = append(out, inst)
	}
	return out
}
