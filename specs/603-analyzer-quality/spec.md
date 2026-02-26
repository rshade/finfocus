# Feature Specification: Analyzer Quality Cluster

**Feature Branch**: `603-analyzer-quality`
**Created**: 2026-02-24
**Status**: Draft
**Input**: User description: "Analyzer Quality Cluster (#746, #754, #755, #756, #757)"
**Issues**: [#746](https://github.com/rshade/finfocus/issues/746),
[#754](https://github.com/rshade/finfocus/issues/754),
[#755](https://github.com/rshade/finfocus/issues/755),
[#756](https://github.com/rshade/finfocus/issues/756),
[#757](https://github.com/rshade/finfocus/issues/757)

## User Scenarios & Testing

### User Story 1 - Accurate Stack Cost Summary (Priority: P1)

A DevOps engineer runs `pulumi preview` with the finfocus analyzer configured.
The analyzer calculates per-resource costs correctly (e.g., $7.59 for an EC2
instance), but the stack-level summary diagnostic always reports "$0.00 USD
(0 resources analyzed)." The engineer cannot trust the analyzer output because
the summary contradicts the individual resource costs. After this fix, the
stack summary accurately reflects the sum of all per-resource costs and the
correct count of analyzed resources.

**Why this priority**: The stack summary is the primary signal users see during
`pulumi preview`. A perpetually $0.00 summary makes the entire analyzer feature
appear broken, undermining trust in all cost output.

**Independent Test**: Can be tested by running `pulumi preview` with the
analyzer configured against any stack with priced resources and verifying the
summary matches the sum of individual resource costs.

**Acceptance Scenarios**:

1. **Given** a Pulumi stack with 5 resources that have per-resource costs
   calculated by the analyzer, **When** `AnalyzeStack` is called,
   **Then** the `stack-cost-summary` diagnostic displays the sum of those
   5 costs and reports "5 resources analyzed."
2. **Given** a Pulumi stack where 3 of 5 resources produce cost errors
   (e.g., unsupported resource types), **When** `AnalyzeStack` is called,
   **Then** the summary includes only the 2 successful costs and reports
   "2 resources analyzed."
3. **Given** a Pulumi stack with no priced resources (all errors or
   unsupported types), **When** `AnalyzeStack` is called, **Then** the
   summary reports "$0.00 USD (0 resources analyzed)" correctly (this is
   the legitimate case for $0.00).

---

### User Story 2 - Policy Pack Directory Setup (Priority: P2)

A DevOps engineer installs the finfocus analyzer using
`finfocus analyzer install`. After installation, they want to use the
`--policy-pack` workflow with `pulumi preview --policy-pack ~/.finfocus/analyzer`.
Currently this fails because the policy pack directory structure
(`PulumiPolicy.yaml` and the correctly-named binary) does not exist.
The engineer must manually create the directory, write a YAML file,
and create a symlink. After this feature, `finfocus analyzer install`
sets up the policy pack directory automatically so the workflow works
immediately.

**Why this priority**: The policy pack workflow is the primary use case
for the analyzer. Without automatic setup, users face a confusing manual
process that requires deep Pulumi knowledge.

**Independent Test**: Can be tested by running `finfocus analyzer install`,
then verifying the policy pack directory exists with the correct contents,
and that `pulumi preview --policy-pack ~/.finfocus/analyzer` succeeds.

**Acceptance Scenarios**:

1. **Given** the finfocus binary is installed, **When** the user runs
   `finfocus analyzer install`, **Then** the policy pack directory
   `~/.finfocus/analyzer/` is created with `PulumiPolicy.yaml` and
   a `pulumi-analyzer-policy-finfocus` binary reference.
2. **Given** the policy pack directory already exists from a previous
   install, **When** the user runs `finfocus analyzer install`,
   **Then** the existing directory is preserved without error.
3. **Given** the user is on Windows, **When** they run
   `finfocus analyzer install`, **Then** the binary reference uses a
   copy instead of a symlink (Windows symlink limitations).

---

### User Story 3 - Force Reinstall Syncs Policy Pack (Priority: P3)

A DevOps engineer upgrades finfocus to a new version and runs
`finfocus analyzer install --force`. The Pulumi plugin binary is updated,
but the policy pack binary at `~/.finfocus/analyzer/` remains stale from
the previous version. Running `pulumi preview --policy-pack` now uses
the old version. After this fix, `--force` updates both the Pulumi plugin
binary and the policy pack binary.

**Why this priority**: This is a correctness issue that only manifests
after upgrades. It depends on the policy pack setup feature (User Story 2)
being implemented first.

**Independent Test**: Can be tested by installing version A, then force
reinstalling version B, and verifying both binary locations reflect
version B.

**Acceptance Scenarios**:

1. **Given** the analyzer was previously installed and the policy pack
   directory exists, **When** the user runs
   `finfocus analyzer install --force`, **Then** both the Pulumi plugin
   binary and the policy pack binary are updated to the current version.
2. **Given** the policy pack directory does not exist (user never set it
   up), **When** the user runs `finfocus analyzer install --force`,
   **Then** the force reinstall succeeds for the Pulumi plugin binary and
   the missing policy pack directory does not cause an error.
3. **Given** the force reinstall succeeds for the Pulumi plugin binary but
   the policy pack sync fails (e.g., permission denied), **When** the
   install completes, **Then** the user sees a warning about the failed
   policy pack sync but the overall install is not marked as failed.

---

### User Story 4 - Post-Install PATH Instructions (Priority: P4)

A DevOps engineer installs the finfocus analyzer for the first time. After
installation, they have no guidance on how to configure their PATH so that
Pulumi can discover the analyzer binary. They attempt
`pulumi preview --policy-pack` and receive a cryptic error about a missing
plugin. After this feature, the install command prints clear next-step
instructions including the PATH export command and the `pulumi preview`
invocation.

**Why this priority**: This is a UX improvement that removes a common
stumbling block. It does not fix broken functionality but eliminates
confusion for new users.

**Independent Test**: Can be tested by running `finfocus analyzer install`
and verifying the output includes PATH export instructions and the
`pulumi preview --policy-pack` command.

**Acceptance Scenarios**:

1. **Given** the user runs `finfocus analyzer install` and the install
   succeeds, **When** the output is displayed, **Then** it includes a
   PATH export command referencing the policy pack directory and a sample
   `pulumi preview --policy-pack` command.
2. **Given** the user runs `finfocus analyzer install` and the analyzer
   is already current (no-op), **When** the output is displayed, **Then**
   no PATH instructions are shown (the user presumably already has it
   configured).
3. **Given** the user runs `finfocus analyzer install --output json`,
   **When** the output is displayed, **Then** PATH instructions are not
   included in the JSON output (they are human-facing guidance only).

---

### User Story 5 - Analyzer Check Command (Priority: P5)

A DevOps engineer has installed the analyzer but is unsure if everything
is configured correctly. Rather than running a full `pulumi preview` to
find out, they run `finfocus analyzer check`. The command verifies each
component of the setup (policy pack directory, YAML config, PATH,
gRPC connectivity) and reports pass/fail status with actionable
remediation instructions for any failures.

**Why this priority**: This is a diagnostic tool that adds polish and
self-service troubleshooting capability. It depends on the policy pack
setup (User Story 2) to have a defined structure to check against.

**Independent Test**: Can be tested by running `finfocus analyzer check`
in both correctly and incorrectly configured environments and verifying
the output matches the expected pass/fail states.

**Acceptance Scenarios**:

1. **Given** the analyzer is fully and correctly configured, **When** the
   user runs `finfocus analyzer check`, **Then** all checks pass and the
   command exits with code 0.
2. **Given** the policy pack directory is missing, **When** the user runs
   `finfocus analyzer check`, **Then** the check for the policy pack
   directory fails with an actionable message (e.g., "Run
   `finfocus analyzer install` to set up the policy pack directory").
3. **Given** the binary is not in PATH, **When** the user runs
   `finfocus analyzer check`, **Then** the PATH check fails with an
   actionable message including the exact `export PATH=...` command.
4. **Given** the user runs `finfocus analyzer check --output json`,
   **When** the output is displayed, **Then** the check results are
   returned as a machine-readable JSON object with pass/fail status
   and messages for each check.

---

### Edge Cases

- What happens when the analyzer binary does not exist at the expected
  path (e.g., installed via a different method)?
- How does the system handle a corrupted `PulumiPolicy.yaml` (present
  but missing required fields)?
- What happens if the gRPC smoke test in `analyzer check` times out
  (server starts but never responds)?
- What happens when `ConfigureStack` is called after `Analyze()` calls,
  clearing the cost cache before `AnalyzeStack` reads it?
- How does `--force` behave when the policy pack binary is locked by
  another process?
- What happens on systems where symlinks are not supported (older
  Windows without developer mode)?

## Requirements

### Functional Requirements

- **FR-001**: The stack cost summary MUST display the correct total of all
  successfully calculated per-resource costs from `Analyze()` calls.
- **FR-002**: The stack cost summary MUST report the accurate count of
  resources that had successful cost calculations.
- **FR-003**: Resources with cost errors (validation failures, plugin
  errors) MUST be excluded from the summary total but their count MUST
  be distinguishable from successfully priced resources.
- **FR-004**: The analyzer install command MUST create a policy pack
  directory with `PulumiPolicy.yaml` declaring `runtime: finfocus` and
  a correctly-named binary reference.
- **FR-005**: The policy pack directory location MUST default to
  `~/.finfocus/analyzer/`.
- **FR-006**: On systems without symlink support, the binary reference
  MUST use a file copy instead.
- **FR-007**: When `--force` is used and a policy pack directory exists,
  the install command MUST update the policy pack binary to match the
  current version.
- **FR-008**: If the policy pack directory does not exist during a
  `--force` reinstall, the policy pack sync MUST be silently skipped.
- **FR-009**: A failed policy pack sync during `--force` MUST produce a
  warning but MUST NOT cause the overall install to fail.
- **FR-010**: After a successful install, the command MUST print PATH
  setup instructions and a sample `pulumi preview --policy-pack` command.
- **FR-011**: PATH instructions MUST NOT be printed when the install is
  a no-op (already current) or when JSON output mode is active.
- **FR-012**: The `analyzer check` command MUST verify: policy pack
  directory existence, `PulumiPolicy.yaml` validity, binary discoverability
  in PATH, and gRPC server responsiveness.
- **FR-013**: Each check MUST report pass or fail with an actionable
  remediation message on failure.
- **FR-014**: The `analyzer check` command MUST exit 0 when all checks
  pass and non-zero when any check fails.
- **FR-015**: The `analyzer check` command MUST support `--output json`
  for machine-readable results.

### Key Entities

- **Cost Cache**: In-memory map of resource URN to cost result, populated
  during `Analyze()` calls and read during `AnalyzeStack()`.
- **Policy Pack Directory**: A directory containing `PulumiPolicy.yaml`
  and the analyzer binary, required by Pulumi's `--policy-pack` workflow.
- **Check Result**: The outcome of a single verification step in
  `analyzer check`, including status (pass/fail), message, and
  remediation guidance.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The stack cost summary displays the correct total for 100%
  of stacks where per-resource costs are successfully calculated (zero
  discrepancy between individual costs and summary).
- **SC-002**: After running `finfocus analyzer install`, the
  `pulumi preview --policy-pack` workflow succeeds without any additional
  manual setup steps.
- **SC-003**: After a `--force` reinstall, both binary locations reflect
  the current version (verified by version output or file modification
  timestamp).
- **SC-004**: First-time users can complete the full analyzer setup
  (install through first successful `pulumi preview`) by following only
  the instructions printed by the install command.
- **SC-005**: `finfocus analyzer check` correctly identifies all
  misconfiguration scenarios and provides actionable remediation for
  each failure.
- **SC-006**: All five issues in this cluster are resolved with
  accompanying tests achieving 80%+ coverage of new code.

## Assumptions

- The `~/.finfocus/analyzer/` directory is the standard location for the
  policy pack. This aligns with the existing `~/.finfocus/` directory
  convention used elsewhere in the project.
- `PulumiPolicy.yaml` requires `runtime: finfocus` as the runtime
  declaration. This matches the Pulumi policy pack specification.
- The gRPC smoke test in `analyzer check` will use a short timeout
  (e.g., 5 seconds) to avoid blocking the user.
- Policy pack setup is performed by default during `finfocus analyzer install`
  (not gated behind an opt-in flag), since the policy pack workflow is the
  primary use case. This follows the principle of making the common path easy.
- The `ConfigureStack`/`Analyze`/`AnalyzeStack` call ordering follows the
  Pulumi Analyzer protocol: `ConfigureStack` is called once before any
  `Analyze` calls, and `AnalyzeStack` is called after all `Analyze` calls.
- Dependencies: #755 (policy pack setup) must be implemented before
  #754 (`--force` sync), as the sync behavior requires the setup
  infrastructure to exist.
