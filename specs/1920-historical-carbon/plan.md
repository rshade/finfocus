# Implementation Plan: Historical Carbon Emissions Reporting

**Branch**: `feat/historical-carbon` | **Date**: 2026-10-28 |
**Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1920-historical-carbon/spec.md`

## Summary

Adopt the `GetCarbonEmissions` RPC from finfocus-spec v0.7.10. A small package
validates and merges pages into a provenance report. One function opens the
`aws-ce` plugin and runs that query for the CLI, the MCP tool, and the web hook.

## Technical Context

- Go 1.27.3, finfocus-spec v0.7.10 (`pluginsdk`, generated `pbc` types), Cobra,
  ax-go, gRPC, testify.
- No new dependencies. No persistent storage; nothing is cached.

## Constitution Check

- Principle II (tests first): each package has tests written with the code.
- Principle VI (no stubs): every task below is a full implementation.
- Principle VII (web parity): hook ships here; UX follow-up is #1921.
- Plugin domain knowledge stays in the plugin: core knows the capability name,
  the `aws-ce` plugin name, and the generic RPC only.

## Design

| Unit | Role |
| --- | --- |
| `internal/carbonreport/selectors.go` | Flags and JSON fields to a validated `GetCarbonEmissionsRequest` |
| `internal/carbonreport/report.go` | Page loop, record validation, report and total |
| `internal/pluginhost/carbon.go` | `Client.GetCarbonEmissions`, gated on `historical_carbon` |
| `internal/cli/carbon_actual.go` | `carbon actual` command, `queryHistoricalCarbon`, table and JSON output |
| `internal/webui/handlers_carbon.go` | `POST /api/carbon/actual/query`, error mapping |
| `internal/cli/web.go`, `internal/webui/session.go` | Inject `queryHistoricalCarbon` as `SessionOptions.HistoricalCarbon` |
| `internal/pluginupgrade/hops.go` | v0.7.10 hop and guide |

The web package does not import `cli`, `registry`, or `pluginhost`: the hook
receives the query function through `SessionOptions`.

## Dependencies

finfocus-spec v0.7.10 (released 2026-10-22). All modules pin it together; CI
compares them.

## Verification

`go build ./...`, `go vet ./...`, `make test`, the four nested-module test
targets, `make lint`, `make check-plugin-boundaries`, `make docs-lint`, and the
MCP `tools.golden` check. Real-plugin verification needs an `aws-ce` build that
implements `GetCarbonEmissions` and AWS credentials; see the pull request.
