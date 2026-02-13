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
	precedence := getOpPrecedence()
	planByURN := make(map[string]PlanStep, len(planSteps))
	for _, step := range planSteps {
		existing, exists := planByURN[step.URN]
		if !exists || precedence[step.Op] > precedence[existing.Op] {
			planByURN[step.URN] = step
		}
	}

	// Track URNs we have seen from state so we can detect new creates.
	seenURNs := make(map[string]struct{}, len(stateResources))

	rows := make([]OverviewRow, 0, len(stateResources)+len(planSteps))

	// Phase 1: iterate state resources in order (preserves FR-011).
	for _, res := range stateResources {
		if !res.Custom {
			continue
		}
		seenURNs[res.URN] = struct{}{}

		status := StatusActive
		if step, ok := planByURN[res.URN]; ok {
			status = MapOperationToStatus(step.Op)
		}

		rows = append(rows, OverviewRow{
			URN:        res.URN,
			Type:       res.Type,
			ResourceID: res.ID,
			Status:     status,
			Properties: res.Properties,
		})
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
		rows = append(rows, OverviewRow{
			URN:    step.URN,
			Type:   step.Type,
			Status: StatusCreating,
		})
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "merge_resources").
		Int("total_rows", len(rows)).
		Msg("resource merge complete")

	return rows, nil
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
