// Package main provides the finfocus CLI tool for calculating cloud infrastructure costs.
// It supports both projected costs from Pulumi infrastructure definitions and actual historical
// costs from cloud provider APIs via a plugin-based architecture.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/pkg/version"
)

// run executes the main application logic for the finfocus program.
// It either starts the Pulumi analyzer plugin serve path when the executable name
// indicates an analyzer invocation (supports both legacy policy-pack and direct
// analyzer names), or it runs the regular CLI root command for normal operation.
// It returns an error if starting the analyzer serve or executing the root command fails.
func run() error {
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
		return cli.RunAnalyzerServe(dummyCmd)
	}

	// Original CLI execution for the main finfocus CLI tool
	// Initialize a minimal startup logger for early error reporting
	startupCfg := logging.LoggingConfig{
		Level:  "error",
		Format: "json",
		Output: "stderr",
	}
	startupLogger := logging.NewLogger(startupCfg)
	startupLogger = logging.ComponentLogger(startupLogger, "main")

	root := cli.NewRootCmd(version.GetVersion())
	if err := root.Execute(); err != nil {
		// Print user-friendly error to stderr for immediate visibility
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		// Also log for debugging purposes
		startupLogger.Error().Err(err).Msg("command execution failed")
		return err
	}
	return nil
}

// extractBudgetExitCode returns the custom exit code from a BudgetExitError,
// or 1 for non-budget errors, or 0 for nil errors.
// Invariant: when err is non-nil the returned code is always >= 1.
func extractBudgetExitCode(err error) int {
	if err == nil {
		return 0
	}
	var budgetErr *cli.BudgetExitError
	if errors.As(err, &budgetErr) && budgetErr.ExitCode != 0 {
		return budgetErr.ExitCode
	}
	return 1
}

func main() {
	if err := run(); err != nil {
		os.Exit(extractBudgetExitCode(err))
	}
}
