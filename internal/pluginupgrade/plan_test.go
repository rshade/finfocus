package pluginupgrade_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginupgrade"
)

const latest = "v0.7.0"

func hopTargets(plan *pluginupgrade.Plan) []string {
	targets := make([]string, 0, len(plan.Hops))
	for _, h := range plan.Hops {
		targets = append(targets, h.To)
	}
	return targets
}

func TestNewPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		project      pluginupgrade.Project
		target       string
		wantHops     []string
		wantGo       string
		wantUpToDate bool
		wantWarnings []string
	}{
		{
			name: "oldest supported plugin",
			project: pluginupgrade.Project{
				Version: "v0.5.0", GoVersion: "1.25.5",
			},
			target:   latest,
			wantHops: []string{"v0.5.7", "v0.6.0", "v0.6.1", "v0.6.2", "v0.7.0"},
			wantGo:   "1.27.1",
		},
		{
			name: "stale constant and replace",
			project: pluginupgrade.Project{
				Version: "v0.5.3", GoVersion: "1.25.5",
				ReplacePath:      "../finfocus-spec",
				SpecVersionDecls: []pluginupgrade.SpecVersionDecl{{File: "plugin.go", Value: "v0.5.2"}},
			},
			target:   latest,
			wantHops: []string{"v0.5.7", "v0.6.0", "v0.6.1", "v0.6.2", "v0.7.0"},
			wantGo:   "1.27.1",
			wantWarnings: []string{
				`plugin.go declares SpecVersion "v0.5.2" but go.mod requires v0.5.3`,
				"go.mod replaces github.com/rshade/finfocus-spec with ../finfocus-spec; the replace is left unchanged",
			},
		},
		{
			name: "constant without the v prefix",
			project: pluginupgrade.Project{
				Version: "v0.6.1", GoVersion: "1.27.1",
				SpecVersionDecls: []pluginupgrade.SpecVersionDecl{{File: "calculator.go", Value: "0.6.1"}},
			},
			target:   latest,
			wantHops: []string{"v0.6.2", "v0.7.0"},
			wantWarnings: []string{
				`calculator.go declares SpecVersion "0.6.1" without the v prefix, which GetPluginInfo rejects; ` +
					`it is rewritten as "v0.7.0"`,
			},
		},
		{
			name: "versioned replace stops applying",
			project: pluginupgrade.Project{
				Version: "v0.6.2", GoVersion: "1.27.1",
				ReplacePath: "../finfocus-spec", ReplaceVersion: "v0.6.2",
			},
			target:   latest,
			wantHops: []string{"v0.7.0"},
			wantWarnings: []string{
				"go.mod replaces github.com/rshade/finfocus-spec v0.6.2 with ../finfocus-spec; " +
					"it stops applying once the requirement is v0.7.0, so update or remove it",
			},
		},
		{
			name: "explicit earlier target stops early",
			project: pluginupgrade.Project{
				Version: "v0.5.3", GoVersion: "1.25.5",
			},
			target:   "v0.6.0",
			wantHops: []string{"v0.5.7", "v0.6.0"},
			wantGo:   "1.25.7",
		},
		{
			name: "pseudo-version orders before its release",
			project: pluginupgrade.Project{
				Version: "v0.6.2-0.20260920000000-abcdefabcdef", GoVersion: "1.27.1",
			},
			target:   latest,
			wantHops: []string{"v0.6.2", "v0.7.0"},
		},
		{
			name: "already at target",
			project: pluginupgrade.Project{
				Version: "v0.7.0", GoVersion: "1.27.1",
			},
			target:       "v0.7.0",
			wantUpToDate: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project := tc.project
			plan, err := pluginupgrade.NewPlan(&project, tc.target, latest)
			require.NoError(t, err)

			assert.Equal(t, tc.target, plan.Target)
			assert.Equal(t, project.Version, plan.Current)
			assert.Equal(t, tc.wantUpToDate, plan.UpToDate)
			assert.Equal(t, tc.wantGo, plan.RequiredGo)
			if tc.wantHops == nil {
				assert.Empty(t, plan.Hops)
			} else {
				assert.Equal(t, tc.wantHops, hopTargets(plan))
			}
			assert.Equal(t, tc.wantWarnings, plan.Warnings)
		})
	}
}

func TestNewPlanPatchWithinHop(t *testing.T) {
	t.Parallel()

	project := pluginupgrade.Project{Version: "v0.6.2", GoVersion: "1.27.1"}
	plan, err := pluginupgrade.NewPlan(&project, "v0.6.3", "v0.6.3")
	require.NoError(t, err, "core's own version is a known release")
	assert.False(t, plan.UpToDate, "the require line still has to move")
	assert.Empty(t, plan.Hops)
}

func TestNewPlanErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		target  string
		wantErr error
		wantMsg string
	}{
		{name: "target above core", version: "v0.6.0", target: "v0.8.0",
			wantErr: pluginupgrade.ErrInvalidTarget, wantMsg: "newer than this finfocus build supports (v0.7.0)"},
		{name: "target below plugin", version: "v0.6.0", target: "v0.5.7",
			wantErr: pluginupgrade.ErrInvalidTarget, wantMsg: "older than the plugin's v0.6.0"},
		{name: "unparseable target", version: "v0.6.0", target: "banana",
			wantErr: pluginupgrade.ErrInvalidTarget, wantMsg: `"banana"`},
		{name: "target that is not a known release", version: "v0.6.0", target: "v0.6.5",
			wantErr: pluginupgrade.ErrInvalidTarget, wantMsg: "v0.5.7, v0.6.0, v0.6.1, v0.6.2, v0.7.0"},
		{name: "non-canonical target", version: "v0.6.0", target: "0.6",
			wantErr: pluginupgrade.ErrInvalidTarget, wantMsg: `"0.6"`},
		{name: "plugin newer than core", version: "v0.8.0", target: latest,
			wantErr: pluginupgrade.ErrNewerThanCore, wantMsg: "v0.8.0"},
		{name: "plugin older than supported", version: "v0.4.14", target: latest,
			wantErr: pluginupgrade.ErrTooOld, wantMsg: "plugin init"},
		{name: "unparseable plugin version", version: "latest", target: latest,
			wantErr: pluginupgrade.ErrNotPlugin, wantMsg: `"latest"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project := pluginupgrade.Project{Version: tc.version, GoVersion: "1.27.1"}
			_, err := pluginupgrade.NewPlan(&project, tc.target, latest)
			require.ErrorIs(t, err, tc.wantErr)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}
