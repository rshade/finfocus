// Package webui implements the localhost-only HTTP layer for `finfocus
// --web`. It is a thin transport: all cost, filter, sort, aggregation, and
// estimation behavior lives in internal/engine, internal/viewmodel, and
// internal/cli and is consumed, never reimplemented, here (FR-011a/b).
package webui
