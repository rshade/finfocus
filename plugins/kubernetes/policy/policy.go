// Package policy defines the kubernetes allocator's policy document: built-in
// defaults, strict decoding of user overrides, and a canonical digest.
//
// Overrides are applied by decoding onto the defaults, so nested objects merge
// field by field. Any future list-valued field is replaced wholesale, not
// appended to.
package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// CurrentVersion is the only policy schema version this plugin understands.
const CurrentVersion = 1

// Supported values for the policy fields this version defines. Every field
// currently supports exactly one value; Validate rejects anything else.
const (
	valueSeparate         = "separate"
	valueUnitPriceRatio   = "unit-price-ratio"
	valueMaxRequestUsage  = "max-request-usage"
	valueOnDemandWithNote = "on-demand-with-note"

	// defaultCPUCoreHour and defaultMemGiBHour are the built-in node-split
	// unit prices, in dollars per vCPU-hour and per GiB-hour respectively.
	defaultCPUCoreHour = 0.031611
	defaultMemGiBHour  = 0.004237
)

// NodeSplit controls how a node's price is divided between CPU and memory.
type NodeSplit struct {
	Method      string  `json:"method"`
	CPUCoreHour float64 `json:"cpu_core_hour"`
	MemGiBHour  float64 `json:"mem_gib_hour"`
}

// Policy is the effective allocation policy.
type Policy struct {
	Version         int       `json:"version"`
	Idle            string    `json:"idle"`
	SystemWorkloads string    `json:"system_workloads"`
	NodeSplit       NodeSplit `json:"node_split"`
	Charge          string    `json:"charge"`
	ControlPlane    string    `json:"control_plane"`
	SpotNodes       string    `json:"spot_nodes"`
}

// Defaults returns the built-in policy.
func Defaults() Policy {
	return Policy{
		Version:         CurrentVersion,
		Idle:            valueSeparate,
		SystemWorkloads: valueSeparate,
		NodeSplit: NodeSplit{
			Method:      valueUnitPriceRatio,
			CPUCoreHour: defaultCPUCoreHour,
			MemGiBHour:  defaultMemGiBHour,
		},
		Charge:       valueMaxRequestUsage,
		ControlPlane: valueSeparate,
		SpotNodes:    valueOnDemandWithNote,
	}
}

// Decode applies a JSON override document to the defaults. Unknown fields
// (reported with their JSON path), unsupported values, and unknown versions
// are errors. Strict decoding is delegated to pluginsdk.DecodePolicy so every
// allocator applies the same rules.
func Decode(data []byte) (Policy, error) {
	p := Defaults()
	if err := pluginsdk.DecodePolicy(data, &p); err != nil {
		return Policy{}, fmt.Errorf("allocation policy: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Policy{}, fmt.Errorf("allocation policy: %w", err)
	}
	return p, nil
}

// Validate checks that every field holds a value this version supports.
func (p Policy) Validate() error {
	if p.Version != CurrentVersion {
		return fmt.Errorf("unsupported policy version %d (supported: %d)", p.Version, CurrentVersion)
	}
	for _, f := range []struct{ name, got, want string }{
		{"idle", p.Idle, valueSeparate},
		{"system_workloads", p.SystemWorkloads, valueSeparate},
		{"node_split.method", p.NodeSplit.Method, valueUnitPriceRatio},
		{"charge", p.Charge, valueMaxRequestUsage},
		{"control_plane", p.ControlPlane, valueSeparate},
		{"spot_nodes", p.SpotNodes, valueOnDemandWithNote},
	} {
		if f.got != f.want {
			return fmt.Errorf("%s: unsupported value %q (supported: %q)", f.name, f.got, f.want)
		}
	}
	if p.NodeSplit.CPUCoreHour < 0 || p.NodeSplit.MemGiBHour < 0 {
		return errors.New("node_split weights must be >= 0")
	}
	return nil
}

// Canonical returns the policy's canonical JSON and its hex SHA-256 digest.
func (p Policy) Canonical() ([]byte, string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, "", fmt.Errorf("marshal policy: %w", err)
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}
