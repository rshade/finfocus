package history

import (
	"errors"
	"fmt"
)

const (
	SourceStateSnapshot = "state_snapshot"
	SourcePlanLineage   = "plan_lineage"
	SourceAnalyzerEvent = "analyzer_event"
)

// Validation limits for ResourceHistoryEntry fields.
const (
	MaxURNLength      = 1024
	MaxCloudIDLength  = 512
	MaxTypeLength     = 256
	MaxProviderLength = 64
	MaxTagCount       = 50
	MaxTagKeyLength   = 128
	MaxTagValueLength = 256
)

// ResourceHistoryEntry represents a single observation of a resource's cloud
// identity at a point in time. Multiple entries can exist for the same URN
// (one per cloud ID incarnation).
type ResourceHistoryEntry struct {
	URN       string            `json:"urn"`
	CloudID   string            `json:"cloud_id"`
	Type      string            `json:"type"`
	Provider  string            `json:"provider"`
	FirstSeen int64             `json:"first_seen"`
	LastSeen  int64             `json:"last_seen"`
	Source    string            `json:"source"`
	Tags      map[string]string `json:"tags,omitempty"`
}

func isValidSource(s string) bool {
	return s == SourceStateSnapshot || s == SourcePlanLineage || s == SourceAnalyzerEvent
}

// Validate enforces all validation rules for a ResourceHistoryEntry.
// It returns descriptive error messages for any violations.
func (e *ResourceHistoryEntry) Validate() error {
	if e == nil {
		return errors.New("entry is nil")
	}

	if e.URN == "" {
		return errors.New("URN is required and cannot be empty")
	}
	if len(e.URN) > MaxURNLength {
		return fmt.Errorf("URN exceeds maximum length of %d characters: got %d", MaxURNLength, len(e.URN))
	}

	if e.CloudID == "" {
		return errors.New("cloudID is required and cannot be empty")
	}
	if len(e.CloudID) > MaxCloudIDLength {
		return fmt.Errorf("cloudID exceeds maximum length of %d characters: got %d", MaxCloudIDLength, len(e.CloudID))
	}

	if e.Type == "" {
		return errors.New("type is required and cannot be empty")
	}
	if len(e.Type) > MaxTypeLength {
		return fmt.Errorf("type exceeds maximum length of %d characters: got %d", MaxTypeLength, len(e.Type))
	}

	if e.Provider == "" {
		return errors.New("provider is required and cannot be empty")
	}
	if len(e.Provider) > MaxProviderLength {
		return fmt.Errorf(
			"provider exceeds maximum length of %d characters: got %d",
			MaxProviderLength, len(e.Provider),
		)
	}

	if !isValidSource(e.Source) {
		return fmt.Errorf("source must be one of %q, %q, or %q, got %q",
			SourceStateSnapshot, SourcePlanLineage, SourceAnalyzerEvent, e.Source)
	}

	if e.FirstSeen <= 0 {
		return fmt.Errorf("firstSeen must be a positive Unix timestamp, got %d", e.FirstSeen)
	}

	if e.LastSeen < e.FirstSeen {
		return fmt.Errorf("lastSeen (%d) must be greater than or equal to firstSeen (%d)",
			e.LastSeen, e.FirstSeen)
	}

	if len(e.Tags) > MaxTagCount {
		return fmt.Errorf("tags exceeds maximum of %d entries: got %d", MaxTagCount, len(e.Tags))
	}

	for key, value := range e.Tags {
		if len(key) > MaxTagKeyLength {
			return fmt.Errorf("tag key exceeds maximum length of %d characters: %q (length %d)",
				MaxTagKeyLength, key, len(key))
		}
		if len(value) > MaxTagValueLength {
			return fmt.Errorf("tag value for key %q exceeds maximum length of %d characters: length %d",
				key, MaxTagValueLength, len(value))
		}
	}

	return nil
}
