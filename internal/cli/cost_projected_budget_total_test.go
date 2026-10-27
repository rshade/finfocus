// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// Runs real CLI budget evaluation with isolated process config and Pulumi runner.
func TestProjectedBudgetUsesPlanTotalWithCompleteStack(t *testing.T) {
	setupMembershipProject(t)
	plan, err := filepath.Abs("preview.json")
	require.NoError(t, err)
	specDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "aws-ec2-t3.medium.yaml"), []byte(
		"provider: aws\nservice: ec2\nsku: t3.medium\ncurrency: USD\npricing:\n  monthlyEstimate: 20\n",
	), 0o600))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(os.Getenv("FINFOCUS_HOME"), "config.hujson"), []byte(
			`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[{"threshold":50,"type":"actual"}]}}}}`,
		), 0o600),
	)
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "aws-ec2-t3.large.yaml"), []byte(
		"provider: aws\nservice: ec2\nsku: t3.large\ncurrency: USD\npricing:\n  monthlyEstimate: 20\n",
	), 0o600))
	previous := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(previous) })
	run := func() axtest.Result {
		config.ResetGlobalConfigForTest()
		return axtest.Run(context.Background(), t, NewRootCmd("test"), []string{
			"cost", "projected", "--pulumi-json", plan, "--spec-dir", specDir,
			"--output", "json", "--exit-on-threshold", "--exit-code", "3",
		})
	}
	live := run()
	require.Zero(t, live.ExitCode, "%s", live.Stderr)
	var doc struct {
		Finfocus struct {
			Summary struct {
				TotalMonthly float64 `json:"totalMonthly"`
			} `json:"summary"`
			Stack struct {
				TotalMonthly float64 `json:"totalMonthly"`
				Complete     bool    `json:"complete"`
			} `json:"stack"`
		} `json:"finfocus"`
	}
	require.NoError(t, json.Unmarshal(live.Stdout, &doc))
	assert.InDelta(t, 20, doc.Finfocus.Summary.TotalMonthly, 1e-9)
	assert.True(t, doc.Finfocus.Stack.Complete)
	assert.Greater(
		t,
		doc.Finfocus.Stack.TotalMonthly,
		50.0,
		"the stack total crosses the budget threshold",
	)
	t.Chdir(t.TempDir())
	baseline := run()
	assert.Equal(
		t,
		baseline.ExitCode,
		live.ExitCode,
		"adding state inventory must preserve the baseline budget exit code",
	)
	// Prove this setup evaluates the configured threshold by moving it below
	// the plan total and observing the configured exit code.
	require.NoError(
		t,
		os.WriteFile(filepath.Join(os.Getenv("FINFOCUS_HOME"), "config.hujson"), []byte(
			`{"cost":{"budgets":{"global":{"amount":25,"currency":"USD","alerts":[{"threshold":50,"type":"actual"}]}}}}`,
		), 0o600),
	)
	exceeded := run()
	assert.Equal(t, 3, exceeded.ExitCode, "%s", exceeded.Stderr)
}
