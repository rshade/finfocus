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
	"slices"
	"strings"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// CurrentVersion is the only policy schema version this plugin understands.
const CurrentVersion = 1

// Supported values for the policy fields this version defines. Validate
// rejects anything outside the set named for that field.
const (
	valueSeparate         = "separate"
	valueShare            = "share"
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
	for _, f := range []struct {
		name string
		got  string
		ok   []string
	}{
		{"idle", p.Idle, []string{valueSeparate, valueShare}},
		{"system_workloads", p.SystemWorkloads, []string{valueSeparate, valueShare}},
		{"node_split.method", p.NodeSplit.Method, []string{valueUnitPriceRatio}},
		{"charge", p.Charge, []string{valueMaxRequestUsage}},
		{"control_plane", p.ControlPlane, []string{valueSeparate}},
		{"spot_nodes", p.SpotNodes, []string{valueOnDemandWithNote}},
	} {
		if err := supportedValue(f.name, f.got, f.ok); err != nil {
			return err
		}
	}
	if p.NodeSplit.CPUCoreHour < 0 || p.NodeSplit.MemGiBHour < 0 {
		return errors.New("node_split weights must be >= 0")
	}
	return nil
}

// ShareIdle reports whether idle node capacity is folded into that node's workloads.
func (p Policy) ShareIdle() bool { return p.Idle == valueShare }

// ShareSystemWorkloads reports whether kube-system and DaemonSet cost is folded
// into the other workloads on the same node.
func (p Policy) ShareSystemWorkloads() bool { return p.SystemWorkloads == valueShare }

func supportedValue(name, got string, ok []string) error {
	if slices.Contains(ok, got) {
		return nil
	}
	quoted := make([]string, len(ok))
	for i, want := range ok {
		quoted[i] = fmt.Sprintf("%q", want)
	}
	return fmt.Errorf("%s: unsupported value %q (supported: %s)", name, got, strings.Join(quoted, ", "))
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
