# Data Model: Web Plugin Page

## CatalogPlugin

One row on the Plugins page.

| Field | Meaning |
| --- | --- |
| name | Catalog name, or the install directory name when the plugin is outside the catalog |
| description | Catalog description, empty outside the catalog |
| providers | Catalog `supported_providers` |
| capabilities | Catalog capability strings |
| securityLevel | Catalog `security_level`, empty outside the catalog |
| repository | Catalog `owner/repo` source, empty outside the catalog |
| inCatalog | False for an installed plugin the built-in catalog does not list |
| installedVersions | Versions present as directories under the discovery plugin dir, sorted |
| latestVersion | Highest installable catalog version when known; empty outside the catalog or when the lookup fails |
| updateAvailable | True when an installed version is older than `latestVersion` |

`latestVersion` lookup failure is a row warning, not a failed page. The rest
of the catalog still renders.

## PluginPlan

The confirmation summary. Produced before any download or delete.

| Field | Meaning |
| --- | --- |
| action | `install`, `update`, or `remove` |
| name | Plugin name |
| version | Version the operation will apply |
| repository | Source repository for install and update; empty for remove |
| securityLevel | Catalog security level; empty outside the catalog |
| fallbackFrom | Requested version when `version` is a fallback; empty otherwise |
| checksumFile | `present`, `missing`, or `unknown` (remove) |
| inCatalog | Copied from the catalog row |

Validation:

- `install` and `update` require `inCatalog`.
- `remove` requires `version` to be one of `installedVersions`.
- A URL or `owner/repo` specifier never becomes a plan.

## PluginOperation

An in-memory install, update, or remove.

| Field | Meaning |
| --- | --- |
| id | Unguessable id |
| name | Plugin the lock covers |
| status | `running`, `succeeded`, `failed` |
| events | Ordered `ProgressEvent` values |
| error | Safe message when failed. No token, secret, or URL userinfo |

Only one `running` operation per plugin name. The server drops the record
when the session shuts down.

## ProgressEvent

| Field | Meaning |
| --- | --- |
| phase | `resolve`, `download`, `verify`, `extract`, `validate`, `done` |
| message | The same sentence the CLI string callback would print, when one exists |
| bytes | Bytes downloaded, zero outside download |
| total | Total bytes, zero when unknown |
| warning | True for a non-fatal warning |

## SettingDescriptor

A copy of one `PluginSetting` from `GetPluginInfo`, with no secret value.

| Field | Meaning |
| --- | --- |
| key | `plugins.<name>.<key>` storage key is this key under the plugin |
| type | `string`, `int`, `bool`, `enum`, `url`, `duration` |
| required | Plugin flag |
| secret | Plugin flag, or true when a generic key matches `IsCredentialName` |
| default | Descriptor default. A secret default is reported as set or unset only |
| description | Plugin text, at most 1024 bytes |
| env | Variable the plugin reads. Not shown as the only label; shown as help |
| enumValues | Present only for `enum` |

Generic mode has no descriptor. Each stored key is a string row. Credential
names are secret.

## SettingsView

| Field | Meaning |
| --- | --- |
| name | Plugin name |
| mode | `descriptor` or `generic` |
| descriptors | Empty in generic mode |
| versions | Same `global` and `project` file hashes as the Config page |
| projectAvailable | Same rule as the Config page |
| values | Per key: project set/value, global set/value, env set (never the env value), effective source |
| message | Empty until a save |

Secret rows omit every value. `env set` is true when the parent environment
contains that descriptor's variable or the generic `FINFOCUS_PLUGIN_<NAME>_<KEY>`
variable. The page cannot override that variable.

## DeliveredSetting

The value a plugin process receives for one variable.

Precedence, first hit wins:

1. The variable is already present in the parent environment (value unchanged).
2. Project configuration `plugins.<name>.<key>`.
3. Global configuration `plugins.<name>.<key>`.
4. Descriptor default.

A key with a descriptor is delivered only as `descriptor.env` into that
plugin's process. A stored key with no descriptor is delivered only as
`FINFOCUS_PLUGIN_<NAME>_<KEY>` into that plugin's process. No other plugin
receives either variable.

## ValidateResult

| Field | Meaning |
| --- | --- |
| passed | True only when file checks and `GetPluginInfo` both succeed |
| reason | Empty on success. On failure, the file-check or launch error with secrets redacted |

## State transitions

```text
plan (no disk change)
  -> confirmed operation running
      -> succeeded (files match plugin list)
      -> failed (no new installed version; partial downloads removed)
busy: a second operation for the same name while running -> failed, no disk change
```

Settings have no separate lifecycle. Preview writes nothing. Save either
patches one tier or leaves that file byte-for-byte unchanged.
