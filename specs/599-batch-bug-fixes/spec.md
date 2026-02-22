# Feature Specification: Batch Bug Fixes — Analyzer, Registry, Config, and CI Correctness

**Feature Branch**: `599-batch-bug-fixes`
**Created**: 2026-02-21
**Status**: Draft
**Issues**: #698, #723, #746, #747, #748, #749, #750, #751, #752, #753, #754

## Overview

This specification covers 11 confirmed bugs across four areas: Analyzer correctness
and output, registry symlink discovery, configuration/documentation mismatches, and
CI pipeline reliability. Each bug is independently fixable and testable.

---

## Clarifications

### Session 2026-02-21

- Q: For the three documentation/implementation mismatches (#751, #752, #753), should
  the fix implement the missing features or correct the documentation? → A: Implement
  the features. Research in `finfocus-demo/scripts/run-analyzer-preview.sh` confirms
  `FINFOCUS_HOME` isolation is the real-world pattern in use but is cumbersome; all
  three knobs have clear, narrow implementation paths in existing code and the
  infrastructure is already structured to support them.

- Q: How should "running as a Pulumi analyzer (policy pack)" be detected for #748 log
  redirect? → A: Check `constants.EnvAnalyzerMode` ("FINFOCUS_ANALYZER_MODE"), which
  is already set to `"true"` by `RunAnalyzerServe()` before any logging calls. No new
  flag or env var needed; the existing mechanism is the detection signal.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Analyzer Stack Summary Shows Correct Total Cost (Priority: P1)

A developer runs `pulumi preview` with the FinFocus analyzer policy pack on a stack
that has 20 resources. Individual per-resource advisories correctly show costs (e.g.,
`$7.59` for an EC2 instance). After all resources are analyzed, the
`stack-cost-summary` advisory should display the aggregate total (e.g.,
`$137.00 USD (20 resources analyzed)`).

Currently, `AnalyzeStack` always reports `$0.00 USD (0 resources analyzed)` regardless
of how many resources were priced by `Analyze()`. This makes the summary advisory
useless and erodes user trust in the tool.

**Why this priority**: The stack summary is the primary value-add of the Pulumi
analyzer integration. A permanent zero makes the feature appear broken for all users.

**Independent Test**: Install the analyzer, run `pulumi preview` on a stack with at
least two priced resources, and verify the `stack-cost-summary` advisory shows a
non-zero total equal to the sum of per-resource costs.

**Acceptance Scenarios**:

1. **Given** a Pulumi stack with 5 resources that each have a valid projected cost,
   **When** `pulumi preview --policy-pack ~/.finfocus/analyzer` completes,
   **Then** the `stack-cost-summary` advisory shows the correct aggregate total and a
   resource count of 5.

2. **Given** a Pulumi stack with 0 priceable resources (e.g., all validation errors),
   **When** `AnalyzeStack` is called,
   **Then** the summary shows `$0.00 USD (0 resources analyzed)` with no regression.

3. **Given** the analyzer server receives `ConfigureStack` followed by multiple
   `Analyze` calls and then `AnalyzeStack`,
   **When** the cost cache is inspected after `AnalyzeStack`,
   **Then** costs cached during `Analyze` are still present (not cleared prematurely).

---

### User Story 2 — Analyzer Install Creates Correct Version Directory (Priority: P1)

A developer runs `finfocus analyzer install`. The tool should create a directory
named `analyzer-finfocus-v0.3.1` (single `v` prefix). Currently it creates
`analyzer-finfocus-vv0.3.1` (double `v`), which prevents Pulumi from finding the
analyzer plugin.

**Why this priority**: This breaks `pulumi preview` integration for all users on
production builds that embed a `v`-prefixed version string. The tool cannot be used
as an analyzer without a manual rename.

**Independent Test**: Run `finfocus analyzer install` and verify the created directory
under `~/.pulumi/plugins/` has exactly one `v` in its name.

**Acceptance Scenarios**:

1. **Given** the current binary has version `v0.3.1`,
   **When** `finfocus analyzer install` is run,
   **Then** the directory `~/.pulumi/plugins/analyzer-finfocus-v0.3.1/` is created
   (not `analyzer-finfocus-vv0.3.1/`).

2. **Given** a version string without a leading `v` (dev builds like `0.3.1-dirty`),
   **When** `finfocus analyzer install` is run,
   **Then** the directory is `~/.pulumi/plugins/analyzer-finfocus-v0.3.1-dirty/`.

3. **Given** the analyzer is already installed at the correct path,
   **When** `finfocus analyzer install` is run again without `--force`,
   **Then** the tool reports "already installed" and does not create a duplicate.

---

### User Story 3 — SBOM Attached to GitHub Releases Successfully (Priority: P1)

When a new version tag is pushed, the Docker/SBOM workflow should scan the image and
attach the resulting SBOM `.spdx.json` file as an asset on the GitHub Release. Currently
the workflow fails with `Resource not accessible by integration` because the job has
`contents: read` instead of `contents: write`.

**Why this priority**: The CI pipeline fails on every release, leaving releases without
the SBOM artifact that is required for supply-chain compliance.

**Independent Test**: Push a version tag and confirm the SBOM `.spdx.json` file appears
as a release asset without errors in the workflow log.

**Acceptance Scenarios**:

1. **Given** a tag push triggers the SBOM workflow,
   **When** the `anchore/sbom-action` step runs,
   **Then** it completes without a permission error and the `.spdx.json` file appears
   on the GitHub Release.

2. **Given** the workflow runs on a non-tag push (e.g., to main),
   **When** the SBOM step runs,
   **Then** no release asset upload is attempted (correct skip behavior is preserved).

---

### User Story 4 — No JSON Log Lines in `pulumi preview` Diagnostics (Priority: P2)

A developer runs `pulumi preview --policy-pack ~/.finfocus/analyzer`. The
`Diagnostics:` section of Pulumi's output should contain only Pulumi-generated
messages. Currently, 50+ finfocus JSON log lines (e.g.,
`{"level":"info","component":"pluginhost",...}`) appear in `Diagnostics:` because
Pulumi captures the analyzer's stderr as diagnostic output.

**Why this priority**: The log noise makes the `Diagnostics:` section unusable and
the correct advisory output in `Policies:` harder to find.

**Independent Test**: Run `pulumi preview --policy-pack ~/.finfocus/analyzer` and
confirm that `Diagnostics:` contains no lines starting with `{\"level\":`.

**Acceptance Scenarios**:

1. **Given** `finfocus analyzer serve` is running as a Pulumi policy pack,
   **When** `pulumi preview` completes,
   **Then** the `Diagnostics:` section contains no finfocus JSON log lines.

2. **Given** log output is redirected to a file when running as an analyzer,
   **When** a debug-level log message is generated,
   **Then** it appears in the log file (e.g., `~/.finfocus/logs/analyzer.log`) rather
   than on stderr.

3. **Given** `FINFOCUS_ANALYZER_MODE` is not set (e.g., user runs `finfocus analyzer
   serve` directly from a terminal without Pulumi),
   **When** logs are generated,
   **Then** log output appears on the console as expected (no regression).

---

### User Story 5 — Recorder Plugin Does Not Flood Logs with Nil Summary Warnings (Priority: P2)

A developer uses the recorder plugin during local development or testing. Running
`finfocus cost recommendations` should not produce a WARN log entry per resource
(`"plugin returned response with nil summary"`). Currently, every resource triggers
this warning, flooding the diagnostic output and masking real issues.

**Why this priority**: This log noise makes the recorder plugin's own output hard to
interpret and pollutes structured log aggregators in CI environments.

**Independent Test**: Run any cost command with the recorder plugin installed and confirm
no `"nil summary"` WARN entries appear in the log output.

**Acceptance Scenarios**:

1. **Given** the recorder plugin is installed with mock mode disabled,
   **When** `finfocus cost recommendations` is called,
   **Then** no `"plugin returned response with nil summary"` WARN entries appear.

2. **Given** the recorder plugin is installed with mock mode enabled,
   **When** `finfocus cost recommendations` is called with a stack of 10 resources,
   **Then** 10 recommendation responses are returned, each with a valid (possibly
   empty) summary object.

---

### User Story 6 — Registry Discovers Plugins via Directory-Level Symlinks (Priority: P3)

A developer uses `FINFOCUS_HOME` isolation to test with a specific plugin set, creating
the plugin directory layout with `ln -s ~/.finfocus/plugins/aws-public
~/.finfocus/demo/plugins/aws-public`. Running `FINFOCUS_HOME=~/.finfocus/demo finfocus
plugin list` should list `aws-public`. Currently it returns no plugins because the
registry silently skips symlinks-to-directories.

**Why this priority**: Symlink-based isolation is the natural `ln -s` workflow. The
current behavior is surprising and forces users into a more complex per-file symlink
approach.

**Independent Test**: Create a symlinked plugin directory and verify `finfocus plugin list`
returns the expected plugin name.

**Acceptance Scenarios**:

1. **Given** a plugin name directory (`aws-public/`) is a symlink to a real directory,
   **When** `finfocus plugin list` runs,
   **Then** `aws-public` appears in the list.

2. **Given** a plugin version directory (`aws-public/v0.1.5/`) is a symlink to a real
   directory,
   **When** `finfocus plugin list` runs,
   **Then** the plugin with that version is discoverable.

3. **Given** symlinks point to non-existent targets (broken symlinks),
   **When** `finfocus plugin list` runs,
   **Then** the broken symlink is silently skipped (no crash, no error to the user).

4. **Given** file-level symlinks within version directories already worked,
   **When** the fix is applied,
   **Then** file-level symlinks continue to work (no regression).

---

### User Story 7 — Documentation Matches Implementation for Config/Env Vars (Priority: P3)

A developer reads the FinFocus docs and configures either `FINFOCUS_PLUGIN_DIR` or
`plugins.dir` in `config.yaml` expecting to redirect the plugin search path, or sets
`analyzer.plugins.<name>.enabled: false` to disable a plugin. All three settings are
documented but have no effect.

All three settings must be made functional. Research confirms each has a clear,
narrow implementation path in existing code with no architectural rework required.

**Why this priority**: Mismatches between docs and behavior cause wasted debugging time
and reduce confidence in the documentation as a whole.

**Independent Test**: Set each documented knob and verify either the behavior changes as
documented or the docs no longer mention the knob.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_PLUGIN_DIR=/custom/plugins` is set in the environment,
   **When** `finfocus plugin list` runs,
   **Then** either plugins are discovered from `/custom/plugins` OR the env var is
   removed from the environment-variables reference doc.

2. **Given** `plugins:\n  dir: /custom/plugins` is set in `config.yaml`,
   **When** `finfocus plugin list` runs,
   **Then** either plugins are discovered from `/custom/plugins` OR the config key is
   removed from the config reference doc.

3. **Given** `analyzer.plugins.recorder.enabled: false` is set in `config.yaml`,
   **When** `finfocus analyzer serve` runs,
   **Then** either the recorder plugin is not loaded OR the `enabled` field is removed
   from the config and docs.

---

### User Story 8 — Intermittent TUI Zero-Cost Display Investigated and Resolved (Priority: P3)

A developer runs `finfocus cost overview` multiple times. On some runs all projected
costs show `$0.00` even for known-priced resources; on other runs costs show correctly.
The investigation should identify the root cause (e.g., plugin startup timing, SKU
extraction failure, or gRPC connection issue) and produce a fix or a clear explanation
with mitigation.

**Why this priority**: Intermittent incorrect data in the TUI overview undermines trust
in all projected cost figures, including correct ones.

**Independent Test**: Run `finfocus cost overview --debug` repeatedly (10+ times) and
observe whether zero-cost runs can be reproduced and correlated with debug log entries
identifying the root cause.

**Acceptance Scenarios**:

1. **Given** the root cause is identified as plugin startup timing,
   **When** the fix is applied (e.g., retry logic or startup confirmation),
   **Then** 10 consecutive runs of `finfocus cost overview` all display non-zero costs
   for priced resources.

2. **Given** the root cause is identified as SKU/region extraction failure,
   **When** the fix is applied,
   **Then** the adapter logs no `"failed to extract SKU"` or `"failed to extract region"`
   warnings for resources that previously showed $0.00.

3. **Given** a zero-cost run occurs before the fix,
   **When** `--debug` logs are captured,
   **Then** the logs contain enough information to identify which component produced
   the $0.00 result.

---

### User Story 9 — Force Reinstall Keeps Policy Pack Binary in Sync (Priority: P4)

A developer upgrades finfocus and runs `finfocus analyzer install --force`. Both the
Pulumi plugin binary (`~/.pulumi/plugins/analyzer-finfocus-v.../`) and the policy pack
binary (`~/.finfocus/analyzer/pulumi-analyzer-policy-finfocus`) should be updated to
the new version. Currently, the policy pack binary is left at the old version.

**Why this priority**: This is blocked by the policy pack setup feature (which first
establishes the `--setup-policy-pack` workflow). Prioritized lower because the policy
pack setup must land first.

**Independent Test**: Install the analyzer, then simulate an upgrade by running
`--force` after a version change, and verify both binary locations reflect the new version.

**Acceptance Scenarios**:

1. **Given** the analyzer is installed with both the Pulumi plugin and policy pack
   binaries present,
   **When** `finfocus analyzer install --force` is run,
   **Then** both binary locations are updated to the new version.

2. **Given** the policy pack directory does not exist,
   **When** `finfocus analyzer install --force` is run,
   **Then** the Pulumi plugin binary is updated and no error is returned for the
   missing policy pack directory (graceful no-op).

---

### Edge Cases

- Symlinks that point to targets outside the plugin directory structure are skipped
  gracefully (security boundary preserved).
- When `ConfigureStack` is called by Pulumi after `Analyze` calls (unusual ordering),
  the analyzer must not clear cached costs prematurely.
- Version strings without a leading `v` (e.g., dev builds) must still produce a
  single-`v` prefix in the analyzer install directory.
- Documentation fixes must not remove information users rely on; they should redirect
  to the correct mechanism (`FINFOCUS_HOME` or `plugins:\n  dir:` if implemented).
- The log redirect for analyzer mode must not affect non-analyzer CLI usage.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `AnalyzeStack` handler MUST return a stack summary that sums all per-resource
  costs cached during preceding `Analyze` calls in the same session.
- **FR-002**: `ConfigureStack` MUST NOT call `clearCostCache()`. The cache MUST be cleared
  at the end of `AnalyzeStack` (after building the summary), not at stack configuration
  time. This prevents Pulumi's `ConfigureStack` call (which may occur after `Analyze`
  calls complete in practice) from wiping accumulated cost data before `AnalyzeStack`
  reads it. Additionally, the `BuildCostSummary` filtering logic MUST be validated to
  ensure it does not exclude valid (non-error) cost results.
- **FR-003**: `finfocus analyzer install` MUST create a directory with exactly one `v` prefix
  in the version component, regardless of whether `GetVersion()` returns a `v`-prefixed string.
- **FR-004**: The `.github/workflows/docker.yml` workflow MUST grant `contents: write`
  permission to the job that runs `anchore/sbom-action` with release asset upload enabled.
- **FR-005**: When `FINFOCUS_ANALYZER_MODE=true` is set in the environment (which
  `analyzer serve` sets before any logging occurs), all finfocus log output MUST be
  directed to the log file (default: `~/.finfocus/logs/analyzer.log`) and stderr MUST
  receive no structured log output, preventing Pulumi from capturing log lines as
  diagnostics.
- **FR-006**: The recorder plugin's `GetRecommendations` handler MUST return a valid,
  non-nil `Summary` object in all response paths (both mock-enabled and mock-disabled).
- **FR-007**: `ListPlugins()` in the registry MUST discover plugins whose name or version
  directories are symlinks to real directories, by following symlinks when checking
  directory status.
- **FR-008**: Broken symlinks (symlinks to non-existent targets) encountered during
  `ListPlugins()` MUST be silently skipped without crashing or emitting user-visible errors.
- **FR-009**: `FINFOCUS_PLUGIN_DIR` env var MUST override `cfg.PluginDir` when set,
  applied in `applyEnvOverrides()` with lower precedence than `FINFOCUS_HOME` (which
  sets the entire base directory). Documentation in `environment-variables.md` requires
  no change once implemented.
- **FR-010**: `plugins.dir` in `config.yaml` MUST override `cfg.PluginDir` when set.
  The `PluginDir` field's `yaml:"-"` tag MUST be replaced with a parsed config struct
  so the value is read from YAML and applied after the default is computed in `New()`.
- **FR-011**: `analyzer.plugins.<name>.enabled: false` in `config.yaml` MUST prevent
  that plugin from being launched during `analyzer serve`. The fix is in
  `setupAnalyzerInfra()`: after `reg.Open()` returns clients, filter out any client
  whose name appears in `cfg.Analyzer.Plugins` with `Enabled == false`.
- **FR-012**: When `finfocus analyzer install --force` is run and a policy pack directory
  already exists, the policy pack binary MUST be updated to match the newly installed
  Pulumi plugin binary.
- **FR-013**: The root cause of intermittent $0.00 projected costs in the TUI overview
  MUST be identified, documented, and either fixed or mitigated with a minimum
  reproducible test case.

### Key Entities

- **Cost Cache**: The in-memory store (`map[string]engine.CostResult`) in the analyzer
  server that accumulates per-resource costs across `Analyze` calls and is read once
  by `AnalyzeStack`.
- **Analyzer Install Directory**: The filesystem directory under `~/.pulumi/plugins/`
  whose name must exactly follow the `analyzer-finfocus-v{version}` format.
- **Policy Pack Directory**: The directory at `~/.finfocus/analyzer/` containing
  `PulumiPolicy.yaml` and the renamed finfocus binary used with `--policy-pack`.
- **Plugin Discovery Root**: The directory scanned by `ListPlugins()`; symlinks within
  it at the name or version level must be followed using stat-following directory checks.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After the fix, running `pulumi preview` on a stack with N priced resources
  results in a `stack-cost-summary` advisory showing the correct aggregate total and
  resource count of N, verified across 5 consecutive runs.
- **SC-002**: `finfocus analyzer install` creates a directory with exactly one `v` prefix,
  verified by automated unit test across version strings both with and without a
  leading `v`.
- **SC-003**: The release workflow completes with the SBOM `.spdx.json` attached as a
  release asset, with zero permission-related errors in the workflow log.
- **SC-004**: Running `pulumi preview --policy-pack ~/.finfocus/analyzer` on any stack
  produces zero JSON log lines in the `Diagnostics:` section, verified by string search
  on the captured output.
- **SC-005**: Running any cost command with the recorder plugin installed produces zero
  `"nil summary"` WARN log entries, verified by log inspection after 10 consecutive runs.
- **SC-006**: A symlinked plugin directory is discovered and listed by `finfocus plugin list`,
  confirmed on platforms that support symlinks (Linux/macOS).
- **SC-007**: All three documentation mismatches (#751, #752, #753) are resolved — each
  documented configuration knob either works as documented or is replaced with accurate
  documentation referencing the correct mechanism.
- **SC-008**: The root cause of intermittent $0.00 projected costs is documented in the
  issue with a reproducible test case or is fixed with a regression test.
- **SC-009**: All modified Go packages maintain at least the existing test coverage
  percentage; new behavior introduced by each fix is covered by at least one unit test.

---

## Assumptions

- For #752 (`FINFOCUS_PLUGIN_DIR`): the env var override is added to `applyEnvOverrides()`
  and takes lower precedence than `FINFOCUS_HOME`. No new struct or YAML key required.
- For #753 (`plugins.dir`): requires replacing `yaml:"-"` on `PluginDir` (or adding a
  `PluginsTopConfig` struct with a `Dir` field). The default computed in `New()` is used
  when the config key is absent; an explicit config value wins. `FINFOCUS_HOME` still
  takes highest precedence for the entire base directory.
- For #751 (`AnalyzerPlugin.Enabled`): all plugins are still opened by `reg.Open()` for
  efficiency; only the resulting `Client` list is filtered in `setupAnalyzerInfra()`.
  A plugin with no `enabled` key defaults to enabled (backward compatible).
- For #748 (log redirect): `FINFOCUS_ANALYZER_MODE` is already set by `RunAnalyzerServe()`
  before logging is initialized. Logging setup checks this env var and, when "true",
  replaces the console output with a file output pointing to
  `~/.finfocus/logs/analyzer.log`. The zerolog `LoggingConfig.Outputs` mechanism already
  supports this without new infrastructure.
- For #746 (stack summary zero cost): `ConfigureStack` is the confirmed root cause —
  it clears the cache that `Analyze` calls populate. Moving `clearCostCache()` to the
  end of `AnalyzeStack` is the fix. The `BuildCostSummary` filter path (error results
  excluded) must also be verified as a secondary cause.
- Issue #754 (force reinstall policy pack sync) is explicitly lower priority because
  it depends on the policy pack setup feature; its fix will be additive once that
  feature lands.
- For #723 (intermittent $0.00), the minimum deliverable is a documented root cause
  with a reproducible test case; a full fix is expected but the investigation outcome
  may reveal the fix belongs in the plugin, not core.
