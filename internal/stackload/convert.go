package stackload

import (
	"encoding/json"
	"fmt"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"

	"reflect"
	"sort"
	"strings"
	"time"
)

// convertStateResources converts ingest.StackExportResource to engine.StateResource.
func convertStateResources(resources []ingest.StackExportResource) []engine.StateResource {
	result := make([]engine.StateResource, len(resources))
	for i, r := range resources {
		var createdAt *time.Time
		if r.Created != nil {
			t := *r.Created // copy value to avoid aliasing the ingest layer's pointer
			createdAt = &t
		}
		result[i] = engine.StateResource{
			URN:        r.URN,
			Type:       r.Type,
			ID:         r.ID,
			Custom:     r.Custom,
			Properties: ingest.MergeProperties(r.Outputs, r.Inputs),
			CreatedAt:  createdAt,
			External:   r.External,
		}
	}
	return result
}

const (
	opReplace           = "replace"
	opCreateReplacement = "create-replacement"
)

// convertPlanSteps converts ingest.PulumiStep to engine.PlanStep.
// For update/replace operations, it also extracts property diffs
// by comparing OldState.Inputs to NewState.Inputs.
func convertPlanSteps(steps []ingest.PulumiStep) []engine.PlanStep {
	result := make([]engine.PlanStep, len(steps))
	for i, s := range steps {
		result[i] = engine.PlanStep{
			URN:  s.URN,
			Op:   s.Op,
			Type: s.ResourceType(),
		}
		switch s.Op {
		case engine.DiffOperationCreate, engine.DiffOperationUpdate, opReplace, opCreateReplacement:
			result[i].ProjectedProperties = projectedPropertiesFromStep(s)
		}
		switch s.Op {
		case engine.DiffOperationUpdate, opReplace, opCreateReplacement:
			result[i].PropertyDiffs = diffInputs(s.OldState, s.NewState)
		}
		if s.OldState != nil {
			result[i].OldCloudID = s.OldState.ID
		}
		if s.NewState != nil {
			result[i].NewCloudID = s.NewState.ID
		}
	}
	return result
}

// diffInputs compares OldState.Inputs and NewState.Inputs from a Pulumi step
// and returns a sorted slice of PropertyDiff for keys whose values differ.
func diffInputs(oldState, newState *ingest.PulumiState) []engine.PropertyDiff {
	if oldState == nil || newState == nil {
		return nil
	}
	oldInputs := oldState.Inputs
	newInputs := newState.Inputs
	if len(oldInputs) == 0 && len(newInputs) == 0 {
		return nil
	}

	// Collect all keys from both maps.
	keys := make(map[string]struct{}, len(oldInputs)+len(newInputs))
	for k := range oldInputs {
		keys[k] = struct{}{}
	}
	for k := range newInputs {
		keys[k] = struct{}{}
	}

	var diffs []engine.PropertyDiff
	for k := range keys {
		// Skip internal Pulumi metadata keys (e.g., __defaults).
		if strings.HasPrefix(k, "__") {
			continue
		}
		rawOld, oldOK := oldInputs[k]
		rawNew, newOK := newInputs[k]
		// Presence-aware comparison: a key existing in one map but not
		// the other is always a diff, even if the value is nil.
		if oldOK == newOK && reflect.DeepEqual(rawOld, rawNew) {
			continue
		}
		oldDisplay := formatDiffValue(rawOld)
		newDisplay := formatDiffValue(rawNew)
		diffs = append(diffs, engine.PropertyDiff{
			Key:      k,
			OldValue: oldDisplay,
			NewValue: newDisplay,
		})
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Key < diffs[j].Key
	})
	return diffs
}

// projectedPropertiesFromStep builds pricing properties for projected-cost calls
// by deep-merging old state with new inputs:
//
//	deepMerge(oldStateMerged, newState.inputs)
//
// This preserves unchanged state-derived fields while applying intended preview
// changes, avoiding mispricing from "new inputs only" payloads.
func projectedPropertiesFromStep(step ingest.PulumiStep) map[string]any {
	var oldMerged map[string]any
	if step.OldState != nil {
		oldMerged = ingest.MergeProperties(step.OldState.Outputs, step.OldState.Inputs)
	}

	var newInputs map[string]any
	switch {
	case step.NewState != nil && len(step.NewState.Inputs) > 0:
		newInputs = step.NewState.Inputs
	case len(step.Inputs) > 0:
		newInputs = step.Inputs
	}

	merged := deepMergeProperties(oldMerged, newInputs)
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// deepMergeProperties recursively merges two maps and returns a fresh map.
// Nested map[string]interface{} values are merged; all other override values
// replace the base value.
func deepMergeProperties(base, override map[string]any) map[string]any {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := deepCopyProperties(base)
	if out == nil {
		out = make(map[string]any, len(override))
	}
	for k, v := range override {
		existing, exists := out[k]
		out[k] = deepMergeValue(exists, existing, v)
	}
	return out
}

func deepMergeValue(exists bool, baseValue, overrideValue any) any {
	if !exists {
		return deepCopyAny(overrideValue)
	}
	baseMap, baseIsMap := baseValue.(map[string]any)
	overrideMap, overrideIsMap := overrideValue.(map[string]any)
	if baseIsMap && overrideIsMap {
		return deepMergeProperties(baseMap, overrideMap)
	}
	return deepCopyAny(overrideValue)
}

func deepCopyProperties(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyAny(v)
	}
	return out
}

func deepCopyAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyProperties(t)
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = deepCopyAny(t[i])
		}
		return out
	default:
		return t
	}
}

// formatDiffValue converts a property value to a human-readable string.
// Simple types use [fmt.Sprintf]; complex types (maps, slices) use compact JSON.
func formatDiffValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool, float64, int, int64:
		return fmt.Sprintf("%v", val)
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
}
