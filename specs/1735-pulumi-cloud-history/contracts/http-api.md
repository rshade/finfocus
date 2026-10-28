# Contract: Web Hooks

The feature adds no endpoint and no `internal/webui` handler. Its web surface
is one row on the existing Config page, which the existing handlers serve
from the shared setting list. The Cost page reads the history store as it does
today. Endpoint shapes are those of
`specs/628-web-config-page/contracts/http.md`.

## Hooks

| Endpoint | Handler | Go method the hook calls | Same method on the CLI |
| --- | --- | --- | --- |
| `GET /api/config` | `webui.(*Server).handleConfigGet` | `config.BuildPage` (through `SessionOptions.ConfigView`, wired in `cli.wireConfigEditor`) | `config get` reads the same `config.KnownSettings` entry |
| `POST /api/config/preview` | `webui.(*Server).handleConfigPreview` | `config.PatchFile` with `dryRun` true (through `SessionOptions.ConfigPreview`) | `config set` calls `config.PatchFile` |
| `POST /api/config` | `webui.(*Server).handleConfigSave` | `config.PatchFile` (through `SessionOptions.ConfigSave`) | `config set` calls `config.PatchFile` |
| Cost page data | existing web cost callbacks in `internal/cli/web_data.go` | `cli.enrichWithHistoricalResources` (unchanged) | `cost actual` calls `cli.enrichWithHistoricalResources` after `cloudhistory.Import` |

No hook calls `cloudhistory.Import` or any `pulumicloud` function.
`internal/webui` imports neither package (task T052).

## Settings row

The row is the `config.KnownSettings` entry added by task T038:

```json
{
  "key": "cost.history.pulumi_cloud.enabled",
  "kind": "enum",
  "help": "Import resource identity from Pulumi Cloud update history",
  "secret": false,
  "readOnly": false,
  "enum": ["false", "auto", "true"],
  "effective": { "source": "default", "value": "false" }
}
```

- The default tier value is `"false"` (`config.defaultValue`).
- A file that holds the boolean `false` or `true` shows as `"false"` or
  `"true"`, so the enum control always has a matching option.
- A save writes the string form. `PulumiCloudMode` reads either form.
- A value outside the enum is refused by `config.PatchFile`, as for any other
  enum setting: 400 with code `invalid_request` when the change itself is
  rejected, or `validation` when the patched tier fails `validateTier`.
- The row carries no secret, so `RedactJSON` changes nothing.
