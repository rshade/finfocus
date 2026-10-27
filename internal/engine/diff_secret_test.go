// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestProjectedDiffStateOriginRedactsSecrets(t *testing.T) {
	t.Parallel()
	plugin := &recordingCostClient{}
	eng := New([]*pluginhost.Client{{Name: "test", API: plugin}}, nil)
	resource := diffResource("state-resource", "same", "t3.micro", "")
	resource.Origin = OriginState
	resource.Properties["userData"] = encryptedSecret()
	resource.Properties["settings"] = map[string]any{
		"region": "us-east-1",
		"dsn":    decryptedSecret(),
	}
	_, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{resource})
	require.NoError(t, err)
	plugin.mu.Lock()
	defer plugin.mu.Unlock()
	require.Len(t, plugin.projected, 1)
	require.Len(t, plugin.projected[0].Resources, 1)
	sent := plugin.projected[0].Resources[0]
	encoded, err := json.Marshal([]any{sent.Properties, sent.Attributes.AsMap()})
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "LEAK-")
	assert.NotContains(t, string(encoded), testPulumiSecretSig)
	assert.NotContains(t, string(encoded), "ciphertext")
	require.Len(t, plugin.projected[0].Resources, 1)
	assert.Equal(t, "t3.micro", plugin.projected[0].Resources[0].Properties["instanceType"])
	assert.Equal(t, encryptedSecret(), resource.Properties["userData"])
}
