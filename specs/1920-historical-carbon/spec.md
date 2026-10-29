# Feature Specification: Historical Carbon Emissions Reporting

**Feature Branch**: `feat/historical-carbon`
**Created**: 2026-10-28
**Status**: Implemented; verification in the pull request
**Input**: GitHub Issue #1920 - report provider-published historical carbon
emissions (finfocus-spec v0.7.10)

## Intent and Scope

Report the monthly emissions inventory a provider has published, with full
provenance, from the command line, the MCP tool surface, and the web server's
domain hook. This is measured data at account, region, and service level. It is
separate from the projected, per-resource estimates of `specs/631-carbon-plugin`
and never mixes with them.

Core stays provider-agnostic. The `aws-ce` plugin owns every AWS call; core
gets the generic `GetCarbonEmissions` channel from finfocus-spec v0.7.10 and a
`historical_carbon` capability gate.

## User Scenarios & Testing

### User Story 1 - Query Available Monthly Emissions (Priority: P1)

A sustainability owner asks for the emissions the provider reported for a range
of complete months.

**Independent Test**: Run `finfocus carbon actual --start 2026-01-01 --end
2026-03-01 --output json` against a recording plugin and compare each record
with the plugin's response.

**Acceptance Scenarios**:

1. **Given** a plugin advertising `historical_carbon`, **When** the owner queries
   two months, **Then** the report lists every returned record with provider,
   source, account, region, service, original period, method, scope, model
   version, retrieval time, and `gCO2e`.
2. **Given** a provider that returns an explicit zero, **When** the report is
   built, **Then** the record keeps `value: 0` and the total is numeric.
3. **Given** an empty provider answer, **When** the report is built, **Then**
   `records` is empty, `totalGrams` is null, and the resolved request is shown.

### User Story 2 - Refuse Unsafe Or Invalid Queries (Priority: P1)

**Independent Test**: Run the command with a mid-month date, then against a
plugin that does not advertise the capability.

**Acceptance Scenarios**:

1. **Given** a start or end that is not a first-of-month UTC date, **When** the
   command runs, **Then** it exits 2 with a `validation_error` and calls no
   plugin.
2. **Given** a plugin without `historical_carbon`, **When** the command runs,
   **Then** it fails with guidance to enable the setting and no RPC is sent.
3. **Given** a repeated page token, a duplicate record identity, mixed methods
   or scopes, a non-finite value, or more than 100 pages, **When** pages are
   collected, **Then** the query fails and writes no partial report.

### User Story 3 - Agents And The Web Hook Get The Same Report (Priority: P2)

**Independent Test**: List MCP tools and call `POST /api/carbon/actual/query`
with a session; compare the body with the CLI JSON.

**Acceptance Scenarios**:

1. **Given** the MCP server, **When** tools are listed, **Then**
   `finfocus-carbon-actual` appears as read-only and listing it starts no plugin.
2. **Given** an authenticated web session, **When** the hook is posted, **Then**
   the response is the CLI's JSON report with no browser arithmetic.
3. **Given** a provider failure, **When** the hook answers, **Then** it returns
   a stable error code and a fixed safe message, never the provider's text.

## Requirements

- **FR-001**: Core MUST pin finfocus-spec v0.7.10 in the root module, every
  nested module, and `test/e2e/kind/fixedcost`.
- **FR-002**: The CLI, the MCP tool, and the web hook MUST call
  `queryHistoricalCarbon`, which opens only the `aws-ce` plugin.
- **FR-003**: Requests MUST pass `pluginsdk.ValidateGetCarbonEmissionsRequest`
  and use inclusive-start, exclusive-end, first-of-month UTC boundaries.
- **FR-004**: Core MUST call `GetCarbonEmissions` only when the plugin
  advertises `historical_carbon`.
- **FR-005**: The total MUST sum returned records only. Missing data MUST NOT
  be reported as zero.
- **FR-006**: Invalid user input MUST exit 2 through `toValidationError`.
- **FR-007**: Web errors MUST be stable codes with fixed safe messages.
- **FR-008**: A plugin-upgrade hop and guide for v0.7.10 MUST exist.
- **FR-009**: The report MUST NOT allocate monthly inventory to resource rows.

## Web UI Fit

- **Disposition**: Hook only. This change adds the authenticated
  `POST /api/carbon/actual/query` domain hook, which calls the same Go method
  as the command line (`queryHistoricalCarbon`). No view ships here.
- **Web data**: The same JSON as `finfocus carbon actual --output json`.
- **Follow-up**: One UX-only `web-ui` issue, #1921, for the query form and
  provenance table. It holds no engine, API, or data-shape work.
- Constitution Principle VII: the hook and its contract are recorded here and
  in `contracts/carbon-actual.md`; the feature is complete for the web once
  #1921 closes.

## Out of Scope

- The `aws-ce` plugin's `GetCarbonEmissions` implementation.
- Allocating monthly inventory to resource cost rows (FR-009).
- Other providers; the command queries `aws-ce` only.
- Projected estimates (`specs/631-carbon-plugin`).
