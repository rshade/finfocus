# Implementation Plan: Serve the Web Dashboard Behind a Same-Machine HTTPS Proxy

**Branch**: `2011-web-allowed-host-trusted-proxy` | **Date**: 2026-11-08 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2011-web-allowed-host-trusted-proxy/spec.md`

## Summary

Add two opt-in flags to `finfocus --web`. `--allowed-host` (repeatable) widens
the Host and Origin checks from the two loopback names to those plus exact,
normalized proxy names, and marks the session cookie `Secure` when the proxy
reports HTTPS. `--trusted-proxy` plus `--allowed-user` lets a request that
arrives from a loopback peer, for an allowed host, with an allowed
`Tailscale-User-Login` skip the per-session token. The bind address stays
`127.0.0.1`. With neither flag, no code path changes. Amend spec 627
(FR-016, SC-006, HTTP contract) and three docs pages.

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`)
**Primary Dependencies**: standard library `net`, `net/http` only; Cobra for
flags. No new dependencies.
**Storage**: N/A
**Testing**: testify; table tests in `internal/webui/server_test.go` style and
flag-validation tests in `internal/cli/web_test.go`
**Target Platform**: Linux, macOS, Windows (pure `net/http`, no OS calls)
**Project Type**: single Go module
**Performance Goals**: none; a few string compares per request
**Constraints**: loopback-only bind (627 FR-016a); token never logged;
unchanged behavior when flags absent
**Scale/Scope**: about 2 Go files changed plus tests, 1 spec amended, 3 docs

## Constitution Check

- [x] **Plugin-First Architecture**: transport layer only; no cost or plugin logic.
- [x] **Test-Driven Development**: tests first (tasks order tests before code).
- [x] **Cross-Platform Compatibility**: no platform-specific code.
- [x] **Documentation Integrity**: guide, architecture, security docs and spec
  627 amended; `make docs-lint` run.
- [x] **Protocol Stability**: no protobuf or plugin protocol change.
- [x] **Implementation Completeness**: no stubs; both flags fully wired.
- [x] **Persistence Model**: no state added.
- [x] **Web Parity**: `Web UI Fit` is `Not applicable` with a reason (admission
  change, no view). No `internal/webui` hook for a CLI method is needed.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint` before handoff.
- [x] **Multi-Repo Coordination**: none; no finfocus-spec or plugin change.

**Violations Requiring Justification**: none. Spec 627 FR-016 is amended, not
violated; the default behavior is preserved.

## Project Structure

### Documentation (this feature)

```text
specs/2011-web-allowed-host-trusted-proxy/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/http-api.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/webui/
├── server.go            # Options, hostCheck, isOwnOrigin, authenticate, cookie
├── hosts.go             # NEW: allowed-host normalization and matching
├── server_test.go       # extend: host/origin/identity/cookie tables
└── hosts_test.go        # NEW: normalization table tests
internal/cli/
├── web.go               # flags, webCompanionFlags, validate, startup output
└── web_test.go          # flag validation
specs/627-web-ui/        # amend spec.md, contracts/http-api.md
docs/src/content/docs/{guides,architecture}/web-ui.md
docs/src/content/docs/deployment/security.md
```

**Structure Decision**: Keep the policy in `internal/webui` (it owns the
middleware) with the pure normalization in a small new file so it is
unit-testable without a server. `internal/cli` only parses flags, validates
combinations, and passes `webui.Options` fields; `internal/webui` keeps its
rule of importing no `cli`.

## Design Notes

- `Options` gains `AllowedHosts []string`, `TrustedProxy bool`,
  `AllowedUsers []string`. `New` normalizes and validates them (error wraps a
  new `ErrInvalidAllowedHost`), so both the CLI and tests get the same rule.
  The CLI maps that error through `toValidationError` (exit 2).
- `hostCheck`: loopback exact match as today, else `matchAllowedHost(r.Host)`.
- `isOwnOrigin`: loopback as today, else `https://<allowed>`; an allowed value
  with an explicit non-443 port keeps it in the origin.
- `authenticate`: after the existing token-bootstrap exemption and before the
  cookie check, `identityAuthenticated(r)` (false unless trusted-proxy on).
  A valid cookie still works in every mode.
- `handleRoot` cookie: `Secure` when `X-Forwarded-Proto` is `https`.
  Replaces the `nolint:gosec G124` comment with one that explains both cases.
- Startup (`listenAndPrintWeb`): when trusted-proxy on, print warning lines to
  stderr, then `https://<first allowed host>/` and the token URL on stdout.
  Without the flags the single stdout line is byte-identical.
