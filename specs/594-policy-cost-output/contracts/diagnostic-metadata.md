# Diagnostic Cost Metadata Contract

## Format

Machine-parseable cost metadata is embedded in the diagnostic `Message` field as an HTML comment, appended after the human-readable text.

## Schema

```text
<human-readable message>
<!-- finfocus:cost:<JSON> -->
```

### Per-Resource Metadata (in CostToDiagnostic Message)

```json
{"monthly":150.00,"currency":"USD","adapter":"aws-public"}
```

| Field | Type | Description |
| ----- | ---- | ----------- |
| monthly | number | Monthly cost estimate |
| currency | string | ISO 4217 currency code |
| adapter | string | Source of pricing data |

### Example

```text
Estimated Monthly Cost: $150.00 USD (source: aws-public)
<!-- finfocus:cost:{"monthly":150.00,"currency":"USD","adapter":"aws-public"} -->
```

## Parsing

To extract metadata from a diagnostic message:

1. Find the substring `<!-- finfocus:cost:`
2. Extract content between that prefix and the closing `-->`
3. Parse the extracted content as JSON

### Regex Pattern

```text
<!-- finfocus:cost:(.*?) -->
```

### Example (pseudocode)

```text
match = regex("<!-- finfocus:cost:(.*?) -->", message)
if match:
    metadata = json.parse(match.group(1))
```

## Compatibility

- The HTML comment is always the last line of the message
- The human-readable text preceding the comment is unchanged from current behavior
- If no metadata is present (e.g., zero-cost internal resources), no comment is appended
- The metadata JSON is always a single line (no pretty-printing)
