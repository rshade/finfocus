# Contract: CLI Behavior

No HTTP surface: Web UI Fit is Not applicable (see spec and research R13).
These are the user-visible contracts tests pin.

## Name matching (routing, validate, `--adapter`)

Given aws-public installed as `aws-public`, reporting `finfocus-plugin-aws-public`,
with the catalog alias `finfocus-plugin-aws-public`:

| Input | `config validate` | `config routes test` | cost commands |
| --- | --- | --- | --- |
| `name: aws-public` | valid | rule applied | rule applied |
| `name: finfocus-plugin-aws-public` | valid, retiring-name warning | rule applied, retiring-name warning | rule applied, no warning |
| `name: aws-publik` | error `plugin not found` | warning, entry not simulated | warning logged, entry not applied |
| `--adapter aws-public` | n/a | n/a | aws-public results only; only aws-public started |
| `--adapter finfocus-plugin-aws-public` | n/a | n/a | aws-public results only; only aws-public started (alias) |

After aws-public reports `aws-public` (Phase 4), every row above is unchanged
except the warning wording (retiring-name becomes former-name): the old
spelling resolves through the catalog alias instead of the reported name.

A plugin with no catalog alias whose reported name differs from its install
name (for example a URL install):

| Input | `config validate` | `config routes test` | cost commands |
| --- | --- | --- | --- |
| `name: <reported>` | valid | warning, entry not simulated (points to `config validate`) | rule applied |
| `--adapter <reported>` | n/a | n/a | that plugin's results; every plugin started |
| `--adapter aws-publik` | n/a | n/a | no results; every plugin started; warning that no loaded plugin answers to `aws-publik` |
| `name: finfocus-plugin-<x>`, `<x>` installed, no match | error `plugin not found (did you mean "<x>"?)` | warning with the same hint | warning logged, entry not applied |

Warning texts:

- Retiring name (alias that is still the plugin's reported name):
  `"finfocus-plugin-aws-public" will be retired; use "aws-public"`
- Former name (alias the plugin no longer reports):
  `"finfocus-plugin-aws-public" is a former name of "aws-public"; use "aws-public"`

## Run-time warnings (log, level warn)

| Event | Message fields |
| --- | --- |
| `--adapter` value matches no loaded plugin (each run) | `component=cli`, `adapter`, message: `no loaded plugin answers to the --adapter value` |
| Plugin reported name differs from install name (once per process) | `component=registry`, `install_name`, `reported_name`, message: `plugin reports a name that differs from its install name; both are accepted` |
| Routing entry matched no loaded plugin and names no installed plugin (each run) | `component=cli`, `operation=create_router`, `plugin`, message: `routing entry matches no loaded plugin; its feature and pattern restrictions are not applied`. An entry naming an installed plugin that was not loaded (`--adapter` narrowed the set, or it failed to start, which is logged there) is not warned about |

## `finfocus config routes test`

Table output, after the existing sections, only when warnings exist:

```text
Warnings:
  aws-publik: does not match an installed plugin; not simulated (run 'finfocus config validate' to check reported plugin names)
  finfocus-plugin-aws-public: "finfocus-plugin-aws-public" will be retired; use "aws-public"
```

JSON adds `"warnings": [{"plugin": "...", "message": "..."}]`, omitted when empty.

## `finfocus plugin conformance`

New flag: `--expected-name <name>` (string, default empty).

New protocol tests:

| ID | Pass | Warn |
| --- | --- | --- |
| `Name_MatchesInstallName` | reported == expected | reported != expected; or expected name unresolved (`install name not checked: ...`, says to pass `--expected-name`) |
| `Name_MatchesPluginInfo` | `Name` == `GetPluginInfo().name` | they differ; or `GetPluginInfo` unimplemented or failing |
| `Name_IsCanonicalForm` | name is canonical | not canonical (reason given) |

None of them skips or fails: certification treats a skip as unverified, and
these checks are advisory. A failing `Name` call makes each warn that it did
not run (`Name_ReturnsPluginIdentifier` still fails for it).

Report formats:

- Table: `⚠ <ID>` with the message on the next line; `Warnings: N` printed
  only when N > 0.
- JSON: result `"status": "warn"`; `"summary": {"warnings": N, ...}`.
- JUnit: passing `<testcase>` with `<system-out>WARNING: <message></system-out>`.

Exit code: unchanged by warnings. A run whose only non-pass results are
warnings exits 0.

## `finfocus plugin certify`

Resolves the expected name from the binary path only. Warnings do not block
certification; the markdown report adds `- Warnings: N`.

## Plugin catalog (`internal/registry/registry.json`)

Entries accept `"aliases": ["<former name>", ...]`. Validation fails when an
alias is not a valid plugin name, repeats its own entry's name, or equals any
entry name or alias. The `plugin list` table and the dashboard catalog do not
show aliases; `plugin list --available --output json`, which prints catalog
entries as they are, includes the field like `deprecated` and `replacement`.

## `finfocus plugin init`

The generated Go source declares `const PluginName = "<name>"` and uses it for
`NewBasePlugin` and the plugin info name.
