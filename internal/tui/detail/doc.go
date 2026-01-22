// Package detail provides lazy loading and error recovery for TUI detail views.
//
// This package implements on-demand data loading for resource-intensive operations
// (e.g., fetching historical cost data) to keep the TUI responsive. Key features:
//   - Lazy loading: data fetched only when detail view is activated
//   - Async loading with immediate loading state display
//   - Inline error recovery with keyboard-navigable retry ('r' key)
//   - Non-blocking error states that don't crash the application
//
// Lazy loading ensures fast navigation through lists without waiting for secondary
// data to load, meeting the <500ms detail view load target or immediate loading state.
package detail
