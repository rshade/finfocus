// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

func setupMembershipProject(t *testing.T) {
	t.Helper()
	state, err := os.ReadFile("testdata/pulumi/stack-export-dev.json")
	require.NoError(t, err)
	plan, err := os.ReadFile("testdata/pulumi/preview-create-only.json")
	require.NoError(t, err)
	project := t.TempDir()
	writePulumiProject(t, project, false)
	require.NoError(t, os.WriteFile(filepath.Join(project, "preview.json"), plan, 0o600))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_HISTORY_ENABLED", "false")
	t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())
	t.Chdir(project)
	installFakePulumi(t)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: state, preview: plan}))
}

func membershipCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("stack", "dev", "")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func membershipURN(name, resourceType string) string {
	return "urn:pulumi:dev::test-stack::" + resourceType + "::" + name
}

func liveMembershipURNs() []string {
	return []string{
		membershipURN("new", "aws:ec2/instance:Instance"),
		membershipURN("web-server", "aws:ec2/instance:Instance"),
		membershipURN("database", "aws:rds/instance:Instance"),
		membershipURN("storage", "aws:s3/bucket:Bucket"),
		membershipURN("queue", "aws:ec2/instance:Instance"),
		membershipURN("secret", "aws:ec2/instance:Instance"),
	}
}

//nolint:paralleltest // Installs a process-wide Pulumi runner and changes the working directory.
func TestRecommendationsRequestsLiveInventory(t *testing.T) {
	setupMembershipProject(t)
	ctx := context.Background()
	cmd := membershipCommand()
	resources, unchanged, err := loadRecommendationsResources(
		ctx,
		cmd,
		costRecommendationsParams{},
		newAuditContext(ctx, "test", nil),
	)
	require.NoError(t, err)
	assert.False(t, unchanged)
	client := &includeDismissedClient{}
	eng := engine.New([]*pluginhost.Client{{Name: "aws", API: client}}, nil)
	_, err = fetchRecommendationsWithProgress(ctx, cmd, eng, resources, false)
	require.NoError(t, err)
	var requested []string
	for _, request := range client.requests() {
		for _, resource := range request.TargetResources {
			requested = append(requested, resource.ID)
		}
	}
	assert.ElementsMatch(
		t,
		liveMembershipURNs(),
		requested,
		"recommendations must reach the plugin for creates and every unchanged deployed resource",
	)
}

type membershipRecommendationFetcher struct {
	resources []engine.ResourceDescriptor
}

func (f *membershipRecommendationFetcher) GetRecommendationsForResources(
	_ context.Context, resources []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	f.resources = append(f.resources, resources...)
	result := &engine.RecommendationsResult{}
	for _, resource := range resources {
		result.Recommendations = append(
			result.Recommendations,
			engine.Recommendation{ResourceID: resource.ID, Type: "RIGHTSIZE"},
		)
	}
	return result, nil
}

//nolint:paralleltest // Installs a process-wide Pulumi runner and changes the working directory.
func TestProjectedRecommendationsMatchEntriesAfterOriginSplit(t *testing.T) {
	setupMembershipProject(t)
	ctx := context.Background()
	resources, _, err := loadProjectedResources(
		ctx,
		membershipCommand(),
		costProjectedParams{},
		newAuditContext(ctx, "test", nil),
	)
	require.NoError(t, err)
	eng := engine.New([]*pluginhost.Client{{Name: "aws", API: &previewPricingClient{}}}, nil)
	diff, err := eng.GetProjectedCostDiff(ctx, resources)
	require.NoError(t, err)
	require.Len(t, diff.Entries, 1)
	fetcher := &membershipRecommendationFetcher{}
	mergeProjectedDiffRecommendations(ctx, fetcher, resources, diff)
	require.Len(t, fetcher.resources, 1)
	assert.Equal(t, membershipURN("new", "aws:ec2/instance:Instance"), fetcher.resources[0].ID)
	require.Len(t, diff.Entries[0].After.Recommendations, 1)
	assert.Equal(t, diff.Entries[0].ResourceID, diff.Entries[0].After.Recommendations[0].ResourceID)
}

// A detected Pulumi project supplies the live inventory, as for cost projected.
//
//nolint:paralleltest // Configures the process-wide Pulumi runner, environment, and working directory.
func TestRecommendationsInputsAcceptDetectedPulumiProject(t *testing.T) {
	setupMembershipProject(t)
	require.NoError(t, validateRecommendationsInputs(membershipCommand(), costRecommendationsParams{}))
}

// Configures the working directory.
func TestRecommendationsInputsRequireSourceOutsideProject(t *testing.T) {
	t.Chdir(t.TempDir())
	err := validateRecommendationsInputs(membershipCommand(), costRecommendationsParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Pulumi or Terraform project")
}
