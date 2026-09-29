package engine

import (
	"errors"
	"fmt"
	"time"
)

// Property keys for Pulumi metadata in ResourceDescriptor.Properties.
const (
	// PropertyPulumiCreated is the property key for resource creation timestamp.
	PropertyPulumiCreated = "pulumi:created"
	// PropertyPulumiModified is the property key for resource modification timestamp.
	PropertyPulumiModified = "pulumi:modified"
	// PropertyPulumiExternal indicates the resource was imported (not created by Pulumi).
	PropertyPulumiExternal = "pulumi:external"
)

// Errors for state cost calculation.
var (
	// ErrNoTimestampedResources is returned when no resources have timestamps.
	ErrNoTimestampedResources = errors.New("no resources have created timestamps")
)

// StateCostInput contains input for state-based cost calculation.
type StateCostInput struct {
	Resource   ResourceDescriptor
	HourlyRate float64
	CreatedAt  time.Time
	IsExternal bool
}

// Validate checks that the StateCostInput has valid fields.
func (s *StateCostInput) Validate() error {
	if s.CreatedAt.IsZero() {
		return errors.New("created timestamp is required")
	}
	if s.HourlyRate < 0 {
		return errors.New("hourly rate cannot be negative")
	}
	if s.CreatedAt.After(time.Now()) {
		return errors.New("created timestamp cannot be in the future")
	}
	return nil
}

// StateCostResult contains the result of state-based cost calculation.
type StateCostResult struct {
	TotalCost    float64
	RuntimeHours float64
	Notes        string
}

// CalculateStateCost calculates the estimated cost of a resource based on its runtime.
// The cost is calculated as: hourly_rate × runtime.Hours()
// where runtime = referenceTime - CreatedAt.
//
// For imported resources (IsExternal=true), a warning note is added since the
// Created timestamp reflects import time, not actual creation.
//
// Parameters:
//   - input: StateCostInput with resource details, hourly rate, and creation time
//   - referenceTime: The time to calculate runtime against (usually [time.Now])
//
// Returns StateCostResult with TotalCost, RuntimeHours, and any warning Notes.
func CalculateStateCost(input StateCostInput, referenceTime time.Time) StateCostResult {
	runtime := referenceTime.Sub(input.CreatedAt)
	hours := runtime.Hours()

	// Ensure non-negative runtime
	if hours < 0 {
		hours = 0
	}

	totalCost := input.HourlyRate * hours

	var notes string
	if input.IsExternal {
		notes = "Note: Imported resource - timestamp reflects import time, not actual creation"
	}

	return StateCostResult{
		TotalCost:    totalCost,
		RuntimeHours: hours,
		Notes:        notes,
	}
}

// ExtractCreatedTimestamp extracts the pulumi:created timestamp from resource properties.
// Returns zero time if the property is missing.
// Returns an error if the timestamp is present but cannot be parsed.
func ExtractCreatedTimestamp(resource ResourceDescriptor) (time.Time, error) {
	if resource.Properties == nil {
		return time.Time{}, nil
	}

	createdVal, exists := resource.Properties[PropertyPulumiCreated]
	if !exists {
		return time.Time{}, nil
	}

	// Handle different types
	switch v := createdVal.(type) {
	case string:
		if v == "" {
			return time.Time{}, nil
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing created timestamp: %w", err)
		}
		return t, nil
	case time.Time:
		return v, nil
	default:
		return time.Time{}, fmt.Errorf("unexpected type for created timestamp: %T", createdVal)
	}
}

// FindEarliestCreatedTimestamp finds the earliest Created timestamp from a set of resources.
// This is used to auto-detect the --from date when using --pulumi-state.
// Returns ErrNoTimestampedResources if no resources have timestamps.
func FindEarliestCreatedTimestamp(resources []ResourceDescriptor) (time.Time, error) {
	if len(resources) == 0 {
		return time.Time{}, ErrNoTimestampedResources
	}

	var earliest time.Time
	found := false

	for _, r := range resources {
		ts, err := ExtractCreatedTimestamp(r)
		if err != nil {
			// Skip resources with invalid timestamps
			continue
		}
		if ts.IsZero() {
			// Skip resources without timestamps
			continue
		}

		if !found || ts.Before(earliest) {
			earliest = ts
			found = true
		}
	}

	if !found {
		return time.Time{}, ErrNoTimestampedResources
	}

	return earliest, nil
}

// IsExternalResource checks if a resource was imported (not created by Pulumi).
// Returns true if the pulumi:external property is set to "true" or true.
func IsExternalResource(resource ResourceDescriptor) bool {
	if resource.Properties == nil {
		return false
	}

	externalVal, exists := resource.Properties[PropertyPulumiExternal]
	if !exists {
		return false
	}

	// Handle different types
	switch v := externalVal.(type) {
	case string:
		return v == "true"
	case bool:
		return v
	default:
		return false
	}
}

// UptimeAssumptionNote is the standard note documenting the 100% uptime assumption.
// Per spec T023a, this should be included in all state-based cost estimates.
const UptimeAssumptionNote = "Note: Estimate assumes 100% uptime. Stopped/restarted resources are not tracked."
