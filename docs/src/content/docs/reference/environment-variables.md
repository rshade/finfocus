---
title: Environment Variables
description: Environment variables for FinFocus Core
---

FinFocus supports configuration via environment variables.

## Global

| Variable               | Description                              | Default                   |
| ---------------------- | ---------------------------------------- | ------------------------- |
| `FINFOCUS_LOG_LEVEL`   | Log verbosity (debug, info, warn, error) | info                      |
| `FINFOCUS_CONFIG_FILE` | Path to configuration file               | `~/.finfocus/config.yaml` |
| `FINFOCUS_PLUGIN_DIR`  | Directory for plugins                    | `~/.finfocus/plugins`     |

## Plugins

| Variable                         | Description                   |
| -------------------------------- | ----------------------------- |
| `AWS_ACCESS_KEY_ID`              | AWS Access Key for AWS plugin |
| `AWS_SECRET_ACCESS_KEY`          | AWS Secret Key for AWS plugin |
| `AZURE_CLIENT_ID`                | Azure Client ID               |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to GCP credentials       |

## Pulumi

Finfocus reads these when it opens Pulumi ESC for plugin credentials. It does
not write Pulumi's credential files.

| Variable | Why finfocus reads it |
| --- | --- |
| `PULUMI_ACCESS_TOKEN` | Pulumi Cloud token. Wins over a token saved in `credentials.json`, except for a backend chosen by `backend.url` in `Pulumi.yaml`, which uses only its saved token. A keychain-encrypted login has nothing finfocus can read, so set this. |
| `PULUMI_BACKEND_URL` | Pulumi Cloud API to open. Wins over `backend.url` in `Pulumi.yaml` and over `credentials.json`. |
| `PULUMI_HOME` | Directory of `credentials.json` and the workspace file that names the selected stack. |
| `PULUMI_CREDENTIALS_PATH` | Alternate directory that holds `credentials.json`. Wins over `PULUMI_HOME`. |
| `PULUMI_DEFAULT_ORGANIZATION` | Organization for a reference that does not include one, when the stack name does not supply it either. |

A `backend.url` in `Pulumi.yaml` does not receive `PULUMI_ACCESS_TOKEN`. For a
self-hosted Pulumi Cloud backend, set `PULUMI_BACKEND_URL` or run
`pulumi login` so the saved token matches that URL.

## E2E Testing

| Variable                  | Description                       |
| ------------------------- | --------------------------------- |
| `FINFOCUS_E2E_AWS_REGION` | Region for AWS E2E tests          |
| `FINFOCUS_E2E_TOLERANCE`  | Cost tolerance for E2E validation |
