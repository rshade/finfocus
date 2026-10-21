---
title: Pulumi ESC credentials
description: Give plugins credentials from the ESC environments a Pulumi stack imports, or from an explicit list.
---

## Overview

FinFocus can pass plugin process environment variables from Pulumi ESC. The
usual setup is the `environment:` imports already on the stack. You can also
name environments in `config.hujson`. A plugin receives a variable only when
it advertises that name, or when you list the name for that plugin.

ESC is the highest-precedence source. An allowed ESC value wins over the
shell, the project file, the global file, and the plugin's default. That is
a change from earlier releases: exporting a variable in the shell does not
hide the value ESC supplies.

FinFocus opens Pulumi Cloud only. It does not write Pulumi's credential
files, and a value from ESC is never printed, logged, or written into the
cost cache.

## Setup

Sign in to Pulumi Cloud with `pulumi login`, or set `PULUMI_ACCESS_TOKEN`.
A login stored only in the system keychain has no token finfocus can read, so
set `PULUMI_ACCESS_TOKEN` in that case.

Put the variables the plugin should see under `environmentVariables` in the
ESC environment. Then either import that environment from the stack:

```yaml
# Pulumi.dev.yaml
environment:
  - billing/aws-readonly
```

or name it in finfocus configuration:

```bash
finfocus config set esc.environments 'billing/aws-readonly@stable,shared/tags'
```

`esc.plugins.<name>.environments` replaces that list for one plugin. An empty
list turns ESC off:

```bash
finfocus config set esc.environments ''
```

The empty string writes `[]`. Delete the key when you want the stack imports
again. A project `esc` section replaces the global `esc` section as a whole.
The keys and the validation messages are in the
[configuration reference](../reference/config-reference.md).

### Which variables a plugin receives

The plugin's advertised settings are the default allowlist. To pass an extra
name, such as a credential the plugin reads without advertising it, list it
for that plugin only:

```bash
finfocus config set esc.plugins.vantage.variables 'VANTAGE_TOKEN'
```

Another plugin that opens the same environment does not receive
`VANTAGE_TOKEN`. Names reserved for plugin startup (`FINFOCUS_PLUGIN_PORT`,
the log-file variable, and the trace id) are rejected and are never taken
from ESC.

### Self-hosted Pulumi Cloud

`PULUMI_ACCESS_TOKEN` is sent only to a backend selected by
`PULUMI_BACKEND_URL` or by `pulumi login`. A `backend.url` in `Pulumi.yaml`
does not receive that token. For a self-hosted Pulumi Cloud API, set
`PULUMI_BACKEND_URL` to that API or run `pulumi login` against it. DIY
backends (`file://`, `s3://`, `gs://`, `azblob://`) and non-loopback
`http://` cannot host ESC.

References look like `[[org/]project/]name[@version]`. One segment is read as
`default/<name>`. When the reference has no organization, finfocus uses the
organization in a qualified stack name, then `PULUMI_DEFAULT_ORGANIZATION`,
then the organization saved for the backend.

## When ESC is unavailable

Every failure continues the command. Plugins use the shell and the config
files. Stderr shows one warning per environment and category. The warning
names the reference and the category, never a value. The categories, causes,
and fixes are in
[ESC environment skipped](troubleshooting.md#esc-environment-skipped).

Inline `environment:` definitions, including a `values:` block, are not read.
Import an environment, or set `esc.environments`.

## Cache

Cost answers are cached by the environment reference and its revision, not by
the variable values. A rotated credential at the same revision still hits the
cache. Editing the environment changes the revision and misses the cache.

A value edited in an environment that your environment imports does not
change your environment's revision. That answer can be reused until the cache
TTL expires.

## Analyzer

`finfocus analyzer serve` does not open the stack's ESC imports. Pulumi has
already placed those variables in the process environment, and the stack is
not known when the analyzer starts. Explicit `esc` configuration is still
opened.

## Dashboard

Plugin settings in `finfocus --web` show an ESC-supplied row as
`from ESC (<reference>)` in the Status column. The reference is the
environment name, never the variable value. The row's help text names that
environment and says Pulumi ESC overrides the shell and both configuration
files. A required setting that ESC supplies counts as set.
