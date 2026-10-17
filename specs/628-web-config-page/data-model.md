# Data Model: Web Config Page

## Setting

A known configuration item.

| Field | Meaning |
| --- | --- |
| Key | Dot path, such as `output.precision` or `plugins.aws.region` |
| Kind | `enum`, `int`, `number`, `bool`, `string`, `string_list`, `plugin`, `cluster_context` |
| Help | One line shown next to the control |
| Secret | True when the value must not leave the process |
| ReadOnly | True for alert destinations, routing, and analyzer |
| Enum | Allowed strings, when kind is `enum` |
| Min, Max | Inclusive bounds for `int` and `number` |

Editable keys are the `config set` surface:

- `output.default_format` (`table`, `json`, `ndjson`), `output.precision` (0–10)
- `logging.level` (`trace`, `debug`, `info`, `warn`, `error`), `logging.file`
- `plugin_host.strict_compatibility` (bool)
- `cost.budgets.global.amount` (number ≥ 0), `currency` (three letters), `period` (`monthly`)
- `scoring.enabled` (bool), `scoring.plugin`, `scoring.identifier_mode`
  (`pseudonymized`, `raw`, `omitted`), `scoring.timeout_seconds` (int ≥ 1),
  `scoring.field_allowlist` (comma-separated names),
  `scoring.needs_review.risk`, `false_positive`, `insufficient_evidence`,
  `dead_band` (0–1)
- `overview.cluster_contexts.<cluster>` (string; empty removes the entry)
- `plugins.<name>.<key>` (string). Secret when any segment matches
  `IsCredentialName`

`cost.budgets.amount`, `cost.budgets.currency`, and `cost.budgets.period`
are command aliases only. They are not settings on the page.

## Tier value

| Field | Meaning |
| --- | --- |
| Set | The tier has a value at this path |
| Value | Present only when `Secret` is false |
| Source label | `default`, `global`, `project`, or `env` on the effective value |

Precedence for the effective value:

1. Environment variable mapped to that key, when set and non-empty
2. Project file, when it contains the top-level section that owns the key
3. Global file, when it contains the path
4. Built-in default

Plugin environment names follow the existing scan:
`FINFOCUS_PLUGIN_<NAME>_<KEY>` becomes `plugins.<name>.<key>` in lower case.
`FINFOCUS_STRICT_COMPATIBILITY` is the environment source for
`plugin_host.strict_compatibility` even though `New()` does not copy it into
the struct.

A secret tier value is `{ "set": true }` or `{ "set": false }`. The wire
value never includes the secret, the masked placeholder, or a notification
URL or header.

## Config view

| Field | Meaning |
| --- | --- |
| ProjectAvailable | False when project resolution returns empty |
| ProjectUnavailableReason | Why the project tier cannot be selected |
| Versions | `global` and `project` content hashes (or `absent`) |
| Settings | Known settings with effective value, source, and per-tier values |
| ReadOnly | Alert, routing, and analyzer rows, secrets masked |

## Change

| Field | Meaning |
| --- | --- |
| Key | A setting key |
| Value | Replacement text. Omitted with `Clear: false` leaves a secret unchanged |
| Clear | Remove the key from the chosen tier |

`[REDACTED]` as `Value` is treated as "leave unchanged" and is never written.

## Write

| Field | Meaning |
| --- | --- |
| Tier | `global` or `project` |
| BaseVersion | Hash the page loaded for that tier |
| Changes | One or more `Change` values |

The server resolves the path. The write document has no path field.

## Validation result

Reuses `config.ValidationResult`: `valid`, `errors[]` with `path`, `message`,
`hint`, and `line`. A failed validation or a version mismatch does not
replace the file.

## State

```text
loaded view --preview--> diff (no write)
loaded view --save, matching version, valid--> new view
loaded view --save, stale version--> conflict, file unchanged
loaded view --save, invalid--> field errors, file unchanged
```
