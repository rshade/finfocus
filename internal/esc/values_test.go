package esc

import (
	"bytes"
	"context"
	"testing"

	sdk "github.com/pulumi/esc-sdk/sdk/go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentVariables(t *testing.T) {
	t.Parallel()
	secret := true
	plain := false
	env := &sdk.Environment{
		Properties: &map[string]sdk.Value{
			"environmentVariables": {
				Value: map[string]sdk.Value{
					"STR":  {Value: "hello", Secret: &secret},
					"NUM":  {Value: 1.5, Secret: &plain},
					"BOOL": {Value: true},
					"OBJ":  {Value: map[string]sdk.Value{"nested": {Value: "hidden-object"}}},
					"LIST": {Value: []any{"hidden-list"}},
					"NIL":  {Value: nil},
				},
				Trace: sdk.Trace{},
			},
		},
	}
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	got := environmentVariables(logger.WithContext(context.Background()), env)
	require.Len(t, got, 3)
	assert.Equal(t, entry{value: "hello", secret: true}, got["STR"])
	assert.Equal(t, entry{value: "1.5", secret: false}, got["NUM"])
	assert.Equal(t, entry{value: "true", secret: false}, got["BOOL"])
	logText := buf.String()
	assert.Contains(t, logText, "OBJ")
	assert.Contains(t, logText, "LIST")
	assert.NotContains(t, logText, "hidden-object")
	assert.NotContains(t, logText, "hidden-list")
	assert.NotContains(t, logText, "hello")
}

func TestEnvironmentVariablesMissing(t *testing.T) {
	t.Parallel()
	assert.Empty(t, environmentVariables(context.Background(), &sdk.Environment{}))
	assert.Empty(t, environmentVariables(context.Background(), &sdk.Environment{
		Properties: &map[string]sdk.Value{},
	}))
	assert.Empty(t, environmentVariables(context.Background(), nil))
}
