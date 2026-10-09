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
		if sensitiveRecord(typed) || history.IsPulumiSecret(typed) {
			return nil
		}
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if engine.IsCredentialKey(key) || history.IsPulumiSecret(item) {
				continue
			}
			if key == "deltas" {
				item = redactSecretDeltas(item, typed["resource"])
			}
			out[key] = RedactValue(item)
		}
		return out
	case []any:
		return redactArray(typed)
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
	safe := RedactValue(decoded)
	// Only this typed response contains engine-computed numeric domain maps.
	// Preserve their canonical labels after recursive redaction, while resource
	// properties, breakdowns and arbitrary dictionaries retain the shared rules.
	if response, ok := v.(actualCostQueryResponse); ok {
		if object, isObject := safe.(map[string]any); isObject {
			if summary, isSummary := object["summary"].(map[string]any); isSummary {
				summary["byService"] = response.Summary.ByService
				summary["byProvider"] = response.Summary.ByProvider
				summary["byAdapter"] = response.Summary.ByAdapter
			}
		}
	}
	// Only the typed session snapshot carries this operational boolean. Raw
	// property dictionaries still follow engine credential-name redaction.
	if snapshot, ok := v.(OverviewSnapshot); ok {
		if object, isObject := safe.(map[string]any); isObject {
			object["passphraseRequired"] = snapshot.PassphraseRequired
		}
	}
	redacted, err := json.Marshal(safe)
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

// sensitiveRecord recognizes named property diffs and estimate deltas without
// introducing another credential rule list.
func sensitiveRecord(record map[string]any) bool {
	for _, field := range []string{"key", "property"} {
		if name, ok := record[field].(string); ok && engine.IsCredentialKey(name) {
			return true
		}
	}
	for _, field := range []string{"oldValue", "newValue", "originalValue"} {
		value, ok := record[field]
		if !ok {
			continue
		}
		if history.IsPulumiSecret(value) {
			return true
		}
		if text, isString := value.(string); isString {
			var decoded any
			if json.Unmarshal([]byte(text), &decoded) == nil && history.IsPulumiSecret(decoded) {
				return true
			}
		}
	}
	return false
}

func redactArray(items []any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		if record, ok := item.(map[string]any); ok && sensitiveRecord(record) {
			continue
		}
		if history.IsPulumiSecret(item) {
			continue
		}
		out = append(out, RedactValue(item))
	}
	return out
}

// A secret property's engine delta has an intentionally blank original value.
// Retain the source property's sensitivity while the original descriptor is
// still available, before the recursive writer removes that descriptor value.
func redactSecretDeltas(value, resource any) any {
	descriptor, ok := resource.(map[string]any)
	if !ok {
		return value
	}
	props, ok := descriptor["properties"].(map[string]any)
	if !ok {
		return value
	}
	deltas, ok := value.([]any)
	if !ok {
		return value
	}
	safe := make([]any, 0, len(deltas))
	for _, delta := range deltas {
		record, isRecord := delta.(map[string]any)
		if isRecord {
			property, _ := record["property"].(string)
			if history.IsPulumiSecret(props[property]) {
				continue
			}
		}
		safe = append(safe, delta)
	}
	return safe
}
