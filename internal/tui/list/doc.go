// Package listview provides virtual scrolling components for Bubble Tea TUI applications.
//
// This package implements efficient rendering for large lists (10,000+ items) by only
// rendering visible rows within the viewport. Key features:
//   - Virtual scrolling with O(viewport_height) render complexity
//   - Keyboard navigation (up/down, pgup/pgdn, home/end)
//   - Integration with Bubble Tea's viewport and lipgloss styling
//   - Smooth scrolling with <100ms latency target
//
// Virtual scrolling enables responsive TUI experiences even with massive datasets,
// ensuring the application starts immediately without pre-rendering all rows.
package listview
