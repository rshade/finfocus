package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisibleDetail_ScrollsToHiddenLines(t *testing.T) {
	t.Parallel()

	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "line " + string(rune('a'+i))
	}
	lines[9] = "line tail"
	content := strings.Join(lines, "\n")

	full, offset := visibleDetail(content, 0, 0)
	assert.Equal(t, content, full)
	assert.Equal(t, 0, offset)

	top, offset := visibleDetail(content, 0, 4)
	assert.Equal(t, 0, offset)
	assert.Contains(t, top, "line a")
	assert.NotContains(t, top, "line tail")
	assert.Contains(t, top, detailScrollHint)

	bottom, offset := visibleDetail(content, 100, 4)
	assert.Equal(t, 7, offset)
	assert.Contains(t, bottom, "line tail")
	assert.NotContains(t, bottom, "line a")
}

func TestVisibleDetail_ShortContent(t *testing.T) {
	t.Parallel()

	got, offset := visibleDetail("one\ntwo", 3, 10)
	assert.Equal(t, "one\ntwo", got)
	assert.Equal(t, 0, offset)
	require.Equal(t, 0, clampDetailScroll(4, 2, 10))
}
