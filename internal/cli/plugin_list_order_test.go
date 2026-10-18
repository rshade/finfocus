package cli

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComparePluginVersions_MixedIsTransitive(t *testing.T) {
	t.Parallel()

	versions := []string{"2.0.0", "1.5.invalid", "1.0.0", "bad", "1.2.0"}
	want := []string{"2.0.0", "1.2.0", "1.0.0", "1.5.invalid", "bad"}

	for _, left := range versions {
		for _, right := range versions {
			for _, mid := range versions {
				if comparePluginVersions(left, mid) < 0 && comparePluginVersions(mid, right) < 0 {
					assert.Negative(t, comparePluginVersions(left, right), "%s < %s < %s", left, mid, right)
				}
			}
		}
	}

	got := append([]string(nil), versions...)
	sort.Slice(got, func(i, j int) bool { return comparePluginVersions(got[i], got[j]) < 0 })
	assert.Equal(t, want, got)
}
