# Data Model

## `config.ESCPluginConfig.Credentials`

`map[string]string`, JSON `credentials`, omitted when empty. Copied by
`CloneESC` and `PluginOptionsForESC` into `esc.PluginOptions.Credentials`, and
by `esc.copyOptions`. `esc.Source.Credentials(plugin)` returns a copy.

## `credentialPlan` (pluginhost, per Acquire)

| Field | Meaning |
| --- | --- |
| `variables` | Declared ESC variables to withhold. Empty means not opted in |
| `set` | This holder's `pluginsdk.Credentials`. Zero when ESC supplies none |
| `mapping` | Sorted `name=variable` pairs for the offer key |

A plan is usable when `variables` is not empty and the set is valid. Only the
pool builds one. `NewClient` never does.

## Resolution

`launchInputs.stateFor(info)` returns a `launchState` (environment, settings,
`channel`, `unproven`):

| Pool plan usable | Info advertises support | Result |
| --- | --- | --- |
| yes | yes | ESC offer without the declared variables, `channel` true |
| any other | any other | full ESC offer, `channel` false |

## `poolKey.channel`

True only on final keys of processes launched without the values whose running
binary advertises support.

## `poolEntry`

Adds `state`, the running process's `launchState`, used to build each holder's
digest and connection.

## `advertisement` (per binary)

`settings` and `credentials` (advertises support). `Pool.unproven` separately
holds the binaries whose process did not announce its port, which stay isolated
even when the replacement reported no information.
