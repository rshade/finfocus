# Quickstart: Verify Plugin Name Conformance

Run from the repository root after `make build`. Use a scratch home so the
real `~/.finfocus` is untouched.

## 1. Reproduce #2278 and #2206 (before and after)

```bash
export FINFOCUS_HOME="$(mktemp -d)"
./bin/finfocus plugin install aws-public@v0.2.5
./bin/finfocus plugin install cloudzero
./bin/finfocus plugin list
```

Write a routing config into `$FINFOCUS_HOME/config.hujson`:

```json
{
  "routing": {
    "plugins": [
      {"name": "aws-public", "features": ["ProjectedCosts"]},
      {"name": "cloudzero", "features": ["ActualCosts"]}
    ]
  }
}
```

Then, with a Pulumi state holding one `aws:ec2/instance:Instance`:

```bash
./bin/finfocus config validate
./bin/finfocus config routes test aws:ec2/instance:Instance us-west-2
./bin/finfocus cost actual --pulumi-state state.json
./bin/finfocus cost actual --pulumi-state state.json --adapter aws-public
```

Expected after the change: `validate` passes, routes test shows aws-public for
ProjectedCosts only, `cost actual` does not use aws-public as the adapter, the
`--adapter aws-public` run returns aws-public rows, and one warning names
`aws-public` and `finfocus-plugin-aws-public`.

Repeat with `"name": "finfocus-plugin-aws-public"`; the outcomes match, and
`config validate` and routes test warn that the name will be retired. Repeat with
`"name": "aws-publik"`; routes test and the cost run warn.

After aws-public itself reports `aws-public` (Phase 4), install that release
and repeat the `finfocus-plugin-aws-public` run: outcomes are unchanged, through
the catalog alias, and `config validate` now calls it a former name.

An unknown `--adapter aws-publik` returns no rows and logs a warning naming
the value.

## 2. Conformance name checks

```bash
bin=$FINFOCUS_HOME/plugins/aws-public/v0.2.5/finfocus-plugin-aws-public
./bin/finfocus plugin conformance "$bin" --category protocol
./bin/finfocus plugin conformance "$bin" --category protocol --output json | jq '.summary'
./bin/finfocus plugin conformance "$bin" --category protocol --expected-name aws-public
echo "exit: $?"
```

Expected: `Name_MatchesInstallName` and `Name_IsCanonicalForm` warn,
`Name_MatchesPluginInfo` passes (both calls return the same prefixed name),
`summary.warnings` is 2, and the exit code is 0.

A binary copied outside the install layout under another name skips
`Name_MatchesInstallName` with a reason.

## 3. Guidance

```bash
go test -run 'TestFinfocusPluginDevSkill' ./internal/cli/
```

## 4. Gates

```bash
make lint
make test
```
