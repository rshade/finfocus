# Quickstart: Verify ESC Credentials for Plugins

These checks exercise each user story against the real binary. Unit and
integration tests are in `tasks.md`. Use a temporary home so nothing touches
your real `~/.finfocus`.

```bash
make build
export FINFOCUS_HOME="$(mktemp -d)"
```

## 1. A stack import reaches the plugin (User Story 1)

Prerequisites:

- A Pulumi Cloud login (`pulumi login`, or `PULUMI_ACCESS_TOKEN`).
- An ESC environment, for example `finfocus-test/recorder`, with this
  definition:

  ```yaml
  values:
    environmentVariables:
      FINFOCUS_RECORDER_MOCK_RESPONSE: "true"
      UNRELATED_SECRET:
        fn::secret: do-not-deliver
  ```

```bash
make install-recorder   # or the plugin under test
cd ../finfocus-demo
pulumi config env add finfocus-test/recorder --yes   # rewrites Pulumi.dev.yaml; never commit it
unset FINFOCUS_RECORDER_MOCK_RESPONSE
# Allow the variable in case the recorder does not advertise it as a setting
../finfocus/bin/finfocus config set esc.plugins.recorder.variables FINFOCUS_RECORDER_MOCK_RESPONSE
PULUMI_CONFIG_PASSPHRASE='' ../finfocus/bin/finfocus cost projected --debug 2>&1 \
  | grep -E 'resolve_settings|esc'
```

Expected:

- The debug line lists `FINFOCUS_RECORDER_MOCK_RESPONSE` under `esc_env`, with
  `esc_refs` showing `finfocus-test/recorder`.
- `UNRELATED_SECRET` appears nowhere.
- No value appears in the output.

Restore the stack file afterwards: `git -C ../finfocus-demo checkout Pulumi.dev.yaml`.

## 2. ESC beats the shell (User Story 1, scenario 3)

```bash
FINFOCUS_RECORDER_MOCK_RESPONSE=false PULUMI_CONFIG_PASSPHRASE='' \
  ../finfocus/bin/finfocus cost projected --debug 2>&1 | grep resolve_settings
```

Expected: the recorder behaves as with `true`, because the ESC value won.

## 3. Fallback (User Story 2)

```bash
FINFOCUS_HOME="$FINFOCUS_HOME" ../finfocus/bin/finfocus config set esc.environments 'finfocus-test/does-not-exist'
PULUMI_CONFIG_PASSPHRASE='' ../finfocus/bin/finfocus cost projected; echo "exit=$?"
```

Expected:

- One warning: `ESC environment finfocus-test/does-not-exist skipped (not_found)`.
- `exit=0`.
- Results come from shell and config values.

Then try:

- `PULUMI_ACCESS_TOKEN= PULUMI_CREDENTIALS_PATH="$(mktemp -d)"`. This
  produces one `not_logged_in` warning that names `PULUMI_ACCESS_TOKEN`.
- `PULUMI_BACKEND_URL=file://$(mktemp -d)`. This produces one
  `unsupported_backend` warning.
- `config set esc.environments ''`. This produces no warning and no ESC
  traffic (FR-003).

## 4. Unreachable ESC stays bounded (SC-004)

```bash
time PULUMI_BACKEND_URL=https://10.255.255.1 PULUMI_ACCESS_TOKEN=x \
  ../finfocus/bin/finfocus cost projected
```

Expected: a single `timed_out` warning, and the run is under 6 seconds
longer than it is with ESC disabled.

## 5. Web settings show the source (User Story 5)

```bash
PULUMI_CONFIG_PASSPHRASE='' ../finfocus/bin/finfocus --web --no-browser --port 18765
```

Open Plugins, then the recorder's Settings. The setting that ESC supplies
reports `effectiveSource: "esc"` and `escEnvironment`, which you can check in
the network panel or by reading `GET /api/plugins/recorder/settings`. Its help
text says ESC overrides the shell and the files. No value is shown.

Stop the server with:

```bash
pkill -f '^<repo>/bin/finfocus --web'
```

Then check `ps` for orphaned plugin processes.

## 6. Redaction (SC-005)

Repeat step 1 with `--debug --format json`, capture all output, and search it
and the FinFocus home (cache, logs) for the fixture secret:

```bash
OUT="$(mktemp)"
PULUMI_CONFIG_PASSPHRASE='' ../finfocus/bin/finfocus cost projected --debug --format json >"$OUT" 2>&1
grep -r 'do-not-deliver' "$OUT" "$FINFOCUS_HOME"; echo "matches=$?"
```

Expected: `matches=1`, which means no match.

## Cleanup

Remove the environment from the demo stack (`git checkout Pulumi.dev.yaml`),
`unset FINFOCUS_HOME`, and delete the test ESC environment if you created one.
