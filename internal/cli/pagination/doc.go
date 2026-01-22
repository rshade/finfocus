// Package pagination provides utilities for CLI pagination, sorting, and result formatting.
//
// This package contains shared pagination logic used across CLI commands, including:
//   - PaginationParams: CLI flag parsing and validation
//   - PaginationMeta: Response metadata for paginated results
//   - Sorter: Generic sorting interface with field validation
//
// The pagination package ensures consistent pagination behavior across all finfocus commands
// that return lists of items (recommendations, cost results, etc.).
package pagination
