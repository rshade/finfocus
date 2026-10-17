# Contract: Plugins HTTP API

All routes sit behind the existing session cookie, Host check, and
`requestHardening` middleware. State-changing routes require a same-origin
`Origin` and `Content-Type: application/json`. Bodies use `decodeJSON`
(1 MiB, unknown fields rejected). Error bodies use the existing `writeError` shape:
`{ "error": "<safe text>", "code": "<code>" }`.

No response, log, or audit line contains a secret setting value, a GitHub
token, or a download URL that carries userinfo.

## GET /api/plugins/catalog

Returns `{ "plugins": [ CatalogPlugin, … ] }` as defined in
[data-model.md](../data-model.md). Every built-in catalog entry is present.
Installed plugins that are not in the catalog are present with
`"inCatalog": false`.

## POST /api/plugins/{name}/plan

Request:

```json
{ "action": "install", "version": "1.2.3" }
```

`version` is optional for `install` and `update` (latest). `remove` requires
`version`.

Response `200` is a `PluginPlan`. The call does not download or delete.

Errors:

| Status | Code | When |
| --- | --- | --- |
| 400 | `invalid_request` | Unknown action, remove without a version, or a name that is not a catalog slug |
| 404 | `not_found` | Install or update of a name outside the catalog, or remove of a version that is not installed |
| 502 | `upstream` | Catalog host error. A rate limit uses the message `GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits` |

When the requested release has no assets and an older one does, the plan
returns `200` with `version` set to the fallback and `fallbackFrom` set to
the requested version. It does not install it.

## POST /api/plugins/{name}/install

## POST /api/plugins/{name}/update

Request:

```json
{
  "version": "1.2.3",
  "repository": "rshade/finfocus-plugin-aws-public",
  "securityLevel": "community",
  "acceptMissingChecksum": false,
  "acceptFallbackFrom": ""
}
```

The server builds a fresh plan and rejects the call with `409` `conflict`
when `version`, `repository`, or `securityLevel` disagree with that plan.
`acceptFallbackFrom` must equal the plan's `fallbackFrom` when the plan is a
fallback, and must be empty otherwise. `acceptMissingChecksum` must be true
when the plan's `checksumFile` is `missing`.

`update` is an install of the latest catalog version when the plugin is
absent, after the same confirmation.

Response `202`:

```json
{ "operationId": "<id>" }
```

The work continues after the response. `409` `busy` when this plugin already
has a running operation or the installer lock is held. `403` when Origin or
content type fails (middleware).

## DELETE /api/plugins/{name}/{version}

Request body:

```json
{
  "repository": "",
  "securityLevel": ""
}
```

Same match rule against a `remove` plan. Outside-catalog plugins send empty
repository and security level. Response `202` with `operationId`, or `409`.

## GET /api/plugins/operations/{id}/stream

`Content-Type: text/event-stream`.

First event is `snapshot`:

```json
{ "status": "running", "events": [], "error": "" }
```

Following events are `progress` (`ProgressEvent`) and a final `done`:

```json
{ "status": "succeeded", "error": "" }
```

`status` is `succeeded` or `failed`. An unknown id is `404` JSON, not a
stream. A client that connects after completion still receives the snapshot
and the terminal `done`.

## POST /api/plugins/{name}/validate

Request body `{}`. Response:

```json
{ "passed": true, "reason": "" }
```

`passed: false` is still HTTP `200`. The reason is the file-check or launch
failure. `404` when the name is not installed. The call does not install,
and it does not run conformance or certification.

## GET /api/plugins/{name}/settings

Returns a `SettingsView`. `404` when the name is neither cataloged nor
installed. Secret fields have no `value` property anywhere in the body.
A descriptor default that is secret is `{ "set": true }` or `{ "set": false }`
only.

## POST /api/plugins/{name}/settings/preview

## POST /api/plugins/{name}/settings

Request is the Config page `WriteRequest` (`tier`, `baseVersion`, `changes`).
Every change key must be `plugins.<name>.<key>` for this path's name. The
handler delegates to `ConfigPreview` or `ConfigSave`.

Additional rejection before the writer runs:

| Status | Code | When |
| --- | --- | --- |
| 400 | `invalid_request` | A key outside this plugin, an unknown field, or a value that does not match the descriptor type |
| 400 | `invalid_config` | The shared config validation failed. Body matches the Config page field errors |
| 409 | `conflict` | `baseVersion` does not match the tier file |

Descriptor checks:

- `int`: a base-10 integer in the int64 range.
- `bool`: `true` or `false`.
- `enum`: one of `enumValues`.
- `url`: absolute URL with a scheme and a host.
- `duration`: a Go duration the plugins accept (`30s`, `1h30m`).
- `string`: any string the config writer already accepts.
- `required`: rejected when the effective value would be empty and the
  descriptor default is empty. A parent environment value or a default
  satisfies required without a new change.
- A secret change with an empty value and `clear: false` is omitted, not
  written. `clear: true` removes the tier value. The masked placeholder is
  rejected.

Preview response matches `POST /api/config/preview`. Save response matches
`POST /api/config` and adds:

```json
{ "message": "Saved. These settings apply the next time this plugin is launched." }
```

The running dashboard does not inject the new values into a plugin process
that is already running. The next `NewClient` for that plugin does.
