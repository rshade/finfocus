package webui

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

// RedactProperties applies the shared attribute redaction rules to resource
// properties before they are serialized into a web payload: credential-like
// and "__" keys, Pulumi secret values, and unrepresentable values are
// dropped. It delegates to engine.BuildAttributes — the exact rule the CLI
// applies — so the web UI maintains no rule list of its own (FR-018).
func RedactProperties(ctx context.Context, props map[string]any) map[string]any {
	if len(props) == 0 {
		return nil
	}
	return engine.BuildAttributes(ctx, props).AsMap()
}

// RedactValue walks a decoded-JSON-shaped value (map[string]any, []any, or a
// scalar) and drops credential-like property names and Pulumi secret values
// at any depth, using the same shared rules the engine applies:
// engine.IsCredentialKey and history.IsPulumiSecret. Non-sensitive values are
// returned unchanged.
func RedactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if history.IsPulumiSecret(typed) {
			return nil
		}
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if engine.IsCredentialKey(key) || history.IsPulumiSecret(item) {
				continue
			}
			out[key] = RedactValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			if history.IsPulumiSecret(item) {
				continue
			}
			out = append(out, RedactValue(item))
		}
		return out
	default:
		return value
	}
}

// RedactJSON marshals v, applies the shared redaction rules to the decoded
// JSON, and returns the redacted encoding. Handlers use it for every payload
// the server emits so no secret or credential value crosses the wire,
// regardless of how the source structs evolve (FR-018).
func RedactJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("webui: marshal payload: %w", err)
	}
	var decoded any
	if uerr := json.Unmarshal(data, &decoded); uerr != nil {
		return nil, fmt.Errorf("webui: decode payload for redaction: %w", uerr)
	}
	redacted, err := json.Marshal(RedactValue(decoded))
	if err != nil {
		return nil, fmt.Errorf("webui: marshal redacted payload: %w", err)
	}
	return redacted, nil
}

// BudgetHealthPayload converts a BudgetResult into the BudgetHealthResult
// shape used by `finfocus overview --output json`. That shape carries no
// budget notification destinations (webhook URLs, Slack channels, headers),
// so the web payload is exactly as wide as the CLI JSON output and no wider
// (FR-018).
func BudgetHealthPayload(ctx context.Context, result *engine.BudgetResult) []engine.BudgetHealthResult {
	if result == nil {
		return nil
	}
	return engine.CalculateBudgetHealthResults(ctx, result.Budgets)
}
