# Research: Config Routes CLI Commands

**Feature**: 604-config-routes-cli
**Date**: 2026-02-28

## R1: Priority Sort Order

**Decision**: Higher priority number = higher precedence (tried first).

**Rationale**: The router's `sortByPriority` function uses descending stable
sort: `matches[i].Priority > matches[j].Priority`. A plugin with priority 10
is selected before priority 5. This matches the issue examples showing
`aws-public` (priority 10) as the first choice.

**Alternatives considered**:

- The spec originally assumed lower number = higher precedence (corrected).
- The CLAUDE.md router gotcha section states "Lower number = higher priority"
  which contradicts the actual sort implementation. The code is authoritative.

## R2: Output Flag Convention

**Decision**: Use `--output` flag (no short flag) with default `"table"`.

**Rationale**: The majority of CLI commands use `--output` for format
switching (plugin list, plugin conformance, analyzer check, overview, cost
recommendations history). Only `config list` uses `--format`/`-f` because it
defaults to YAML output (not table). Since the new commands display tables by
default, `--output` is the correct convention.

**Alternatives considered**:

- `--format`/`-f`: Only used by `config list` which defaults to YAML. Not
  appropriate for table-default commands.

## R3: Config Path Accessibility

**Decision**: Use `cfg.ConfigPath()` to display the config source path.

**Rationale**: The `Config` struct exposes `ConfigPath() string` as a public
getter for the unexported `configPath` field. This returns the path to the
active config file (e.g., `~/.finfocus/config.yaml`). For project-local
detection, check `config.GetResolvedProjectDir()`.

**Alternatives considered**: None. This is the established API.

## R4: Synthetic Client Construction for Route Testing

**Decision**: Create `pluginhost.Client` structs with only `Name` and
`Metadata` fields populated. No gRPC connection needed.

**Rationale**: The router's `SelectPlugins` only accesses `client.Name`,
`client.Metadata.SupportedProviders`, `client.Metadata.Capabilities`, and
`client.Metadata.Metadata["region"]`. The `Conn`, `API`, and `Close` fields
are never touched during routing. This pattern is used by:

- All router unit tests via `mockClient(name, providers)`
- `config_validate.go` fallback path which creates
  `&pluginhost.Client{Name: p.Name}` synthetic stubs

**Alternatives considered**:

- Launching real plugins: Rejected per FR-010 (must work without plugin
  binaries).

## R5: Feature Types

**Decision**: Display all 6 feature types in route test output.

**Rationale**: The router supports 6 features: ProjectedCosts, ActualCosts,
Recommendations, Carbon, DryRun, Budgets. The `config routes test` command
should iterate all valid features via `router.ValidFeatureNames()` and show
which plugin would handle each.

**Alternatives considered**:

- Show only the 3 most common (ProjectedCosts, ActualCosts, Recommendations):
  Rejected because users configuring Carbon, DryRun, or Budgets features need
  visibility too.

## R6: Router Match Algorithm for Test Command

**Decision**: Use the existing `router.DefaultRouter.SelectPlugins()` method
with synthetic clients constructed from config.

**Rationale**: The `SelectPlugins` method implements the full two-pass
matching algorithm (declarative patterns first, then automatic provider
matching). Reusing it ensures the `config routes test` output exactly mirrors
what would happen at runtime. The method works with synthetic clients that
have no gRPC connections.

**Alternatives considered**:

- Re-implementing matching logic: Rejected because it would diverge from
  actual runtime behavior and require maintenance synchronization.
- Using `router.ValidateRoutingConfig()`: This validates config structure but
  does not simulate selection for a specific resource type.

## R7: Provider Extraction from Resource Type

**Decision**: Extract provider from the resource type string using the
existing `router.ExtractProviderFromType()` function.

**Rationale**: The function parses Pulumi-style type tokens (e.g.,
`aws:ec2:Instance` -> `aws`, `azure:compute:VirtualMachine` -> `azure`). It
handles edge cases like types with `/` separators and empty strings.

**Alternatives considered**: Manual string splitting. Rejected because the
existing function handles all edge cases.

## R8: Table Formatting

**Decision**: Use `tabwriter` with standard parameters matching existing CLI
commands.

**Rationale**: Multiple CLI commands use
`tabwriter.NewWriter(writer, 0, 0, tabPadding, ' ', 0)` where `tabPadding = 2`.
The constant is already defined in the `cli` package
(`cost_recommendations.go`). Column headers use uppercase with dashes as
separators (e.g., `PLUGIN\tPRIORITY`, `------\t--------`).

**Alternatives considered**: None. This is the established pattern.

## R9: Command Registration Pattern

**Decision**: Create `NewConfigRoutesCmd()` (exported) as the parent and
`NewConfigRoutesListCmd()` and `NewConfigRoutesTestCmd()` (exported) as
subcommands. Register via `newConfigCmd()` in root.go.

**Rationale**: Follows the established pattern where group commands use
lowercase `newXxxCmd()` for internal groups and `NewXxxCmd()` (uppercase) for
individually testable subcommands. However, since `config routes` is nested
under `config` (which is already a group), the routes parent should be
exported (`NewConfigRoutesCmd()`) to allow test registration.

**Alternatives considered**: Single file for all route commands vs separate
files. Decision deferred to implementation, but a single
`config_routes.go` file is sufficient given the scope.
