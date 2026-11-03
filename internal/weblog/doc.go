// Package weblog captures, parses, redacts, filters, and buffers log lines for
// the web UI log viewer and the finfocus logs command.
//
// All state is in memory for the life of the process. The package must not
// import cli, webui, registry, or pluginhost; it depends only on config and
// history for the shared redaction rules.
package weblog
