package stackload

import (
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

func (s *Snapshot) updateMetadata() {
	s.Mismatches = nil
	s.OmittedUnchanged = 0
	if s.Plan == nil {
		return
	}
	listedSame := 0
	stateURNs := make(map[string]bool, len(s.RawState))
	for _, r := range s.RawState {
		stateURNs[r.URN] = true
	}
	for _, urn := range s.ComponentURNs {
		stateURNs[urn] = true
	}
	seen := make(map[string]bool)
	for _, step := range s.Steps {
		if step.Op == "same" {
			listedSame++
		}
		if !s.StateStatus.Loaded || engine.IsPulumiInternalType(step.Type) {
			continue
		}
		mismatch := step.Op == engine.DiffOperationCreate && stateURNs[step.URN] ||
			step.Op != engine.DiffOperationCreate && !stateURNs[step.URN]
		if mismatch && !seen[step.URN] {
			s.Mismatches = append(s.Mismatches, step.URN)
			seen[step.URN] = true
		}
	}
	s.OmittedUnchanged = max(0, s.Plan.ChangeSummary["same"]-listedSame)
}

// sameStack reports whether the state and the preview name the same stack and
// project. A side with no parseable URN gives nothing to compare and matches.
func sameStack(state, plan *Snapshot) bool {
	var stateURNs, planURNs []string
	for _, r := range state.RawState {
		stateURNs = append(stateURNs, r.URN)
	}
	stateURNs = append(stateURNs, state.ComponentURNs...)
	if plan.Plan != nil {
		for _, step := range plan.Plan.Steps {
			planURNs = append(planURNs, step.URN)
		}
	}
	stateStack, stateOK := firstURNStack(stateURNs)
	planStack, planOK := firstURNStack(planURNs)
	return !stateOK || !planOK || stateStack == planStack
}

// urnHeadSegments splits "<stack>::<project>::<rest>" from a URN body.
const urnHeadSegments = 3

// firstURNStack returns "<stack>::<project>" from the first well-formed URN.
func firstURNStack(urns []string) (string, bool) {
	for _, urn := range urns {
		rest, ok := strings.CutPrefix(urn, "urn:pulumi:")
		if !ok {
			continue
		}
		if parts := strings.SplitN(rest, "::", urnHeadSegments); len(parts) == urnHeadSegments {
			return parts[0] + "::" + parts[1], true
		}
	}
	return "", false
}
