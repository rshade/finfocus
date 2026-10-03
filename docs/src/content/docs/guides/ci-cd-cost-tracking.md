---
title: CI/CD Cost Tracking
description: Record a cost history snapshot after every Pulumi deploy
---

Run `finfocus cost history collect` as a post-deploy step so each successful
`pulumi up` adds one projected-cost snapshot. Later `view`, `diff`, and
`list` commands read that database and do not call plugins.

`collect` still needs the Pulumi CLI, stack credentials, and a cost plugin.
`--versions` does not remove those requirements.

## `--versions 1`

`--versions` defaults to `0`, which means no limit. A collect with the
default stores every successful `update` and `destroy` that is not already
in the database.

`--versions 1` keeps only the newest successful checkpoint, then skips it
when that version is already stored. Use it after a deploy so the job
records the update that just finished. A retry that finds the same version
prints `No new versions to collect.` and does not call the pricer again.

```bash
finfocus cost history collect --stack dev --versions 1
```

The parent `cost --stack` flag is required. History subcommands do not
declare a second copy.

## Where the database lives

Each stack has one file, `<stack>.history.db`. Slashes in the stack name
become dashes. The default directory is `~/.finfocus/history/`.

`FINFOCUS_HOME` overrides that root. Plugins and config use the same
directory, so a CI job that sets `FINFOCUS_HOME` must install plugins into
that home. A plugin installed only under the runner's real `~/.finfocus`
is invisible to the collect step.

Hosted runners delete the home directory when the job ends. Upload or
cache the `history/` directory if the next deploy should continue the same
timeline. A job that starts from an empty directory still records the
latest checkpoint, and it has no older snapshots to chart.

## GitHub Actions

`pulumi/actions@v7` installs the Pulumi CLI when `command` is omitted.
FinFocus releases are versioned archives such as
`finfocus-v0.4.0-linux-amd64.tar.gz`. The install script picks the archive
for the runner and checks its checksum. There is no raw
`finfocus-linux-amd64` asset on the release.

`actions/cache` restores the newest `history/` directory from earlier runs
on this branch. The exact `run_id` key never matches, so `restore-keys`
does the restore. The upload step keeps a copy you can download from that
run.

```yaml
name: Deploy Infrastructure

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    env:
      FINFOCUS_HOME: ${{ github.workspace }}/.finfocus
    steps:
      - uses: actions/checkout@v4

      - name: Restore cost history
        uses: actions/cache@v4
        with:
          path: .finfocus/history
          key: cost-history-${{ github.ref_name }}-${{ github.run_id }}
          restore-keys: |
            cost-history-${{ github.ref_name }}-

      - name: Setup Pulumi
        uses: pulumi/actions@v7

      - name: Deploy infrastructure
        run: pulumi up -y --stack dev
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}

      - name: Install finfocus
        run: |
          curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh

      - name: Record cost snapshot
        run: finfocus cost history collect --stack dev --versions 1
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}

      - name: Upload cost history
        uses: actions/upload-artifact@v4
        with:
          name: cost-history
          path: .finfocus/history
```

Install a cost plugin into `$FINFOCUS_HOME` before the collect step, and
give the job the cloud credentials that plugin expects. Those steps depend
on the plugin, so they are not pinned here.

## GitLab CI

GitLab cache paths are relative to the project directory. Set
`FINFOCUS_HOME` inside the workspace, then cache `.finfocus/history/`.
`$CI_ENVIRONMENT_NAME` is the stack name when the environment matches the
Pulumi stack. The job image needs `pulumi`, `finfocus`, and a cost plugin
on `PATH`.

```yaml
deploy:
  stage: deploy
  variables:
    FINFOCUS_HOME: $CI_PROJECT_DIR/.finfocus
  script:
    - pulumi up -y --stack "$CI_ENVIRONMENT_NAME"
    - finfocus cost history collect --stack "$CI_ENVIRONMENT_NAME" --versions 1
  cache:
    key: cost-history-$CI_ENVIRONMENT_NAME
    paths:
      - .finfocus/history/
  artifacts:
    paths:
      - .finfocus/history/
```

The cache is what the next pipeline restores. The artifact is the copy you
can download from this pipeline.

## Shell wrapper

```bash
#!/bin/bash
# Deploy one stack and record only the checkpoint that just succeeded.
set -euo pipefail

STACK="${1:?Usage: deploy-and-track.sh <stack-name>}"

echo "Deploying stack: $STACK"
pulumi up -y --stack "$STACK"

echo "Recording cost snapshot..."
finfocus cost history collect --stack "$STACK" --versions 1

echo "Done. View cost history with:"
echo "  finfocus cost history view --stack $STACK"
```

## Pulumi Automation API

Call collect only after `Up` reports `succeeded`. `SelectStackLocalSource`
selects a stack from a local program directory. `SelectStack` takes a
`Workspace`, not a directory string.

```go
stack, err := auto.SelectStackLocalSource(ctx, "dev", workDir)
if err != nil {
    return err
}
result, err := stack.Up(ctx)
if err != nil {
    return err
}
if result.Summary.Result != "succeeded" {
    return fmt.Errorf("pulumi up result %q", result.Summary.Result)
}
cmd := exec.Command("finfocus", "cost", "history", "collect",
    "--stack", "dev", "--versions", "1")
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
    return fmt.Errorf("record cost snapshot: %w", err)
}
```

The program still needs `context`, `fmt`, `os`, `os/exec`, and
`github.com/pulumi/pulumi/sdk/v3/go/auto` imported. `finfocus` must be on
`PATH` for the process that runs the hook.
