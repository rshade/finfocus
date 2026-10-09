// Package viewmodel provides presentation-neutral view logic shared by the
// TUI and the web UI. It operates on internal/engine types and contains the
// filter, sort, and cluster-display behavior extracted from the bubbletea
// models so that every frontend renders the same outputs of the same
// functions (FR-011a/b).
package viewmodel
