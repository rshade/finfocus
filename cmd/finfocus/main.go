// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

// Package main provides the finfocus CLI tool for calculating cloud infrastructure costs.
// It supports both projected costs from Pulumi infrastructure definitions and actual historical
// costs from cloud provider APIs via a plugin-based architecture.
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/pkg/version"
)

// run executes the main application logic for the finfocus program and returns
// the process exit code. It either starts the Pulumi analyzer plugin serve path
// when the executable name indicates an analyzer invocation (supports both
// legacy policy-pack and direct analyzer names), or it runs the regular CLI
// root command through ax.Execute for normal operation.
//
// The analyzer-serve path is intentionally kept outside ax.Execute: Pulumi's
// plugin handshake requires stdout to contain only a port number, which is
// incompatible with ax.Execute's flag-mounting and mode-resolution behavior.
func run() int {
	// Check if the binary is being run as a Pulumi Analyzer plugin
	// Supports both legacy policy pack mode (pulumi-analyzer-policy-finfocus)
	// and direct analyzer mode (pulumi-analyzer-finfocus)
	exeName := filepath.Base(os.Args[0])
	if strings.Contains(exeName, "pulumi-analyzer-policy-finfocus") ||
		strings.Contains(exeName, "pulumi-analyzer-finfocus") {
		// If run as an analyzer plugin, execute the analyzer serve logic directly.
		// Pulumi expects the plugin binary to start a gRPC server and output the port.
		// RunAnalyzerServe sets up its own stderr logger via getAnalyzerLogLevel(),
		// so we only need to provide a basic context here.
		dummyCmd := &cobra.Command{}
		dummyCmd.SetContext(context.Background())
		if err := cli.RunAnalyzerServe(dummyCmd); err != nil {
			return 1
		}
		return 0
	}

	// Regular CLI execution, routed through ax.Execute for deterministic exit
	// codes, ax.Error envelopes on stderr, and the agentic safety primitives
	// (--format/--dry-run/--yes/--idempotency-key, __schema discoverability).
	root := cli.NewRootCmd(version.GetVersion())
	return ax.Execute(context.Background(), root, ax.WithVersion(version.GetVersion()))
}

func main() {
	os.Exit(run())
}
