# Issue drafts for 1734

These drafts are not filed. File them after the owner approves the text.

## FR-013: rshade/finfocus-action analyzer setup

**Title:** Update analyzer setup for Pulumi 3.270 language plugins

**Body:**

Pulumi 3.270.0 starts a directory policy pack through
`pulumi-language-<runtime>` and no longer falls back to
`pulumi-analyzer-policy-<runtime>`. FinFocus Core now writes both entries
from `finfocus analyzer install` (`specs/1734-analyzer-language-plugin/` in
`rshade/finfocus`, issue #1895).

`src/analyze.ts` `setupAnalyzer` still creates only the legacy
`pulumi-analyzer-policy-finfocus` entry. A workflow that uses this action on
Pulumi 3.270.0 or later fails preview with:

```text
no language plugin 'pulumi-language-finfocus' found in the workspace or on your $PATH
```

Pulumi may also print a 404 while looking up a download named `pulumi-finfocus`.

Do this:

1. Read `src/analyze.ts` `setupAnalyzer` and every caller that copies or
   links the finfocus binary into the policy pack directory.
2. Prefer running `finfocus analyzer install` when the finfocus binary is
   already on `PATH`. That command writes `PulumiPolicy.yaml`,
   `pulumi-analyzer-policy-finfocus`, and `pulumi-language-finfocus` (plus
   `.exe` on Windows) and is the layout FinFocus tests.
3. If the action must keep building the pack itself, write both entries next
   to `PulumiPolicy.yaml`. On Windows append `.exe`. Do not drop the legacy
   name: Pulumi before 3.270.0 still starts it.
4. Put the pack directory on `PATH` for the preview step, using the path the
   action already prints.
5. Add a test or fixture check that the pack directory contains both entry
   names after setup.
6. Document the Pulumi 3.270.0 break and the fix in the action README.

Done when a `pulumi preview --policy-pack` using this action succeeds on
Pulumi 3.268.0 and on 3.270.0 or later, and the pack directory contains both
entries.

## FR-014: resolve the analyzer project directory from the Pulumi project

**Title:** Read the analyzer project directory from RunPluginRequest.pwd

**Body:**

In policy-pack mode the FinFocus analyzer resolves its project directory from
`os.Getwd()` (`internal/cli/analyzer_serve.go`). Pulumi 3.270.0 and later
starts `pulumi-language-finfocus`, and `RunPlugin` sets the analyzer working
directory to `info.programDirectory`, which is the policy pack directory
(`~/.finfocus/analyzer`). There is no `Pulumi.yaml` there, so the analyzer
reads only the global config tier. Project `.finfocus/` config, the summary
file, and cost history do not follow the project being previewed.

`RunPluginRequest.pwd` is the Pulumi project. The language host currently
ignores it (`specs/1734-analyzer-language-plugin/`, research R6, option C).
This issue moves that resolution.

Do this:

1. Pass `RunPluginRequest.pwd` to the analyzer process in a way `analyzer
   serve` already understands. Prefer the existing `--project-dir` flag or
   `FINFOCUS_PROJECT_DIR`. Do not invent a second project-directory rule.
2. In `internal/cli/analyzer_serve.go`, resolve the project from that value
   before `os.Getwd()`. `Getwd()` stays the fallback when `pwd` is empty, so
   Pulumi before 3.270.0 (which starts the analyzer directly) keeps today's
   behavior.
3. Write `last-cost-summary.json` and cost history under the resolved project
   when one is found, and under the global directory when it is not. State
   the new paths in the analyzer docs and in the release note.
4. Apply the existing `cost.history.directory` safety check
   (`config.ProjectHistoryDirectory`) to a project-tier directory resolved
   this way. A path that escapes the project root stays ignored with a
   warning. Do not add a second safety rule.
5. Tests: a language-host `RunPlugin` with `pwd` set to a project that has
   `.finfocus/config.hujson` makes the analyzer read that overlay; an empty
   `pwd` still uses the pack directory; a project `history.directory` outside
   the project is ignored. The integration test
   `TestAnalyzerPolicyPack_LoadsInPulumi/language_plugin` should expect the
   summary in the project directory once this lands, not only under
   `FINFOCUS_HOME`.
6. Release note: policy-pack previews on Pulumi 3.270.0 and later now honor
   project-tier config, and summaries and history move to the project. Call
   out the path change for anyone reading `~/.finfocus/last-cost-summary.json`.

Do not change plan totals, budgets, or the language-host dispatch while doing
this. Plugin domain logic stays in the plugins.
