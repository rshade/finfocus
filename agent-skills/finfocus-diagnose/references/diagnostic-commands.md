# Diagnostic commands and environment

The config file is `config.hujson`.
A legacy `config.yaml` is migrated on read.
`finfocus config list` prints the loaded configuration. There is no `finfocus config path`.

## Commands that exist

| Command | What it does |
| --- | --- |
| `finfocus plugin list` | Installed plugins. `--verbose` usage is `Show detailed plugin information` (default `false`). `--output json` is accepted. |
| `finfocus plugin validate` | Checks every installed plugin. `--plugin` limits the check to one name (default empty, which means all). |
| `finfocus config routes list` | Effective routing rules. Prints `No routing configured (automatic mode)` when `routing` is absent. |
| `finfocus config routes test <resource-type> [region]` | Simulates per-feature selection. Does not contact plugins. |
| `finfocus cost projected --pulumi-json plan.json` | Projected cost. `--debug` raises the log level for this process. |
| `finfocus cost actual --fallback-estimate` | Includes a `$0` placeholder when no plugin returned actual cost. Without the flag, that resource is omitted. |

`plugin validate` checks the binary before it talks to the plugin:

- missing file: `plugin binary not found:`
- path is a directory: `plugin path is a directory, not a binary`
- Unix mode has no execute bit (`0111`): `plugin binary is not executable`
- Windows: `plugin binary is not executable (Windows requires .exe extension)`
- optional `plugin.manifest.json`: `manifest name mismatch: expected demo, got other` and `manifest version mismatch: expected 1.0.0, got 9.9.9`
- one or more failures: `one or more plugins failed validation`

## Where config is loaded

Global directory (`ResolveConfigDir`), first match wins:

1. `FINFOCUS_HOME`
2. `$PULUMI_HOME/finfocus`
3. `~/.finfocus`
4. `./.finfocus` when the home directory cannot be determined

Project directory (`ResolveProjectDir`), first match wins. The result is that directory plus `.finfocus` unless it already ends in `.finfocus`:

1. `--project-dir`
2. `FINFOCUS_PROJECT_DIR`
3. Walk up from the working directory to `Pulumi.yaml`, then `$PROJECT/.finfocus`
4. Empty when no project is found, and the global config is used alone

Project `config.hujson` is shallow-merged onto the global file. A key that is present replaces that entire top-level section (`output`, `plugins`, `logging`, `analyzer`, `plugin_host`, `cost`, `routing`, `scoring`). A key that is absent keeps the global value. Nested keys are not merged field by field. A project `cost` block replaces global budgets, cache, and the rest of `cost` together.

`FINFOCUS_CONFIG_STRICT` is `true` or `1`.
In that mode a permission error or a file that cannot be parsed panics.
A missing file does not panic. Any other value warns and keeps defaults.
The strict parse error is `configuration file appears corrupted`.

`plugin_host.strict_compatibility` defaults to false. When it is true, a major spec mismatch returns `plugin spec version incompatible with core`. Otherwise the host logs a warning and still loads the plugin.

`FINFOCUS_ANALYZER_MODE` is read only when the value is `true`.

## Environment variables the process reads

| Variable | Effect |
| --- | --- |
| `FINFOCUS_HOME` | Global config directory. |
| `FINFOCUS_PROJECT_DIR` | Project directory. `.finfocus` is appended unless the path already ends with it. |
| `FINFOCUS_CONFIG_STRICT` | `true` or `1` panics on permission errors and unparsable config. |
| `FINFOCUS_OUTPUT_FORMAT` | Sets `output.default_format` when non-empty. |
| `FINFOCUS_OUTPUT_PRECISION` | Integer precision. A non-integer is ignored. |
| `FINFOCUS_COMPAT` | `1` or `true` also reads legacy `PULUMICOST_*` variables. |
| `FINFOCUS_ENFORCEMENT` | Sets `analyzer.enforcement` when non-empty. |
| `FINFOCUS_MAX_MONTHLY_COST` | Sets `analyzer.max_monthly_cost` when it parses as a float. A non-number is ignored. |
| `FINFOCUS_PLUGIN_DIR` | Plugin directory. This override wins over the directory computed from the config home. |
| `FINFOCUS_ANALYZER_MODE` | Read only when the value is `true`. `analyzer serve` sets it. With no log file, plugin stdout and stderr are discarded. A configured log file still receives them. |
| `FINFOCUS_LOG_LEVEL` | Log level when `--debug` is not set. `--debug` forces `debug`. |
| `FINFOCUS_LOG_FORMAT` | Log format when non-empty. Config validation accepts `json` and `text`. |
| `FINFOCUS_LOG_FILE` | Log file path when non-empty. |
| `FINFOCUS_TRACE_ID` | Trace id. Priority is this variable, then the context, then a generated 32-character hex id. |
| `FINFOCUS_PLUGIN_PORT` | Set on the plugin process for debugging. The `--port` flag is the address the plugin must bind. |
| `FINFOCUS_CACHE_ENABLED` | `ParseBool`. Default enabled is true. An unparsable value is ignored. |
| `FINFOCUS_CACHE_TTL` | Default TTL seconds. Default is `3600`. |
| `FINFOCUS_CACHE_TTL_SECONDS` | Legacy alias, used only when `FINFOCUS_CACHE_TTL` is empty. |
| `FINFOCUS_CACHE_DIR` | Cache directory. Default is `<config dir>/cache`. |
| `FINFOCUS_CACHE_MAX_SIZE_MB` | Max cache size in MB. Default is `100`. An unparsable value is ignored. `0` means unlimited; the default is not `0`. |

`PORT` is not set on the plugin. When `PORT` is present in the parent environment, the host logs `PORT environment variable detected in parent environment (will be ignored, plugin uses --port flag)` at debug.

`CI` changes plugin startup only when it equals `true`. Other values, including a variable that is set but empty, keep the default bind timeout and retry count.

There is no environment variable that changes the engine per-resource timeout. `analyzer.timeout.per_resource` is stored on the config struct (default 5s in config) and no cost or analyzer server path reads it. Changing that field does not change a cost command's deadline.
