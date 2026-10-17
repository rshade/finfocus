# Upgrade to finfocus-spec v0.7.6: advertised plugin settings

v0.7.6 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for most plugins.

## What changed

- `GetPluginInfoResponse` gains `settings`: a list of `PluginSetting`
  descriptors, one per value the plugin reads from its process environment.
  Each entry names the environment variable (`env`) the host injects, so the
  host can show and set it. The SDK never reads the environment itself, and
  the manifest `configuration` object is not the delivery channel.
- `pluginsdk.WithSettings(...)` sets the list on the `PluginInfo` a plugin
  returns. Declaration order is the response order, and each entry is cloned.
- A malformed list is rejected with `ErrInvalidPluginSettings`, and the
  conformance suite checks the same rules.

## Plugins that read environment variables: declare them

A plugin with no settings needs no change. A plugin that reads variables such
as an API key or a timeout can describe them:

```go
pluginsdk.WithSettings(
    &pbc.PluginSetting{
        Key:      "api_key",
        Type:     pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
        Env:      "MYPLUGIN_API_KEY",
        Required: true,
        Secret:   true,
    },
)
```

The descriptor rules:

- `key` is 1-64 bytes matching `[a-z][a-z0-9_]*` and is unique in the list.
- `env` is 1-128 bytes matching `[A-Za-z_][A-Za-z0-9_]*` and is unique in the
  list.
- `type` must be set. An `ENUM` needs 1-32 unique `enum_values`; every other
  type leaves them empty.
- An empty `default_value` means no default, for every type.
- `secret` is advisory: the host still applies its own credential rules.

Run your conformance tests after adding settings. A list that breaks a rule is
rejected at runtime, not only in tests.
