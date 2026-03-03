package engine

import (
	"context"

	"github.com/rshade/finfocus/internal/logging"
)

// Pulumi plan operation constants.
const (
	opCreate            = "create"
	opUpdate            = "update"
	opDelete            = "delete"
	opReplace           = "replace"
	opCreateReplacement = "create-replacement"
	opDeleteReplaced    = "delete-replaced"
)

// getOpPrecedence returns a map defining which operation takes priority when
// multiple plan steps reference the same URN (e.g., create-replacement +
// delete-replaced). Higher values win.
func getOpPrecedence() map[string]int {
	return map[string]int{
		opCreate:            0,
		opUpdate:            1,
		opCreateReplacement: 2, //nolint:mnd // Precedence ordering
		opReplace:           3, //nolint:mnd // Precedence ordering
		opDeleteReplaced:    4, //nolint:mnd // Precedence ordering
		opDelete:            5, //nolint:mnd // Precedence ordering
	}
}

// MapOperationToStatus converts a Pulumi plan operation string to a
// ResourceStatus value. Unknown operations default to StatusActive.
func MapOperationToStatus(op string) ResourceStatus {
	switch op {
	case opCreate:
		return StatusCreating
	case opUpdate:
		return StatusUpdating
	case opDelete:
		return StatusDeleting
	case opReplace, opCreateReplacement, opDeleteReplaced:
		return StatusReplacing
	default:
		return StatusActive
	}
}

// stateResourceToRow converts a StateResource to a skeleton OverviewRow with
// StatusActive. Only call this for custom resources (res.Custom == true).
func stateResourceToRow(res StateResource) OverviewRow {
	return OverviewRow{
		URN:        res.URN,
		Type:       res.Type,
		ResourceID: res.ID,
		Status:     StatusActive,
		Properties: res.Properties,
		CreatedAt:  res.CreatedAt,
	}
}

// buildPlanByURN indexes plan steps by URN, keeping only the highest-precedence
// operation when the same URN appears multiple times (e.g., create-replacement +
// delete-replaced during a replace).
func buildPlanByURN(planSteps []PlanStep) map[string]PlanStep {
	precedence := getOpPrecedence()
	planByURN := make(map[string]PlanStep, len(planSteps))
	for _, step := range planSteps {
		existing, exists := planByURN[step.URN]
		if !exists || precedence[step.Op] > precedence[existing.Op] {
			planByURN[step.URN] = step
		}
	}
	return planByURN
}

// MergeResourcesForOverview builds skeleton OverviewRow entries by combining
// current Pulumi state resources with pending plan steps.
//
// The merge preserves the order of state resources (FR-011), appending any
// newly-created resources from the plan that are not already present in state.
// Only custom resources (cloud resources, not providers or components) are
// included.
//
// The returned rows have URN, Type, ResourceID, and Status populated; cost
// fields are left nil for later enrichment.
func MergeResourcesForOverview(
	ctx context.Context,
	stateResources []StateResource,
	planSteps []PlanStep,
) ([]OverviewRow, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "merge_resources").
		Int("state_resources", len(stateResources)).
		Int("plan_steps", len(planSteps)).
		Msg("starting resource merge for overview")

	// Index plan steps by URN for O(1) lookup, using deterministic precedence.
	planByURN := buildPlanByURN(planSteps)
	// Build diffs separately — status precedence (delete-replaced > create-replacement)
	// differs from diff precedence (create-replacement carries the diffs).
	diffsByURN := BuildPropertyDiffsByURN(planSteps)
	projectedPropsByURN := BuildProjectedPropertiesByURN(planSteps)

	// Track URNs we have seen from state so we can detect new creates.
	seenURNs := make(map[string]struct{}, len(stateResources))

	rows := make([]OverviewRow, 0, len(stateResources)+len(planSteps))

	// Phase 1: iterate state resources in order (preserves FR-011).
	for _, res := range stateResources {
		if !res.Custom {
			continue
		}
		seenURNs[res.URN] = struct{}{}

		row := stateResourceToRow(res)
		if step, ok := planByURN[res.URN]; ok {
			row.Status = MapOperationToStatus(step.Op)
		}
		if diffs, ok := diffsByURN[res.URN]; ok {
			row.PropertyDiffs = append([]PropertyDiff(nil), diffs...)
		}
		if props, ok := projectedPropsByURN[res.URN]; ok {
			row.ProjectedProperties = cloneProperties(props)
		}

		rows = append(rows, row)
	}

	// Phase 2: append new resources that appear only in the plan.
	for _, step := range planSteps {
		if _, seen := seenURNs[step.URN]; seen {
			continue
		}
		if step.Op != opCreate {
			continue
		}
		seenURNs[step.URN] = struct{}{}
		row := OverviewRow{
			URN:    step.URN,
			Type:   step.Type,
			Status: StatusCreating,
		}
		if props, ok := projectedPropsByURN[step.URN]; ok {
			row.ProjectedProperties = cloneProperties(props)
			// Create rows are plan-only, so use projected properties as the base
			// resource properties for enrichment and recommendation requests.
			row.Properties = cloneProperties(props)
		}
		rows = append(rows, row)
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "merge_resources").
		Int("total_rows", len(rows)).
		Msg("resource merge complete")

	return rows, nil
}

// NewRowsFromState creates skeleton OverviewRows from state resources only,
// with no plan data applied. All rows are assigned StatusActive.
//
// This is used for Phase 1 (state-first) loading before pulumi preview runs.
// The rows are structurally identical to those produced by MergeResourcesForOverview
// and can be enriched and later updated with change status via ApplyChangesToRows.
//
// Only custom resources are included — the same filter as MergeResourcesForOverview.
func NewRowsFromState(ctx context.Context, stateResources []StateResource) []OverviewRow {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "new_rows_from_state").
		Int("state_resources", len(stateResources)).
		Msg("creating skeleton rows from state only")

	rows := make([]OverviewRow, 0, len(stateResources))
	for _, res := range stateResources {
		if !res.Custom {
			continue
		}
		rows = append(rows, stateResourceToRow(res))
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "new_rows_from_state").
		Int("total_rows", len(rows)).
		Msg("skeleton rows from state created")

	return rows
}

// ApplyChangesToRows updates the Status field of existing OverviewRows in-place
// using a map of URN → ResourceStatus derived from plan steps.
// Rows whose URN is not in statusByURN retain their current Status.
//
// This is used for Phase 2 when preview completes after initial TUI display.
//
// Thread safety: The Bubble Tea Update() method is single-threaded by design,
// so this function is safe to call from OverviewChangesReadyMsg handling in the
// TUI. Callers in other contexts must ensure no concurrent reads on rows during
// this call.
//
// No-op if rows is nil.
func ApplyChangesToRows(rows []OverviewRow, statusByURN map[string]ResourceStatus) {
	if rows == nil {
		return
	}
	for i := range rows {
		if status, ok := statusByURN[rows[i].URN]; ok {
			rows[i].Status = status
		}
	}
}

// ApplyPropertyDiffsToRows updates the PropertyDiffs field of existing OverviewRows
// in-place using a map of URN → []PropertyDiff derived from plan steps.
// Rows whose URN is not in diffsByURN retain their current PropertyDiffs.
//
// This is used alongside ApplyChangesToRows for Phase 2 when preview completes
// after initial TUI display.
//
// No-op if rows is nil.
func ApplyPropertyDiffsToRows(rows []OverviewRow, diffsByURN map[string][]PropertyDiff) {
	if rows == nil {
		return
	}
	for i := range rows {
		if diffs, ok := diffsByURN[rows[i].URN]; ok {
			rows[i].PropertyDiffs = append([]PropertyDiff(nil), diffs...)
		}
	}
}

// ApplyProjectedPropertiesToRows updates the ProjectedProperties field of
// existing OverviewRows in-place using a map of URN → projected properties
// derived from plan steps. Rows whose URN is not in projectedPropsByURN retain
// their current ProjectedProperties.
//
// No-op if rows is nil.
func ApplyProjectedPropertiesToRows(rows []OverviewRow, projectedPropsByURN map[string]map[string]interface{}) {
	if rows == nil {
		return
	}
	for i := range rows {
		if props, ok := projectedPropsByURN[rows[i].URN]; ok {
			rows[i].ProjectedProperties = cloneProperties(props)
		}
	}
}

// BuildPropertyDiffsByURN converts a slice of PlanSteps to a map of URN → []PropertyDiff.
// Unlike BuildStatusByURN, this uses diff-specific precedence: for a replace flow
// (create-replacement + delete-replaced), the create-replacement step carries the
// PropertyDiffs while delete-replaced has none. Using the status precedence
// (which favors delete-replaced) would silently drop diffs. Instead, we keep
// the first non-empty PropertyDiffs found for each URN.
func BuildPropertyDiffsByURN(planSteps []PlanStep) map[string][]PropertyDiff {
	diffsByURN := make(map[string][]PropertyDiff)
	for _, step := range planSteps {
		if len(step.PropertyDiffs) > 0 {
			if _, exists := diffsByURN[step.URN]; !exists {
				// Copy the slice to avoid aliasing the original backing array.
				copied := make([]PropertyDiff, len(step.PropertyDiffs))
				copy(copied, step.PropertyDiffs)
				diffsByURN[step.URN] = copied
			}
		}
	}
	return diffsByURN
}

// BuildProjectedPropertiesByURN converts a slice of PlanSteps to a map of URN
// → projected properties. For replace flows where multiple operations may
// appear for the same URN, the first non-empty projected properties are kept.
func BuildProjectedPropertiesByURN(planSteps []PlanStep) map[string]map[string]interface{} {
	propsByURN := make(map[string]map[string]interface{})
	for _, step := range planSteps {
		if len(step.ProjectedProperties) == 0 {
			continue
		}
		if _, exists := propsByURN[step.URN]; exists {
			continue
		}
		propsByURN[step.URN] = cloneProperties(step.ProjectedProperties)
	}
	return propsByURN
}

func cloneProperties(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return cloneProperties(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i := range t {
			out[i] = cloneAny(t[i])
		}
		return out
	default:
		return t
	}
}

// BuildStatusByURN converts a slice of PlanSteps to a map of URN → ResourceStatus
// using the same precedence rules as MergeResourcesForOverview. When the same URN
// appears with multiple operations (e.g., create-replacement + delete-replaced for
// a replace), the highest-precedence operation wins.
func BuildStatusByURN(planSteps []PlanStep) map[string]ResourceStatus {
	planByURN := buildPlanByURN(planSteps)
	statusByURN := make(map[string]ResourceStatus, len(planByURN))
	for urn, step := range planByURN {
		statusByURN[urn] = MapOperationToStatus(step.Op)
	}
	return statusByURN
}

// DetectPendingChanges inspects a set of plan steps and reports whether any
// mutating operations are pending, along with the count.
func DetectPendingChanges(ctx context.Context, planSteps []PlanStep) (bool, int) {
	log := logging.FromContext(ctx)

	changeCount := 0
	for _, step := range planSteps {
		switch step.Op {
		case opCreate, opUpdate, opDelete, opReplace, opCreateReplacement, opDeleteReplaced:
			changeCount++
		}
	}

	hasChanges := changeCount > 0

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "detect_pending_changes").
		Bool("has_changes", hasChanges).
		Int("change_count", changeCount).
		Msg("pending change detection complete")

	return hasChanges, changeCount
}
