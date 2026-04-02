---
title: Environment Variables
description: Environment variables for FinFocus Core
layout: default
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

## E2E Testing

| Variable                  | Description                       |
| ------------------------- | --------------------------------- |
| `FINFOCUS_E2E_AWS_REGION` | Region for AWS E2E tests          |
| `FINFOCUS_E2E_TOLERANCE`  | Cost tolerance for E2E validation |
