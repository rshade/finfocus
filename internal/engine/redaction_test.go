// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
)

const leakMarker = "LEAK"

// encryptedSecret is a Pulumi secret as state stores it; decryptedSecret is
// the form a preview run with --show-secrets prints.
func encryptedSecret() map[string]any {
	return map[string]any{testPulumiSecretSig: "1", "ciphertext": "LEAK-cipher"}
}

func decryptedSecret() map[string]any {
	return map[string]any{testPulumiSecretSig: "1b47061264138c4ac30d75fd1eb44270", "value": "LEAK-plain"}
}

func TestConvertToProtoRedactsCredentialsAndSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		props map[string]any
		keep  map[string]string
	}{
		{
			name: "top-level credential keys",
			props: map[string]any{
				"password":   "LEAK-1",
				"dbPassword": "LEAK-2",
				"apiKey":     "LEAK-3",
				"region":     "us-east-1",
			},
			keep: map[string]string{"region": "us-east-1"},
		},
		{
			name:  "top-level secrets",
			props: map[string]any{"userData": encryptedSecret(), "connection": decryptedSecret(), "ami": "ami-1"},
			keep:  map[string]string{"ami": "ami-1"},
		},
		{
			name: "credential nested in a collapsed map",
			props: map[string]any{
				"db": map[string]any{"host": "db.internal", "port": 5432.0, "password": "LEAK-nested"},
			},
			keep: map[string]string{"db.host": "db.internal"},
		},
		{
			name:  "single-key credential map",
			props: map[string]any{"auth": map[string]any{"apiKey": "LEAK-single"}},
		},
		{
			name: "secret under a value key",
			props: map[string]any{
				"config": map[string]any{"value": decryptedSecret()},
			},
		},
		{
			name: "secret nested in a map with other fields",
			props: map[string]any{
				"settings": map[string]any{"region": "eu-west-1", "dsn": decryptedSecret()},
			},
			keep: map[string]string{"settings.region": "eu-west-1"},
		},
		{
			name:  "secret in an array",
			props: map[string]any{"args": []any{"--verbose", decryptedSecret(), encryptedSecret()}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tags := ConvertToProto(tt.props)
			for key, value := range tags {
				assert.NotContains(t, key, leakMarker)
				assert.NotContains(t, value, leakMarker, "tag %q", key)
			}
			for key, want := range tt.keep {
				assert.Equal(t, want, tags[key], "tag %q", key)
			}
		})
	}
}

func TestConvertValueToStringRedactsSecrets(t *testing.T) {
	t.Parallel()

	assert.Empty(t, ConvertValueToString(decryptedSecret()))
	assert.Empty(t, ConvertValueToString(encryptedSecret()))
	assert.NotContains(t, ConvertValueToString(map[string]any{
		"user": "admin", "token": "LEAK-token", "host": "h",
	}), leakMarker)
	assert.Equal(t, "t3.micro", ConvertValueToString(map[string]any{"value": "t3.micro"}))
}

func TestEstimateCostRedactsAttributes(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var sent []*pbc.EstimateCostRequest
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, in *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			mu.Lock()
			defer mu.Unlock()
			sent = append(sent, in)
			return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 10}, nil
		},
	}
	eng := New([]*pluginhost.Client{{Name: "test-plugin", API: mock}}, &mockSpecLoader{})

	_, err := eng.EstimateCost(context.Background(), &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider: "aws", Type: "aws:rds/instance:Instance", ID: "db",
			Properties: map[string]any{
				"instanceClass": "db.t3.micro",
				"password":      "LEAK-password",
				"masterUser":    map[string]any{"name": "admin", "secretToken": "LEAK-token"},
				"userData":      decryptedSecret(),
			},
		},
		PropertyOverrides: map[string]string{"instanceClass": "db.m5.large"},
	})
	require.NoError(t, err)

	require.Len(t, sent, 2)
	for _, req := range sent {
		encoded, marshalErr := protojson.Marshal(req.GetAttributes())
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(encoded), leakMarker)
		assert.Contains(t, req.GetAttributes().AsMap(), "instanceClass")
		assert.Equal(t, map[string]any{"name": "admin"}, req.GetAttributes().AsMap()["masterUser"])
	}
}

func TestBuildAttributesDoesNotLogOmittedValues(t *testing.T) {
	t.Parallel()

	ctx, logs := ctxWithLogBuffer(zerolog.DebugLevel)
	got := BuildAttributes(ctx, map[string]any{"userData": "LEAK-value\xff", "region": "us-east-1"})

	require.NotNil(t, got)
	assert.NotContains(t, got.AsMap(), "userData")
	assert.Contains(t, logs.String(), "userData", "the omission is still logged by key")
	assert.NotContains(t, logs.String(), leakMarker)
}

func TestIsCredentialKey(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"password", "dbPasswd", "sshPassphrase", "private_key", "privateKeyPem", "clientSecret",
		"authToken", "api_key", "AccessKey", "connection_string", "credentials",
	} {
		assert.True(t, isCredentialKey(key), key)
	}
	for _, key := range []string{"instanceType", "region", "passengers", "keyName"} {
		assert.False(t, isCredentialKey(key), key)
	}
}
