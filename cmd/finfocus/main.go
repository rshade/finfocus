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
// the process exit code. The executable name selects one of three modes:
// pulumi-language-finfocus serves Pulumi's language host,
// pulumi-analyzer-policy-finfocus and pulumi-analyzer-finfocus serve the
// analyzer, and any other name runs the CLI through ax.Execute.
//
// The language-host and analyzer-serve paths stay outside ax.Execute: Pulumi's
// plugin handshake requires stdout to contain only a port number, which is
// incompatible with ax.Execute's flag-mounting and mode-resolution behavior.
func run() int {
	switch startModeFor(os.Args[0]) {
	case startModeLanguageHost:
		// Same command shape as the analyzer branch: stdout is the handshake,
		// and the context carries no CLI flags.
		dummyCmd := &cobra.Command{}
		dummyCmd.SetOut(os.Stdout)
		dummyCmd.SetContext(context.Background())
		if err := cli.RunAnalyzerLanguageHost(dummyCmd); err != nil {
			return 1
		}
		return 0
	case startModeAnalyzer:
		// RunAnalyzerServe sets up its own stderr logger, so this context only
		// carries cancellation.
		dummyCmd := &cobra.Command{}
		dummyCmd.SetContext(context.Background())
		if err := cli.RunAnalyzerServe(dummyCmd); err != nil {
			return 1
		}
		return 0
	case startModeCLI:
		// Regular CLI execution, routed through ax.Execute for deterministic exit
		// codes, ax.Error envelopes on stderr, and the agentic safety primitives
		// (--format/--dry-run/--yes/--idempotency-key, __schema discoverability).
		root := cli.NewRootCmd(version.GetVersion())
		return ax.Execute(context.Background(), root, ax.WithVersion(version.GetVersion()))
	}
	return 1
}

type startMode int

const (
	startModeCLI startMode = iota
	startModeAnalyzer
	startModeLanguageHost
)

// startModeFor reports which process entry the executable name selects.
// The language host is matched before the analyzer names. The match uses the
// base name, so a directory prefix and a Windows .exe suffix do not matter.
func startModeFor(exeName string) startMode {
	base := filepath.Base(exeName)
	switch {
	case strings.Contains(base, "pulumi-language-finfocus"):
		return startModeLanguageHost
	case strings.Contains(base, "pulumi-analyzer-policy-finfocus"),
		strings.Contains(base, "pulumi-analyzer-finfocus"):
		return startModeAnalyzer
	default:
		return startModeCLI
	}
}

func main() {
	os.Exit(run())
}
