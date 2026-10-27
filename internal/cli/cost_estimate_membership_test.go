// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Installs a process-wide Pulumi runner and changes the working directory.
func TestEstimateNoMatchListsLiveInventory(t *testing.T) {
	setupMembershipProject(t)
	result := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{
		"cost", "estimate", "--pulumi-json", "preview.json", "--modify", "missing:instanceType=t3.large",
	})
	require.Equal(t, 2, result.ExitCode, "%s", result.Stderr)
	assert.Contains(t, string(result.Stderr), "no resources matched --modify")
	for _, urn := range liveMembershipURNs() {
		parts := strings.Split(urn, "::")
		assert.Contains(t, string(result.Stderr), parts[len(parts)-1]+" ("+parts[len(parts)-2]+")")
	}
}

//nolint:paralleltest // Installs a process-wide Pulumi runner and changes the working directory.
func TestEstimateDetectedPulumiNoMatchListsLiveInventory(t *testing.T) {
	setupMembershipProject(t)
	result := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{
		"cost", "estimate", "--stack", "dev", "--modify", "nomatch:size=1",
	})
	require.Equal(t, 2, result.ExitCode, "%s", result.Stderr)
	require.Contains(t, string(result.Stderr), "no resources matched --modify")
	for _, urn := range liveMembershipURNs() {
		parts := strings.Split(urn, "::")
		assert.Contains(t, string(result.Stderr), parts[len(parts)-1]+" ("+parts[len(parts)-2]+")")
	}
}

//nolint:paralleltest // Installs a process-wide Pulumi runner and changes the working directory.
func TestEstimateDetectedPulumiPreservesSingleResourceValidation(t *testing.T) {
	setupMembershipProject(t)
	result := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{
		"cost", "estimate", "--provider", "aws", "--resource-type", "invalid",
	})
	require.Equal(t, 2, result.ExitCode, "%s", result.Stderr)
	assert.Contains(t, string(result.Stderr), "not a full type token")
	assert.NotContains(t, string(result.Stderr), "cannot mix")
}
