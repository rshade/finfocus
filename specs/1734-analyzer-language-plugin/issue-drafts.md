# Issue drafts for 1734

This draft is not filed. File it after the owner approves the text. FR-014 was
implemented in this change and has no issue.

## FR-013: rshade/finfocus-action analyzer setup

**Title:** Update analyzer setup for Pulumi 3.270 language plugins

**Body:**

Pulumi 3.270.0 starts a directory policy pack through
`pulumi-language-<runtime>` and no longer falls back to
`pulumi-analyzer-policy-<runtime>`. FinFocus Core now writes both entries
from `finfocus analyzer install` (`specs/1734-analyzer-language-plugin/` in
`rshade/finfocus`, issue #1895).

`src/analyze.ts` `setupAnalyzerMode` still creates only the legacy
`pulumi-analyzer-policy-finfocus` entry. A workflow that uses this action on
Pulumi 3.270.0 or later fails preview with:

```text
no language plugin 'pulumi-language-finfocus' found in the workspace or on your $PATH
```

Pulumi may also print a 404 while looking up a download named `pulumi-finfocus`.

Do this:

1. Read `src/analyze.ts` `setupAnalyzerMode` (it writes `PulumiPolicy.yaml`, copies the binary to `pulumi-analyzer-policy-finfocus` with no `.exe` handling, and adds the directory to `PATH`) and every caller that copies or
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
