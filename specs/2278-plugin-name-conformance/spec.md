# Feature Specification: Plugin Name Conformance

**Feature Branch**: `2278-plugin-name-conformance`
**Created**: 2026-11-19
**Status**: Draft
**Input**: User description: "Plugin name conformance (issue #2278, with #2206): one plugin name everywhere. All 5 phases: (1) core conformance checks — Name_MatchesInstallName, Name_MatchesPluginInfo, Name_IsCanonicalForm, which warn rather than fail; (2) core identity: install name recorded on the plugin client; routing, config validate, config routes test, and --adapter match either name; warn on a routing entry that matches no plugin, and on a client whose Name() differs from its install name; (3) a new section in the finfocus-plugin-dev skill on naming conformance; (4) plugin fixes: aws-public (rshade/finfocus-plugin-aws-public#502), aws-ce, vantage, plus convention renames; (5) finfocus-spec pluginsdk WithExpectedName and a Name()==PluginInfo.Name check."

## Background

Every installed plugin has two names, and core does not check that they agree:

- **Install name**: the plugin's directory and catalog entry (`aws-public`),
  shown by `finfocus plugin list` and used to deliver settings and ESC values.
- **Reported name**: what the plugin answers to the `Name` call. Routing rules,
  `config validate`, `--adapter`, the Adapter column, and cost breakdown keys
  all use this one.

aws-public v0.2.5 reports `finfocus-plugin-aws-public` (checked on 2026-11-19
against the installed binary). The documentation tells users to write
`aws-public` in routing rules and `--adapter`. As #2278 shows, that name fails
`config validate`, passes `config routes test`, and is silently ignored at run
time, so the plugin answers features the rule excluded. #2206 shows the same
cause makes `--adapter <install name>` return an empty result with no warning.
cloudzero already changed its reported name to fix the same mismatch
(cloudzero v0.1.1).

The plugins also define their name in different ways: a constant in one file,
a literal in another, the same literal repeated for the logger and result
`Source` fields. The conformance suite checks only that the name is not empty,
so drift ships unnoticed.

This feature makes core accept the install name, a former name listed in
the plugin catalog, or the reported name, warns where they differ, adds
conformance checks and plugin-author guidance so new drift is caught, and
fixes the plugins that drift today. Phases 4 and 5 land in other repositories
under the constitution's cross-repo protocol; this spec records them so the
whole change is planned in one place.

| Phase | Stories | Repository |
| --- | --- | --- |
| 1. Conformance name checks | 4 | finfocus |
| 2. One plugin identity in core, with former-name aliases | 1, 2, 3, 8 | finfocus |
| 3. Plugin-dev naming guidance | 5 | finfocus |
| 4. Plugin fixes | 6 | plugin repositories |
| 5. In-process SDK checks | 7 | finfocus-spec, then finfocus |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Routing rules work with the name `plugin list` shows (Priority: P1)

A user copies a plugin name from `finfocus plugin list` into a routing rule, as
the routing guide says. `config validate` accepts it, `config routes test`
shows the rule applying, and cost commands honor it.

**Why this priority**: Today a documented configuration is silently ignored,
and costs come from a plugin the user excluded. That is a correctness bug in
results users act on (#2278).

**Independent Test**: With aws-public and cloudzero installed and a routing
config that limits aws-public to projected costs and cloudzero to actual costs,
run `config validate`, `config routes test`, and `cost actual` once with
`name: aws-public` and once with `name: finfocus-plugin-aws-public`. Both give
the same result: valid, the same routes, and aws-public is not the actual-cost
adapter.

**Acceptance Scenarios**:

1. **Given** a routing entry using a plugin's install name, **When** the user
   runs `config validate`, **Then** the configuration is reported valid.
2. **Given** a routing entry using a plugin's reported name, **When** the user
   runs `config validate`, **Then** the configuration is still reported valid
   (existing working configs keep working).
3. **Given** a routing entry using either name with a feature restriction,
   **When** the user runs a cost command, **Then** the restriction is applied
   to that plugin.
4. **Given** a routing entry using either name with a resource-type pattern
   rule, **When** a matching resource is priced, **Then** the pattern rule
   selects that plugin.
5. **Given** a routing entry using either name with fallback settings,
   **When** the plugin returns no result, **Then** fallback follows that
   entry's setting.
6. **Given** a routing entry whose name matches neither name of any loaded
   plugin, **When** the user runs `config validate`, **Then** it reports the
   entry as not found, as today.

---

### User Story 2 - `--adapter` selects a plugin by its install name (Priority: P1)

A user runs `finfocus cost actual --adapter aws-public` (or any command, or the
web dashboard, with `--adapter`) and gets that plugin's results.

**Why this priority**: The same root cause as Story 1 returns an empty result
with exit 0 and no warning (#2206). It shares the fix, so it lands with Story 1.

**Independent Test**: With aws-public installed, run a cost command with
`--adapter aws-public` and with `--adapter finfocus-plugin-aws-public`; both
return aws-public's results.

**Acceptance Scenarios**:

1. **Given** a plugin whose reported name differs from its install name,
   **When** the user passes the install name to `--adapter`, **Then** that
   plugin's results are returned.
2. **Given** the same plugin, **When** the user passes the reported name to
   `--adapter`, **Then** that plugin's results are returned.
3. **Given** `--adapter` names no installed plugin, **When** the command runs,
   **Then** it returns no results as today, every installed plugin is started
   to compare reported names, and a warning says no loaded plugin answers to
   that name.

---

### User Story 3 - Silent mismatches become visible (Priority: P2)

When a routing entry applies to no loaded plugin, or a plugin reports a name
that differs from its install name, the user sees a warning that names both
values, instead of finding out from wrong numbers.

**Why this priority**: Even with either-name matching, a typo (`aws-publik`)
still drops a restriction silently, and a plugin rename (Story 6) turns a
working entry into one that matches nothing. Warnings make the remaining
failure modes visible.

**Independent Test**: Configure `name: aws-publik` and run `config routes test`
and a cost command; the routes test reports a warning in table and JSON output,
and the cost command logs a warning that the entry's restrictions are not
applied. Run any cost command with aws-public v0.2.5 installed; a warning names
the install name and the reported name.

**Acceptance Scenarios**:

1. **Given** a routing entry that matches no installed plugin, **When** the
   user runs `config routes test`, **Then** the output contains a warning for
   that entry in both table and JSON formats, and the entry is not presented as
   the selected plugin.
2. **Given** a routing entry that matches no loaded plugin at run time,
   **When** a cost command runs, **Then** a warning naming the entry says its
   feature and pattern restrictions are not applied.
3. **Given** a routing entry of the form `finfocus-plugin-<x>` that matches no
   plugin while `<x>` is an installed plugin, **When** the user runs
   `config validate` or `config routes test`, **Then** the message suggests
   `<x>`.
4. **Given** a loaded plugin whose reported name differs from its install
   name, **When** core opens it, **Then** one warning per plugin per process
   names both values (a `--web` session is one process).
5. **Given** one routing name equals the install name of one plugin and the
   reported name of another, **When** routing resolves it, **Then** the install
   name wins and `config validate` warns about the ambiguity.

---

### User Story 4 - Plugin authors see name drift in conformance runs (Priority: P2)

A plugin author runs `finfocus plugin conformance` against a built binary and
sees whether the plugin's reported name matches its install name, whether it
matches the name in its plugin info, and whether it is in canonical form.
These results are warnings: they appear in every report format but do not fail
the run or change the exit code.

**Why this priority**: This catches the drift before a plugin ships. It is a
warning for now (user decision) so existing plugins are not blocked while
Phase 4 fixes them.

**Independent Test**: Run the conformance suite against aws-public v0.2.5; the
name checks report warnings and the overall run still passes. Run it against a
plugin that reports its install name; all three name checks pass.

**Acceptance Scenarios**:

1. **Given** a binary whose expected name can be determined, **When** its
   reported name differs, **Then** the check reports a warning naming both
   values.
2. **Given** the author passes an explicit expected name, **When** the suite
   runs, **Then** that name is used instead of one derived from the path.
3. **Given** the expected name cannot be determined (no explicit name, no
   install layout, no `finfocus-plugin-` binary prefix), **When** the suite
   runs, **Then** the install-name check warns that it did not run and says to
   pass `--expected-name`; it never guesses.
4. **Given** a plugin that does not implement plugin info, **When** the suite
   runs, **Then** the plugin-info check warns that it did not run.
5. **Given** a reported name with a `finfocus-plugin-` prefix, uppercase
   letters, or characters outside lowercase letters, digits, and single
   hyphens, **When** the suite runs, **Then** the canonical-form check warns.
6. **Given** a run whose only findings are name warnings, **When** the report
   is written, **Then** the summary counts them as warnings and the exit code
   is the same as a run where those checks passed.

---

### User Story 5 - Plugin-dev guidance states the naming convention (Priority: P3)

A plugin author, or an agent using the finfocus-plugin-dev skill, reads one
section that states the naming rule, shows the convention of one exported name
constant used everywhere, lists how to find stray copies, and explains how to
read the new conformance warnings.

**Why this priority**: Guidance prevents the next drift. It is cheap and
depends on the code changes only for the check names it cites.

**Independent Test**: The skill's naming section exists and cites the real
conformance check identifiers and flag; a contract test fails if either is
renamed.

**Acceptance Scenarios**:

1. **Given** the finfocus-plugin-dev skill, **When** an author reads it,
   **Then** a naming-conformance section states the rule, the one-constant
   convention, the places the constant must be used, and how to run and read
   the name checks.
2. **Given** a conformance check identifier or flag cited by the skill is
   renamed, **When** tests run, **Then** a contract test fails.
3. **Given** the skill text changes, **When** tests run, **Then** the packaged
   skill archive matches the source or the archive test fails.

---

### User Story 6 - Plugins report their install name (Priority: P3, cross-repo)

Each published plugin reports its install name from both the `Name` call and
its plugin info, with the name defined once in code and used for the logger
and result `Source` fields.

**Why this priority**: Core tolerates the mismatch after Stories 1 to 3, so
this is cleanup that removes warnings and makes the Adapter column match
`plugin list`. It lands in other repositories.

**Independent Test**: Each fixed plugin passes the three name checks with no
warnings.

**Acceptance Scenarios**:

1. **Given** aws-public, **When** it is fixed
   (rshade/finfocus-plugin-aws-public#502), **Then** its main and per-region
   binaries all report `aws-public`, and routing entries and `--adapter`
   values written as `finfocus-plugin-aws-public` keep working through the
   catalog alias (Story 8); its release notes recommend moving to `aws-public`.
2. **Given** aws-ce, **When** it is fixed, **Then** the logger and result
   `Source` fields use its single name definition instead of repeated literals.
3. **Given** vantage, **When** it is fixed, **Then** its name comes from one
   definition rather than an inline literal.
4. **Given** opencost, flexera, and cloudzero, **When** they adopt the
   convention, **Then** the name constant follows the documented convention.
   This is optional cleanup and does not change behavior.

---

### User Story 7 - Plugin unit tests catch drift without core (Priority: P3, cross-repo)

A plugin author's in-process conformance test (from finfocus-spec) can take an
expected name and checks that `Name` equals the plugin info name, so drift is
reported in the plugin's own CI before a binary exists.

**Why this priority**: It moves detection earlier still, but needs a
finfocus-spec release and an upgrade hop, so it follows the core work.

**Independent Test**: In finfocus-spec, a test plugin whose `Name` differs from
its plugin info name, or from a supplied expected name, is reported by the
in-process conformance run.

**Acceptance Scenarios**:

1. **Given** a plugin whose `Name` differs from its plugin info name, **When**
   the in-process conformance run executes, **Then** the mismatch is reported.
2. **Given** an expected name is supplied, **When** `Name` differs from it,
   **Then** the mismatch is reported.
3. **Given** the finfocus-spec release that adds this, **When** core bumps to
   it, **Then** the plugin-upgrade hop and guide for that version describe the
   new option.

---

### User Story 8 - A plugin's former name keeps working after a rename (Priority: P2)

A user whose config or scripts use a plugin's old name
(`finfocus-plugin-aws-public`, `finfocus-plugin-cloudzero`) upgrades that
plugin to a release that reports its install name. Their routing rules and
`--adapter` values keep working, and `config validate` tells them the new name.

**Why this priority**: Without it, the Phase 4 aws-public fix breaks every
config written to the name that works today, which is the only name that
works today (#2278). Plugin renames should never break configs.

**Independent Test**: With an aws-public that reports `aws-public`, run
the reproduction from #2278 with `name: finfocus-plugin-aws-public` and
`--adapter finfocus-plugin-aws-public`; outcomes match `aws-public`, and
`config validate` and `config routes test` show a deprecation warning naming
`aws-public`.

**Acceptance Scenarios**:

1. **Given** a catalog entry that lists a former name as an alias, **When** a
   routing entry or `--adapter` uses the alias, **Then** it selects that plugin,
   whatever name the installed version reports.
2. **Given** a routing entry that uses an alias, **When** the user runs
   `config validate` or `config routes test`, **Then** the configuration is
   valid and a warning says the name is a former name of `<install name>` and
   suggests it.
3. **Given** a cost command run with an alias, **When** it runs, **Then** no
   per-run warning is printed (validate is where users are told).
4. **Given** `--adapter <alias>`, **When** plugins are opened, **Then** only
   that plugin is started, as with its install name.
5. **Given** a plugin installed from a URL or a custom catalog entry without
   aliases, **When** names are matched, **Then** only its install name and
   reported name are accepted.
6. **Given** a catalog in which an alias equals another entry's name or alias,
   **When** the catalog is validated, **Then** validation fails naming both.

---

### Edge Cases

- A plugin opened outside the registry (a conformance run on a bare binary, a
  test client) has no install name; it matches on its reported name only, and
  no mismatch warning is raised for it.
- Per-region binaries (`finfocus-plugin-aws-public-us-east-1`): the expected
  name is the plugin's install name, not the binary name with the region.
- A plugin installed under a directory that itself carries the
  `finfocus-plugin-` prefix (the local, unreleased civo-public): the
  canonical-form check warns on the reported name; matching uses whatever
  directory exists.
- A plugin whose `Name` call succeeds but whose plugin info is unimplemented:
  matching uses the reported name; the plugin-info check is skipped.
- Two loaded plugins report the same name: behavior is unchanged from today
  apart from the install-name-wins rule in Story 3.
- Multi-stack web sessions share plugin processes through the plugin pool; a
  pooled client carries the install name of the plugin it was opened for.
- Mismatch warnings are emitted at most once per plugin per process (a
  `--web` session counts as one), never once per priced resource or reload.

## Requirements *(mandatory)*

### Functional Requirements

**Identity in core (Stories 1-3)**

- **FR-001**: Core MUST keep each registry-opened plugin's install name with
  the opened plugin, alongside its reported name, including for pooled plugin
  processes.
- **FR-002**: A single matching rule MUST decide whether a user-supplied plugin
  name refers to a loaded plugin: it matches when it equals the install name or
  the reported name. Routing rule lookup, pattern rules, fallback decisions,
  `config validate`'s known-name set, `--adapter` filtering, `--usage-source`
  and `--allocator`, and `scoring.plugin` MUST all use that one rule.
- **FR-003**: When a name equals the install name of one plugin and the
  reported name of another, the install name MUST win, and `config validate`
  MUST warn.
- **FR-004**: `config routes test` MUST compare routing names against installed
  plugin install names without launching plugins, and MUST report a routing
  name that matches no installed plugin as a warning in table and JSON output.
- **FR-005**: When a routing entry matches no loaded plugin at run time, core
  MUST log a warning naming the entry and stating that its feature and pattern
  restrictions are not applied.
- **FR-006**: A not-found routing name of the form `finfocus-plugin-<x>`, where
  `<x>` is an installed plugin, MUST produce a message suggesting `<x>`.
- **FR-007**: When core opens a plugin whose reported name differs from its
  install name, it MUST log one warning per plugin per process naming both
  values.
- **FR-008**: The Adapter column, breakdown keys, and other output that shows a
  plugin's reported name MUST stay unchanged by this feature.
- **FR-009**: The routing guide MUST state that the install name from
  `plugin list` is the recommended form, that the reported name shown in the
  Adapter column is also accepted, and that catalog former names are accepted
  with a `config validate` warning.

**Conformance checks (Story 4)**

- **FR-010**: The conformance suite MUST add three protocol checks:
  install-name match, plugin-info match, and canonical form.
- **FR-011**: The expected install name MUST come from, in order: an explicit
  expected-name option; the install layout `<plugins>/<name>/<version>/`; the
  binary name with the `finfocus-plugin-` prefix removed and any region suffix
  declared in the plugin's metadata removed. When none applies, the check MUST
  warn that it did not run and say how to supply the name. A name check never
  skips, because certification counts a skip as unverified.
- **FR-012**: Canonical form MUST mean lowercase letters and digits in
  hyphen-separated groups, with no `finfocus-plugin-` prefix.
- **FR-013**: A name-check finding MUST be reported as a warning that is
  visible in every report format (table, JSON, JUnit) and counted in the
  summary, and MUST NOT fail the run or change the exit code.

**Guidance (Story 5)**

- **FR-014**: The finfocus-plugin-dev skill MUST gain a naming-conformance
  section covering the rule, the one-constant convention and every place it is
  used (name call, plugin info, logger, result `Source` fields, cache paths),
  search recipes for stray literals, and how to read the name checks. The
  `plugin init` template MUST generate that constant, so new plugins start
  conforming.
- **FR-015**: A contract test MUST pin the check identifiers and flag that the
  skill cites, and the packaged skill archive MUST be rebuilt.

**Cross-repo (Stories 6-7)**

- **FR-016**: aws-public, aws-ce, and vantage MUST report their install names
  from one definition each, tracked as issues in their repositories
  (aws-public: rshade/finfocus-plugin-aws-public#502).
- **FR-017**: finfocus-spec's in-process conformance MUST accept an expected
  name and check that `Name` equals the plugin info name; adopting it in core
  requires the plugin-upgrade hop and version guide.
- **FR-018**: No core behavior in FR-001 to FR-015 or FR-019 to FR-024 may
  depend on the cross-repo changes landing first.

**Former names (Story 8)**

- **FR-019**: A plugin catalog entry MUST be able to list former names
  (`aliases`). aws-public lists `finfocus-plugin-aws-public`; cloudzero lists
  `finfocus-plugin-cloudzero`.
- **FR-020**: The matching rule in FR-002 MUST also accept a catalog alias of
  the plugin's install name. Precedence when names collide: install name, then
  alias, then reported name.
- **FR-021**: Aliases MUST be readable without launching plugins, so
  `config routes test` and the `--adapter` plugin filter use them.
- **FR-022**: A name that resolves only through an alias MUST produce a
  deprecation warning in `config validate` and `config routes test` naming the
  install name, and MUST NOT produce a per-run warning in cost commands. When
  the alias is also the plugin's current reported name (the plugin has not
  been renamed yet), the warning says the name will be retired rather than
  that it is a former name.
- **FR-023**: Catalog validation MUST reject an alias that is not a valid
  plugin name or that equals any entry's name or another alias. A custom
  plugin registry MUST NOT offer a plugin under a built-in name or alias.

**Unmatched adapter (Story 2)**

- **FR-024**: When `--adapter` matches no loaded plugin, core MUST log a
  warning naming the value.

### Key Entities

- **Install name**: the plugin's directory and catalog name; what
  `plugin list` shows and users are told to type.
- **Reported name**: the value the plugin returns from its name call; what
  output columns show today.
- **Name check finding**: a conformance result with a warning outcome, a check
  identifier, and either the expected and actual values or a skip reason.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable - the user-facing changes are in plugin
  selection, which the dashboard already reaches through the same engine path:
  `finfocus --web --adapter <name>` and routing config are honored by the
  dashboard's pricing calls, so Stories 1-2 fix the web with no web change. The
  mismatch warnings are log lines, which the opt-in web Logs view already
  shows. The conformance suite and the plugin-dev skill are developer tooling
  run against a binary path, with no dashboard counterpart.
- **Web data**: none new.
- **Layout fit**: none.
- **Additions**: none.
- **Render evidence**: not required for a Not applicable disposition.
- **Follow-up issue scope**: none.
- **Review**: Not applicable accepted by the owner on 2026-11-19
  (Principle VII), after checking that the dashboard has no routing or
  validation view and that `--web --adapter` uses the fixed code path.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With the #2278 reproduction setup, both spellings of the
  aws-public name give identical outcomes in all three routing surfaces and in
  `cost actual` (3 of 3 surfaces agree, versus 0 of 3 today).
- **SC-002**: `--adapter` with the install name returns the same rows as
  `--adapter` with the reported name for every installed plugin.
- **SC-003**: A misspelled routing name produces at least one visible warning
  in `config routes test` and in a cost run; no routing restriction is dropped
  without a warning.
- **SC-004**: A conformance run against aws-public v0.2.5 reports its name
  findings as warnings, and its exit code equals that of a run where the
  findings pass.
- **SC-005**: After Phase 4, every published plugin in the registry passes all
  three name checks with no warnings.
- **SC-006**: Existing routing configs that use reported names keep the same
  behavior (no regressions in the routing and adapter test suites).
- **SC-007**: After aws-public reports `aws-public`, configs and `--adapter`
  values spelled `finfocus-plugin-aws-public` give the same outcomes as
  before the plugin upgrade: zero configs broken by the rename.

## Assumptions

- Name checks warn rather than fail for now (user decision, 2026-11-19).
  Making them fail is a later change once Phase 4 has landed.
- Guidance goes in a new section of finfocus-plugin-dev, not a separate skill
  (user decision, 2026-11-19).
- Accepting either name follows #2278's proposal; existing configs depend on
  the reported name, so both must work.
- Former names live in core's plugin catalog as an `aliases` field, beside the
  existing `deprecated` and `replacement` fields, so they work for every
  installed version of the plugin and are readable without a launch (user
  request, 2026-11-19).
- No finfocus-spec change is needed for Stories 1-5; Story 7's change is
  additive.
- Plugins opened outside the registry keep matching on the reported name only.
- The `plugin_name` log field and cache directories inside plugins follow the
  same one-constant convention but are not checked by core.

## Out of Scope

- Changing the Adapter column to show install names.
- `config validate` accepting every routing name when plugins fail to load
  (it falls back to stand-in plugins named after the rules).
- `config routes test` listing installed plugins that have no routing entry.
- Making name checks fail the conformance run.
- Fixing the local, unreleased civo-public install layout (held in reserve).

## Related

- #2278 (routing) and #2206 (`--adapter`): same root cause, both resolved here.
- rshade/finfocus-plugin-aws-public#502: aws-public reports its install name.
- rshade/finfocus-plugin-cloudzero#17 and #21: cloudzero's earlier rename.
- #687 and #840: `config routes list` and `config routes test`.
