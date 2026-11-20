# Data Model: Plugin Name Conformance

No persistent state is added. These are in-memory types and output fields.

## pluginhost.Client (changed)

| Field | Type | Meaning |
| --- | --- | --- |
| `Name` | string | Reported name from the `Name` RPC. Unchanged; still what the Adapter column shows. |
| `InstallName` | string | **New.** Install directory name, set when the registry opened the client. Empty for clients opened outside the registry (conformance runs, tests). |
| `Aliases` | []string | **New.** Former names from the plugin's catalog entry, set by the registry. Empty when the plugin has no catalog entry. |

Rules:

- `MatchesName(n)`: `n != ""` and `n` is `InstallName`, one of `Aliases`, or `Name`.
- `ResolveName(clients, n)`: install-name match first, then alias, then
  reported name; returns how it matched (`install`, `alias`, `reported`) and
  `ambiguous` when a different client also matched at a lower precedence.
- Pool holders copy the struct, so `InstallName` is the same on every holder of
  a shared process.

## router.DefaultRouter (changed)

| Field | Type | Meaning |
| --- | --- | --- |
| `configFor` | `map[*pluginhost.Client]*config.PluginRouting` | **New.** Each routing entry resolved to its client once, at construction. Replaces the name-keyed `pluginConfig` lookups. |
| `unmatched` | `[]string` | **New.** Routing entry names that resolved to no client. Exposed by `UnmatchedEntries()`. |

## router.ValidationResult (unchanged type, new messages)

| Case | Kind | Message |
| --- | --- | --- |
| Name resolves to no client | error | `plugin not found` (today), followed by `(did you mean "<x>"?)` when the name is `finfocus-plugin-<x>` and `<x>` resolves |
| Name is one plugin's install name and another's reported name | warning | `name matches install name of <a> and reported name of <b>; using <a>` |
| Name resolves only through a catalog alias that is still the plugin's reported name | warning | `"<alias>" will be retired; use "<install>"` |
| Name resolves only through a catalog alias the plugin no longer reports | warning | `"<alias>" is a former name of "<install>"; use "<install>"` |

## registry.RegistryEntry (changed)

| Field | JSON | Meaning |
| --- | --- | --- |
| `Aliases` | `aliases` | **New.** Former names. Each passes `pluginname.Valid` and is unique across every entry name and alias in the catalog. Seeded: aws-public `finfocus-plugin-aws-public`, cloudzero `finfocus-plugin-cloudzero`. |

## conformance.Status (changed)

`pass`, `fail`, `skip`, `error`, and **new** `warn`.

## conformance.Summary (changed)

| Field | JSON | Meaning |
| --- | --- | --- |
| `Warnings` | `warnings` | **New.** Count of `warn` results. Not counted as failures or errors. |

## conformance.PluginIdentity (new)

| Field | Type | Meaning |
| --- | --- | --- |
| `Expected` | string | Expected install name, empty when unresolved. |
| `Source` | string | `flag`, `install-layout`, or `binary-name`; empty when unresolved. |
| `Reason` | string | Why it could not be resolved; the install-name check warns with it. |

Carried on `TestContext.Identity`, resolved once per suite run.

## conformance.SuiteConfig (changed)

| Field | Type | Meaning |
| --- | --- | --- |
| `ExpectedName` | string | **New.** From `--expected-name`; highest-precedence source. |

## pluginname (new leaf package)

| Symbol | Meaning |
| --- | --- |
| `BinaryPrefix` | `finfocus-plugin-` |
| `Valid(name) bool` | Existing `plugin init` rule, moved here |
| `Canonical(name) error` | `Valid`, no `BinaryPrefix`, no doubled hyphen; error says which |
| `FromBinary(path, region) (string, bool)` | Strip `.exe`, `BinaryPrefix`, `-<region>` |

## Routes test output (changed)

`RoutesTestOutput` (JSON) gains `warnings []RouteWarningOutput`:

| Field | JSON | Meaning |
| --- | --- | --- |
| `Plugin` | `plugin` | Routing entry name |
| `Message` | `message` | Why it is not simulated (with an optional `did you mean` hint), or that the name is a former name |

Omitted when empty, so JSON for a clean config is unchanged.
