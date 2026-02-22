package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/tui"
)

// TestResolveIsStateOnly verifies that detectErr does not override --yes, and that
// without --yes a detection error correctly forces state-only mode.
func TestResolveIsStateOnly(t *testing.T) {
	detectErr := errors.New("change detection failed")
	noErr := error(nil)

	tests := []struct {
		name        string
		params      overviewParams
		signal      pulumidetect.ChangeSignal
		detectErr   error
		wantIsState bool
	}{
		{
			name:        "yes=true, no error: preview runs",
			params:      overviewParams{yes: true},
			signal:      pulumidetect.ChangeSignal{},
			detectErr:   noErr,
			wantIsState: false,
		},
		{
			name:        "yes=true, detect error: preview must still run (bug fix)",
			params:      overviewParams{yes: true},
			signal:      pulumidetect.ChangeSignal{},
			detectErr:   detectErr,
			wantIsState: false,
		},
		{
			name:        "yes=false, no error, no changes: state-only",
			params:      overviewParams{yes: false},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: false},
			detectErr:   noErr,
			wantIsState: true,
		},
		{
			name:        "yes=false, no error, likely changes: preview runs",
			params:      overviewParams{yes: false},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: true},
			detectErr:   noErr,
			wantIsState: false,
		},
		{
			name:        "yes=false, detect error: falls back to state-only",
			params:      overviewParams{yes: false},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: true},
			detectErr:   detectErr,
			wantIsState: true,
		},
		{
			name:        "explicit state-only (pulumi-state set, no pulumi-json, no yes): state-only",
			params:      overviewParams{yes: false, pulumiState: "state.json", pulumiJSON: ""},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: true},
			detectErr:   noErr,
			wantIsState: true,
		},
		{
			name:        "explicit state-only overridden by yes: preview runs",
			params:      overviewParams{yes: true, pulumiState: "state.json", pulumiJSON: ""},
			signal:      pulumidetect.ChangeSignal{},
			detectErr:   noErr,
			wantIsState: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveIsStateOnly(tt.params, tt.signal, tt.detectErr)
			assert.Equal(t, tt.wantIsState, got)
		})
	}
}

// TestPhaseConstantsAlignWithTUI ensures the phase index constants stay in sync
// with tui.PhaseNames. If PhaseNames grows or shrinks, this test fails, forcing
// an explicit update to both the constants and the slice.
func TestPhaseConstantsAlignWithTUI(t *testing.T) {
	n := len(tui.GetPhaseNames())
	require.Greater(t, n, 0, "tui.GetPhaseNames() must not be empty")

	assert.Less(t, phaseEnrichResources, n,
		"phaseEnrichResources (%d) must be a valid index into tui.PhaseNames (len=%d)",
		phaseEnrichResources, n)
	assert.Equal(t, n-1, phaseEnrichResources,
		"phaseEnrichResources should be the last phase index; update the constant if PhaseNames changed")
	assert.Equal(t, 0, phaseLoadStackState,
		"phaseLoadStackState should be the first phase index")
}
