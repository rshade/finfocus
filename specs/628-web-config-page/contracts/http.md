# Contract: Config HTTP API

All routes sit behind the existing session cookie, Host check, and
`requestHardening` middleware. State-changing routes require a same-origin
`Origin` and `Content-Type: application/json`. Bodies use `decodeJSON`
(1 MiB, unknown fields rejected).

## GET /api/config

Returns the config view from [data-model.md](../data-model.md).

```json
{
  "projectAvailable": true,
  "projectUnavailableReason": "",
  "versions": { "global": "<hex sha256 or absent>", "project": "<hex sha256 or absent>" },
  "settings": [
    {
      "key": "output.precision",
      "kind": "int",
      "help": "Decimal places for cost amounts",
      "secret": false,
      "readOnly": false,
      "min": 0,
      "max": 10,
      "effective": { "source": "project", "value": 4 },
      "tiers": {
        "default": { "set": true, "value": 2 },
        "global": { "set": true, "value": 2 },
        "project": { "set": true, "value": 4 },
        "env": { "set": false }
      }
    }
  ],
  "readOnly": [
    {
      "key": "cost.budgets.global.alerts[0].notifications[0].url",
      "secret": true,
      "set": true,
      "source": "global"
    }
  ]
}
```

A secret setting omits `value` at every tier and on `effective`. The response
body must not contain stored secret material.

## POST /api/config/preview

Request:

```json
{
  "tier": "project",
  "baseVersion": "<versions.project from GET>",
  "changes": [
    { "key": "output.precision", "value": "3" },
    { "key": "logging.file", "clear": true },
    { "key": "plugins.aws.secret_key" }
  ]
}
```

A change without `value` and without `clear` leaves a secret unchanged.
`[REDACTED]` is the same as omitting `value`.

Response `200`:

```json
{
  "diff": [ { "key": "output.precision", "before": "2", "after": "3" } ],
  "validation": { "valid": true, "errors": [] }
}
```

Writes nothing. `400` with `code` `invalid_request` when `baseVersion` is
missing or empty. `409` when `baseVersion` does not match. `400` when
validation fails, with `code` `validation` and `fields` set to the
validation errors. The file is unchanged in both cases.

## POST /api/config

Same request as preview. On success, `200` and the same document as
`GET /api/config` after the write, plus `"message"` stating that the running
session keeps its startup configuration.

`409` body: `{ "error": "configuration changed since it was loaded; reload and try again", "code": "conflict" }`.

`400` when the project tier is selected and no project directory resolves:
`{ "error": "no project directory is available", "code": "project_unavailable" }`.

## Audit

The CLI audit record for a save contains `tier` and the changed `keys`.
It does not contain values. The request log stays method, path, and status.
