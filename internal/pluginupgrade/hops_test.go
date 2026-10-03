package pluginupgrade_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/pluginupgrade"
)

// guidesDir is the agent skill's reference directory, relative to this package.
const guidesDir = "../../agent-skills/finfocus-plugin-upgrade/references"

// TestHopsMatchGuides keeps the hop table and the skill's migration guides in
// step: each hop has exactly one guide and each guide belongs to a hop.
func TestHopsMatchGuides(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(filepath.FromSlash(guidesDir))
	require.NoError(t, err)

	var guides []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "to-") {
			guides = append(guides, e.Name())
		}
	}

	var hopGuides []string
	for _, h := range pluginupgrade.Hops() {
		assert.Equal(t, "to-"+h.To+".md", h.Guide, "guide name for hop %s", h.To)
		assert.NotEmpty(t, h.Summary, "summary for hop %s", h.To)
		hopGuides = append(hopGuides, h.Guide)
	}

	sort.Strings(guides)
	sort.Strings(hopGuides)
	assert.Equal(t, hopGuides, guides)
}

// TestHopsReachCoreSpecVersion fails when core moves to a newer finfocus-spec
// without a hop (and guide) for it, even when the release is additive.
func TestHopsReachCoreSpecVersion(t *testing.T) {
	t.Parallel()

	hops := pluginupgrade.Hops()
	require.NotEmpty(t, hops)

	for i := 1; i < len(hops); i++ {
		prev := semver.MustParse(hops[i-1].To)
		cur := semver.MustParse(hops[i].To)
		assert.True(t, prev.LessThan(cur), "hops must be strictly ascending: %s then %s", hops[i-1].To, hops[i].To)
	}

	newest := semver.MustParse(hops[len(hops)-1].To)
	core := semver.MustParse(pluginsdk.SpecVersion)
	assert.False(t, newest.LessThan(core),
		"core builds against finfocus-spec %s but the newest hop is %s: add a hop and a guide in %s",
		pluginsdk.SpecVersion, newest.Original(), guidesDir)
}

func TestHopsReturnsCopy(t *testing.T) {
	t.Parallel()

	hops := pluginupgrade.Hops()
	hops[0].Manual = append(hops[0].Manual[:0], "mutated")
	hops[0].To = "v9.9.9"

	fresh := pluginupgrade.Hops()
	assert.NotEqual(t, "v9.9.9", fresh[0].To)
	assert.NotContains(t, fresh[0].Manual, "mutated")
}
