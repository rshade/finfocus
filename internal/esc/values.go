package esc

import (
	"context"
	"strconv"

	sdk "github.com/pulumi/esc-sdk/sdk/go"

	"github.com/rshade/finfocus/internal/logging"
)

type entry struct {
	value  string
	secret bool
}

func environmentVariables(ctx context.Context, env *sdk.Environment) map[string]entry {
	out := map[string]entry{}
	if env == nil || env.Properties == nil {
		return out
	}
	raw, ok := (*env.Properties)["environmentVariables"]
	if !ok {
		return out
	}
	variables, ok := raw.Value.(map[string]sdk.Value)
	if !ok || len(variables) == 0 {
		return out
	}
	for name, variable := range variables {
		value, keep := scalarString(variable.Value)
		if !keep {
			logDropped(ctx, name)
			continue
		}
		out[name] = entry{value: value, secret: secretFlag(variable.Secret)}
	}
	return out
}

func scalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}

func secretFlag(secret *bool) bool {
	return secret != nil && *secret
}

func logDropped(ctx context.Context, name string) {
	logging.FromContext(ctx).Debug().
		Str("component", "esc").
		Str("operation", "extract").
		Str("name", name).
		Msg("dropped non-scalar environment variable")
}
