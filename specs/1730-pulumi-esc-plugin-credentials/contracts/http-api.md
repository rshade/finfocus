# Contract: Web Hook for ESC Setting Sources (Principle VII)

No new endpoint. One existing endpoint gains fields. The middleware, session,
and redaction rules are those of `specs/629-web-plugin-page/contracts/http.md`.

## GET /api/plugins/{name}/settings

| Layer | Go method |
| --- | --- |
| Handler | `webui.(*Server).handlePluginSettingsGet` (existing) |
| Callback | `webui.SessionOptions.PluginSettings`, wired in `cli.wirePlugins` (`internal/cli/web_plugins.go`) |
| CLI method | `cli.pluginSettingsView` → `cli.loadPluginSettingState` → `cli.resolvePluginDelivery` → `cli.pluginSettingValues` |
| ESC resolution | `cli.resolvePluginDelivery` calls `esc.(*Source).Delivery` and then `pluginhost.ResolveDeliveredEnv`. That is the same resolution `pluginhost.NewClient` uses at launch. `cli.pluginSettingValues` reads `Delivered.Origins` and does not open ESC itself. The `Source` is the session's current one (`webData.escSource`) |

The web code computes no ESC fact itself. It reads `Delivered.Origins` from
the shared resolution.

### Response additions (`webui.PluginSettingValue`)

```json
{
  "key": "api_key",
  "secret": true,
  "project": { "set": false },
  "global": { "set": true },
  "envSet": true,
  "effectiveSource": "esc",
  "escEnvironment": "billing/vantage@stable"
}
```

- `effectiveSource` values: `esc` | `env` | `project` | `global` | `default` | `""`.
- `escEnvironment` is present only when `effectiveSource` is `esc`. It is the
  canonical reference and never a value.
- The `secret` rules are unchanged: no `value` property for secrets, whatever
  the source.
- When ESC failed for this session, no row reports `esc` (User Story 5,
  scenario 2).

## POST /api/plugins/{name}/settings/preview and POST /api/plugins/{name}/settings

| Layer | Go method |
| --- | --- |
| Handlers | `webui.(*Server).handlePluginSettingsPreview` and `handlePluginSettingsSave` → `handlePluginSettingsWrite` (existing) |
| Callback | `webui.SessionOptions.PluginSettingsPrepare`, wired in `cli.wirePlugins` |
| CLI method | `cli.preparePluginSettings` → `cli.requirePluginSettings` |

A required setting that ESC supplies counts as satisfied, as a set shell
variable does today. There are no other changes.

## Config page

There are no handler changes. `esc.*` keys are listed in
`config.KnownSettings`, so the existing `config.PatchFile` path accepts them.

## Follow-up (`web-ui` label, UX only)

Implemented in `internal/webui/static/views/plugins.js` (#1867). An
`effectiveSource: "esc"` row's Status cell reads `from ESC (<reference>)`.
The help paragraph names that reference in a code element. The settings
response is unchanged.
