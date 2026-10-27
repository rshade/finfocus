package stackload

import (
	"context"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// Snapshot contains the inventory and availability of one stack load.
type Snapshot struct {
	// StackName identifies the selected stack when known.
	StackName string
	// ProjectDir records the resolved project directory.
	ProjectDir string
	// ManifestTime is the export manifest timestamp.
	ManifestTime string
	// State holds deployed resources used to build overview rows.
	State []engine.StateResource
	// RawState holds deployed custom resources used for descriptor mapping.
	RawState []ingest.StackExportResource
	// ComponentURNs lists deployed non-custom resources. A preview step for one
	// is in the state, though it has no descriptor.
	ComponentURNs []string
	// Plan holds the parsed preview when available.
	Plan *ingest.PulumiPlan
	// Steps holds the preview operations used by the overview engine.
	Steps []engine.PlanStep
	// StateStatus reports whether deployed state is available.
	StateStatus Status
	// PlanStatus reports whether a preview is available.
	PlanStatus Status
	// OmittedUnchanged counts unchanged resources absent from the preview.
	OmittedUnchanged int
	// Mismatches lists resource URNs that disagree between state and preview.
	Mismatches []string
}

// WithPlan returns a new snapshot with a preview attached, preserving the receiver.
func (s *Snapshot) WithPlan(plan *ingest.PulumiPlan) *Snapshot {
	next := *s
	next.Plan = plan
	next.Steps = nil
	next.PlanStatus = Status{Reason: ReasonNotRequested}
	if plan != nil {
		next.Steps = convertPlanSteps(plan.Steps)
		next.PlanStatus = Status{Loaded: true, Reason: ReasonNone}
	}
	next.updateMetadata()
	return &next
}

// Rows returns independent overview rows using the common engine merge.
func (s *Snapshot) Rows(ctx context.Context) ([]engine.OverviewRow, error) {
	state := make([]engine.StateResource, len(s.State))
	copy(state, s.State)
	for i := range state {
		state[i].Properties = deepCopyProperties(state[i].Properties)
		if state[i].CreatedAt != nil {
			created := *state[i].CreatedAt
			state[i].CreatedAt = &created
		}
	}
	if !s.PlanStatus.Loaded {
		return engine.NewRowsFromState(ctx, state), nil
	}
	return engine.MergeResourcesForOverview(ctx, state, s.Steps)
}

// PendingChanges reports the number of changing preview steps.
func (s *Snapshot) PendingChanges(ctx context.Context) (bool, int) {
	return engine.DetectPendingChanges(ctx, s.Steps)
}
