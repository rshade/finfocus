---
title: Environment Variables
description: Environment variables for FinFocus Core
---

FinFocus supports configuration via environment variables.

## Global

| Variable               | Description                              | Default                   |
| ---------------------- | ---------------------------------------- | ------------------------- |
| `FINFOCUS_LOG_LEVEL`   | Log verbosity (debug, info, warn, error) | info                      |
| `FINFOCUS_HOME`        | Directory that holds `config.hujson`, plugins, cache, and logs | `~/.finfocus` |
| `FINFOCUS_PLUGIN_DIR`  | Directory for plugins                    | `~/.finfocus/plugins`     |
| `FINFOCUS_WEB_LOGS`    | Show the Logs view in `finfocus --web` (`1`, `true`, `0`, `false`). Overrides `web.logs.enabled` | off |

## Plugins

Plugin processes inherit the environment FinFocus runs in. Each plugin reads
its own variables, listed on its docs page. The public pricing plugins
(`aws-public`, `azure-public`) need no credentials.

| Variable | Description |
| --- | --- |
| `AWS_PROFILE`, `AWS_REGION` | AWS credentials and region for [aws-ce](../plugins/aws-ce.md), through the AWS SDK default credential chain. Other standard AWS SDK variables also work. |
| `FINFOCUS_PLUGIN_<PLUGIN_NAME>_<KEY_NAME>` | Fallback for a `plugins.<name>.<key>` config key the plugin doesn't advertise, in uppercase with hyphens as underscores. An advertised key uses the variable the plugin names for it: aws-ce reads `carbon_enabled` from `FINFOCUS_AWS_CARBON_ENABLED` (see [Historical AWS Carbon Reporting](../plugins/historical-carbon.md)). |

## Pulumi

Finfocus reads these when it opens Pulumi ESC for plugin credentials. It doesn't
write Pulumi's credential files.

| Variable | Why finfocus reads it |
| --- | --- |
| `PULUMI_ACCESS_TOKEN` | Pulumi Cloud token. Wins over a token saved in `credentials.json`, except for a backend chosen by `backend.url` in `Pulumi.yaml`, which uses only its saved token. A keychain-encrypted login has nothing finfocus can read, so set this. |
| `PULUMI_BACKEND_URL` | Pulumi Cloud API to open. Wins over `backend.url` in `Pulumi.yaml` and over `credentials.json`. |
| `PULUMI_HOME` | Directory of `credentials.json` and the workspace file that names the selected stack. |
| `PULUMI_CREDENTIALS_PATH` | Alternate directory that holds `credentials.json`. Wins over `PULUMI_HOME`. |
| `PULUMI_DEFAULT_ORGANIZATION` | Organization for a reference that doesn't include one, when the stack name doesn't supply it either. |

A `backend.url` in `Pulumi.yaml` doesn't receive `PULUMI_ACCESS_TOKEN`. For a
self-hosted Pulumi Cloud backend, set `PULUMI_BACKEND_URL` or run
`pulumi login` so the saved token matches that URL.

## E2E Testing

| Variable                  | Description                       |
| ------------------------- | --------------------------------- |
| `FINFOCUS_E2E_AWS_REGION` | Region for AWS E2E tests          |
| `FINFOCUS_E2E_TOLERANCE`  | Cost tolerance for E2E validation |
