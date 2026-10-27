// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

// Installs a process-wide Pulumi runner and changes the working directory.
//
//nolint:paralleltest // Configures the process-wide Pulumi runner and environment.
func TestActualDeployedInventoryKeepsDeletedHistory(t *testing.T) {
	setupMembershipProject(t)
	ctx := context.Background()
	cmd := membershipCommand()
	resources, err := loadActualResources(
		ctx,
		cmd,
		costActualParams{},
		newAuditContext(ctx, "test", nil),
	)
	require.NoError(t, err)
	var deployed []string
	for _, resource := range resources {
		if !engine.IsPulumiInternalType(resource.Type) {
			deployed = append(deployed, resource.ID)
		}
	}
	assert.ElementsMatch(t, liveMembershipURNs()[1:], deployed,
		"actual billing uses every deployed resource and excludes the plan-only create")
	store, err := history.NewBoltStore(ctx, t.TempDir(), true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	stack, detected := detectHistoryStackContext(ctx)
	require.True(t, detected)
	now := time.Now()
	deletedURN := membershipURN("deleted-earlier", "aws:ec2/instance:Instance")
	require.NoError(t, store.Upsert(stack.Hash(), history.ResourceHistoryEntry{
		URN: deletedURN, CloudID: "i-deleted-earlier", Type: "aws:ec2/instance:Instance", Provider: "aws",
		FirstSeen: now.Add(-48 * time.Hour).
			Unix(),
		LastSeen: now.Add(-24 * time.Hour).Unix(), Source: history.SourceStateSnapshot,
	}))
	enriched := enrichWithHistoricalResources(ctx, store, resources, now.Add(-72*time.Hour), now)
	var cloudIDs []string
	for _, resource := range enriched {
		if id, ok := resource.Properties["pulumi:cloudId"].(string); ok {
			cloudIDs = append(cloudIDs, id)
		}
	}
	assert.Contains(
		t,
		cloudIDs,
		"i-deleted-earlier",
		"actual costs must still query deleted resources active during the billing period",
	)
}

// Configures the process-wide Pulumi runner, environment, and working directory.
func TestActualSavedPreviewUsesSharedInventory(t *testing.T) {
	setupMembershipProject(t)
	ctx := context.Background()
	plan, err := filepath.Abs("preview.json")
	require.NoError(t, err)
	ids := func(resources []engine.ResourceDescriptor) []string {
		var out []string
		for _, resource := range resources {
			if !engine.IsPulumiInternalType(resource.Type) {
				out = append(out, resource.ID)
			}
		}
		return out
	}
	params := costActualParams{planPath: plan}

	inProject, err := loadActualResources(ctx, membershipCommand(), params, newAuditContext(ctx, "test", nil))
	require.NoError(t, err)
	assert.ElementsMatch(t, liveMembershipURNs()[1:], ids(inProject),
		"inside a project, a saved preview reads the deployed inventory like the web Cost view")

	t.Chdir(t.TempDir())
	outside, err := loadActualResources(ctx, membershipCommand(), params, newAuditContext(ctx, "test", nil))
	require.NoError(t, err)
	assert.Contains(t, ids(outside), liveMembershipURNs()[0],
		"without state, a saved preview keeps its v0.5.2 inventory")
}

func TestRenderActualCostOutputEmptyTimeGrouping(t *testing.T) {
	t.Parallel()
	for _, group := range []string{"daily", "monthly"} {
		for format, want := range map[engine.OutputFormat]string{
			engine.OutputJSON:   "[]\n",
			engine.OutputNDJSON: "",
			engine.OutputTable:  "No cost data available for cross-provider aggregation\n",
		} {
			t.Run(group+"/"+string(format), func(t *testing.T) {
				t.Parallel()
				var out strings.Builder
				require.NoError(t, renderActualCostOutput(&out, format, nil, group, false))
				assert.Equal(t, want, out.String(), "nothing deployed is an empty result, not an error")
			})
		}
	}
}
