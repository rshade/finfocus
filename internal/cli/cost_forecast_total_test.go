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

	"github.com/rshade/finfocus/internal/engine"
)

func TestForecastLiveInventoryPreservesPlanTotal(t *testing.T) {
	setupMembershipProject(t)
	plan, err := filepath.Abs("preview.json")
	require.NoError(t, err)
	specDir := t.TempDir()
	for sku, price := range map[string]string{"t3.large": "40", "t3.medium": "20"} {
		require.NoError(t, os.WriteFile(filepath.Join(specDir, "aws-ec2-"+sku+".yaml"), []byte(
			"provider: aws\nservice: ec2\nsku: "+sku+"\ncurrency: USD\npricing:\n  monthlyEstimate: "+price+"\n",
		), 0o600))
	}
	ctx := context.Background()
	resources, _, err := loadProjectedResources(
		ctx,
		membershipCommand(),
		costProjectedParams{},
		newAuditContext(ctx, "test", nil),
	)
	require.NoError(t, err)
	var stateCount int
	for _, resource := range resources {
		if resource.Origin == engine.OriginState {
			stateCount++
		}
	}
	assert.Positive(t, stateCount, "the forecast source must include unchanged state resources")
	run := func() forecastJSON {
		result := axtest.Run(ctx, t, NewRootCmd("test"), []string{
			"cost", "forecast", "--pulumi-json", plan, "--spec-dir", specDir,
			"--no-history", "--months", "1", "--output", "json",
		})
		require.Zero(t, result.ExitCode, "%s", result.Stderr)
		var doc forecastJSON
		require.NoError(t, json.Unmarshal(result.Stdout, &doc))
		require.NotEmpty(t, doc.Series)
		require.NotEmpty(t, doc.Series[0].Points)
		return doc
	}
	live := run()
	t.Chdir(t.TempDir())
	baseline := run()
	assert.InDelta(t, 20, live.Series[0].Points[0].Value, 1e-9)
	assert.InDelta(t, baseline.Series[0].Points[0].Value, live.Series[0].Points[0].Value, 1e-9,
		"unchanged inventory must not increase the v0.5.2 forecast total")
}
