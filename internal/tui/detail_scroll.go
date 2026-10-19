package tui

import (
	"fmt"
	"strings"
)

const detailScrollHint = "↑↓ scroll"

// visibleDetail returns the lines that fit in height, keeping one row for a
// scroll hint when the text is taller than the terminal. A non-positive height
// returns the full text so callers that have not measured the terminal yet
// stay unchanged. The returned offset is clamped.
func visibleDetail(content string, offset, height int) (string, int) {
	lines := strings.Split(content, "\n")
	window := detailScrollWindow(len(lines), height)
	if window >= len(lines) {
		return content, 0
	}
	offset = clampDetailScroll(offset, len(lines), height)
	end := offset + window
	shown := strings.Join(lines[offset:end], "\n")
	hint := fmt.Sprintf("%s  %d-%d of %d", detailScrollHint, offset+1, end, len(lines))
	return shown + "\n" + hint, offset
}

func detailScrollWindow(lineCount, height int) int {
	if height <= 0 || lineCount <= height {
		return lineCount
	}
	if height == 1 {
		return 1
	}
	return height - 1
}

func clampDetailScroll(offset, lineCount, height int) int {
	window := detailScrollWindow(lineCount, height)
	if window >= lineCount {
		return 0
	}
	maxOffset := lineCount - window
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}
