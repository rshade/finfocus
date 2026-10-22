# Quickstart: Verify the Analyzer Language Plugin

Run from the repository root. No cloud credentials are needed; the project has
no resources and uses a local state backend.

## 1. Break check on current `main` (before the change)

```bash
FINFOCUS_TEST_PULUMI=<absolute path to pulumi 3.271.0> \
  go test -count=1 -run TestAnalyzerPolicyPack ./test/integration/ -v
```

Expected: FAIL with `no language plugin 'pulumi-language-finfocus' found`.
With Pulumi 3.268.0 the same command passes.

Pulumi 3.271.0 can be unpacked anywhere outside `/tmp` if `/tmp` is small:

```bash
mkdir -p bin/pulumi-3.271.0
curl -fsSL https://get.pulumi.com/releases/sdk/pulumi-v3.271.0-linux-x64.tar.gz \
  | tar xz -C bin/pulumi-3.271.0
```

`FINFOCUS_TEST_PULUMI` must be the real binary, not a mise shim.

## 2. After the change

```bash
for p in "$HOME/.pulumi/bin/pulumi" "$PWD/bin/pulumi-3.271.0/pulumi/pulumi"; do
  FINFOCUS_TEST_PULUMI="$p" \
    go test -count=1 -run TestAnalyzerPolicyPack ./test/integration/ -v
done
```

Expected on both: `PASS` for the `language_plugin` (including the stack and
project in `last-cost-summary.json`), `default_acquisition` (needs network),
and `plugin_cleanup` subtests. `legacy_entry_only` runs on 3.268.0 and is
skipped on 3.270.0 and later. CI runs the same test on Ubuntu and macOS.

## 3. Unit and package tests

```bash
go test -race ./internal/analyzer/... ./internal/cli/ -run 'Analyzer|PolicyPack|LanguageHost'
make test
make lint
```

## 4. Manual check with a real project

```bash
make build
./bin/finfocus analyzer install
export PATH="$HOME/.finfocus/analyzer:$PATH"
ls -l "$HOME/.finfocus/analyzer"   # PulumiPolicy.yaml and both entries
cd ../finfocus-demo
PULUMI_CONFIG_PASSPHRASE='' pulumi preview --policy-pack "$HOME/.finfocus/analyzer"
```

Expected: cost diagnostics and the stack summary, the same as on Pulumi
3.268.0 before the change. Afterwards check `ps` for leftover
`finfocus-plugin-*` or `pulumi-analyzer-policy-finfocus` processes.

## 5. Windows

The nightly `Tests (windows-latest)` job runs `go test ./internal/...`
natively, which covers the `.exe` entry names. One manual preview on the
newest Pulumi is recorded in the PR (tasks T032): `finfocus analyzer install`,
the PowerShell `PATH` line it prints, then `pulumi preview --policy-pack`
against a project with no resources. Graceful cost plugin cleanup is not
promised on Windows (spec FR-007, SC-005).
