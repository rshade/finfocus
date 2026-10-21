// Package esc opens Pulumi ESC environments and returns the environment
// variables a plugin may receive.
//
// Configuration is passed in as plain values. This package imports
// internal/logging and internal/pulumi only. It does not import cli, engine,
// pluginhost, registry, config, or webui.
//
// An ESC value is never logged, placed in an error, or written to disk.
// Values stay in memory only for the lifetime of the [Source] that read them.
package esc
