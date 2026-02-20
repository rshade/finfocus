package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/tui"
)

// TestPhaseConstantsAlignWithTUI ensures the phase index constants stay in sync
// with tui.PhaseNames. If PhaseNames grows or shrinks, this test fails, forcing
// an explicit update to both the constants and the slice.
func TestPhaseConstantsAlignWithTUI(t *testing.T) {
	n := len(tui.PhaseNames)
	require.Greater(t, n, 0, "tui.PhaseNames must not be empty")

	assert.Less(t, phaseEnrichResources, n,
		"phaseEnrichResources (%d) must be a valid index into tui.PhaseNames (len=%d)",
		phaseEnrichResources, n)
	assert.Equal(t, n-1, phaseEnrichResources,
		"phaseEnrichResources should be the last phase index; update the constant if PhaseNames changed")
	assert.Equal(t, 0, phaseLoadStackState,
		"phaseLoadStackState should be the first phase index")
}
