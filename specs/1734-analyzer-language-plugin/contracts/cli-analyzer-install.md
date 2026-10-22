# Contract: finfocus analyzer install

**Requirements**: FR-004, FR-005, FR-006 | **Research**: R8

Flags are unchanged (`--force`, `--target-dir`, plus the ax-mounted
`--format`, `--dry-run`, `--yes`). Exit codes are unchanged.

## Effect

Every run, including "already installed" runs, calls
`analyzer.SetupPolicyPack`, which writes `PulumiPolicy.yaml` and both pack
entries (see [data-model.md](../data-model.md)). A failure to set up the pack
stays a logged warning, as today, and leaves the policy pack fields empty.

## Human output

After the existing lines, when the pack was set up:

```text
  Policy pack: <policy pack dir>
  Policy pack method: <symlink|copy>
  Language plugin: <policy pack dir>/pulumi-language-finfocus[.exe]

To use the analyzer with pulumi preview:

  export PATH="<policy pack dir>:$PATH"

Then run:

  pulumi preview --policy-pack "<policy pack dir>"
```

On Windows the `PATH` line is the existing PowerShell form. The only new line
is `Language plugin:`; the rest is the current output, kept word for word,
because users and `finfocus-action` may read it.

## JSON output (`--format json`)

`InstallResult` gains `policy_pack_language_plugin` (absolute path, omitted
when the pack was not set up). No field is renamed or removed.

```json
{
  "installed": true,
  "version": "v0.5.2",
  "path": ".../plugins/analyzer-finfocus-v0.5.2/pulumi-analyzer-finfocus",
  "method": "symlink",
  "needs_update": false,
  "action": "installed",
  "policy_pack_dir": "/home/u/.finfocus/analyzer",
  "policy_pack_method": "symlink",
  "policy_pack_language_plugin": "/home/u/.finfocus/analyzer/pulumi-language-finfocus"
}
```

## MCP

`analyzer install` is already exposed through `mcp-server`; the tool list in
`internal/cli/testdata/mcp/tools.golden` does not change, because no flag or
command is added.
