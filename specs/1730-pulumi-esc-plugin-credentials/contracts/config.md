# Contract: `esc` Configuration Section

## Shape (`config.hujson`, project or global tier)

```jsonc
{
  "esc": {
    // Replaces the stack's imports for every plugin. [] turns ESC off.
    "environments": ["billing/aws-readonly@stable"],
    "plugins": {
      "vantage": {
        // Replaces esc.environments and stack imports for this plugin.
        "environments": ["billing/vantage"],
        // Extra names this plugin may receive from ESC.
        "variables": ["VANTAGE_TOKEN"]
      }
    }
  }
}
```

A project `esc` section replaces the global `esc` section as a whole, like
every other top-level section.

## Keys

| Key | Kind | `config get` / `config set` |
| --- | --- | --- |
| `esc.environments` | String list (`KindStringList`) | yes |
| `esc.plugins.<name>.environments` | String list (dynamic, like `plugins.<name>.<key>`) | yes |
| `esc.plugins.<name>.variables` | String list (dynamic) | yes |

- `config set` takes a comma-separated list, as for `scoring.field_allowlist`
  (`encodeStringList`). For example:
  `config set esc.environments 'billing/aws-readonly@stable,shared/tags'`.
- `config set esc.environments ''` writes an explicit empty list, which
  disables ESC (FR-003). There is no unset command. To return to "not set",
  delete the key from the file or on the Config page.
- No value under `esc` is secret. References and variable names are shown
  plainly by `config get` and the Config page.

## Validation (`config validate`, every `cost` pre-run, `ValidateConfigSource`)

| Problem | Message | Severity |
| --- | --- | --- |
| Malformed reference | `esc.environments[0]: invalid ESC environment reference "a//b": <reason>` | Error, exit 2 |
| Bad variable name | `esc.plugins.vantage.variables[1]: invalid environment variable name "1BAD"` | Error, exit 2 |
| Reserved variable | `esc.plugins.vantage.variables[0]: FINFOCUS_PLUGIN_PORT is reserved for plugin startup` | Error, exit 2 |
| Plugin name not installed or not in the catalog | `esc.plugins.foo: no plugin named "foo" is installed` | Warning |
| Wrong JSON type | The existing type-mismatch message | Error, exit 2 |

Validation never contacts Pulumi Cloud. Whether an environment exists is a
runtime fact (FR-009), not a configuration error.
