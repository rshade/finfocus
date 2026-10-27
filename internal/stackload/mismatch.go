package stackload

import "github.com/rshade/finfocus/internal/engine"

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
