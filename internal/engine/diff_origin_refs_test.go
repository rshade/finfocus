// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestProjectedPlanReferencesIgnoreAppendedState(t *testing.T) {
	t.Parallel()
	for _, op := range []string{DiffOperationCreate, DiffOperationUpdate, DiffOperationDelete, DiffOperationSame} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			pool := referencedNodePool(op, "Standard_D4s_v3", "Standard_D2s_v3")
			target := referencedCluster(DiffOperationSame)
			target.Origin = OriginState
			run := func(resources []ResourceDescriptor) ([]map[string]string, []map[string]any) {
				client := &recordingCostClient{}
				_, err := New(
					[]*pluginhost.Client{{Name: "azure", API: client}},
					nil,
				).GetProjectedCostDiff(context.Background(), resources)
				require.NoError(t, err)
				client.mu.Lock()
				defer client.mu.Unlock()
				var properties []map[string]string
				var attributes []map[string]any
				for _, request := range client.projected {
					for _, resource := range request.Resources {
						if resource.ID == nodePoolURN {
							properties = append(properties, resource.Properties)
							attributes = append(attributes, resource.Attributes.AsMap())
						}
					}
				}
				require.NotEmpty(t, properties)
				return properties, attributes
			}
			baselineProps, baselineAttrs := run([]ResourceDescriptor{pool})
			liveProps, liveAttrs := run([]ResourceDescriptor{pool, target})
			assert.Equal(t, baselineProps, liveProps, "state inventory must not change plugin-bound plan properties")
			assert.Equal(t, baselineAttrs, liveAttrs, "state inventory must not change plugin-bound plan attributes")
		})
	}
}

func TestProjectedStateReferencesUsePlanTargets(t *testing.T) {
	t.Parallel()
	pool := referencedNodePool(DiffOperationSame, "Standard_D2s_v3", "")
	pool.Origin = OriginState
	client := newRegionPlugin(map[string]float64{"Standard_D2s_v3": 10})
	_, err := New(
		[]*pluginhost.Client{{Name: "azure", API: client}},
		nil,
	).GetProjectedCostDiff(context.Background(), []ResourceDescriptor{referencedCluster(DiffOperationSame), pool})
	require.NoError(t, err)
	assert.Equal(t, []string{"westeurope"}, client.regionsFor(pool.ID))
}
