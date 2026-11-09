# Tasks: Serve the Web Dashboard Behind a Same-Machine HTTPS Proxy

**Input**: Design documents in `specs/2011-web-allowed-host-trusted-proxy/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http-api.md

Tests come first in every phase (Constitution Principle II). Web UI Fit is
`Not applicable`, so there is no web-parity phase and no `web-ui` follow-up.
Paths are relative to the repository root.

## Phase 1: Setup

- [X] T001 Confirm baseline: run `go test ./internal/webui/... ./internal/cli/ -run 'Web|Host|Origin'` and record that it passes before any change

## Phase 2: Foundational (blocks all stories)

- [X] T002 [P] Write table tests for host normalization and request matching in `internal/webui/hosts_test.go`: lowercase, trailing dot, optional port, `:443` default, exact match only; rejects empty, wildcard, IPv4, IPv6 (bracketed and bare), scheme, path, userinfo, bad port, prefix and suffix near-misses
- [X] T003 Implement `internal/webui/hosts.go`: `ErrInvalidAllowedHost`, `normalizeAllowedHosts([]string)`, `allowedHostSet.matchHost(string)` and `.originFor` helpers per research D2
- [X] T004 Add `AllowedHosts []string`, `TrustedProxy bool`, `AllowedUsers []string` to `Options` in `internal/webui/server.go`; normalize and validate in `New` (also `ErrInvalidTrustedProxy` when trusted-proxy lacks hosts or users); store the derived sets on `Server`

**Checkpoint**: `go test ./internal/webui/ -run Host` passes; existing tests unchanged.

## Phase 3: User Story 1 - Reach the dashboard through an HTTPS proxy name (P1)

**Goal**: allowed hosts pass Host and Origin checks; cookie is `Secure` behind HTTPS.
**Independent test**: server with one allowed host serves it and refuses a foreign host.

- [X] T005 [P] [US1] Extend `TestHostHeaderValidation` in `internal/webui/server_proxy_test.go` with a server built with `AllowedHosts`: allowed host (bare, `:443`, mixed case, trailing dot) returns 200; prefix, suffix, other host return 403; default server still refuses the allowed name
- [X] T006 [P] [US1] Add origin tests in `internal/webui/server_test.go`: POST with `Origin: https://<allowed>` passes; `http://<allowed>` and foreign origins return 403; GET with foreign origin still 403
- [X] T007 [P] [US1] Add cookie test in `internal/webui/server_test.go`: `GET /?token=` with `X-Forwarded-Proto: https` sets `Secure`; without it the cookie has no `Secure`; header never bypasses the token check
- [X] T008 [US1] Update `hostCheck` and `isOwnOrigin` in `internal/webui/server.go` to accept the allowed set (loopback logic untouched)
- [X] T009 [US1] Set `Secure` on the session cookie in `handleRoot` when `X-Forwarded-Proto` is `https`; replace the `nolint:gosec G124` justification so it covers both cases
- [X] T010 [P] [US1] Write flag tests in `internal/cli/web_test.go`: `--allowed-host` listed in `webCompanionArgs` (rejected without `--web`, exit 2), repeatable, invalid values (wildcard, IP, empty) exit 2 before binding
- [X] T011 [US1] In `internal/cli/web.go` add `AllowedHosts` to `webFlagValues`, register `--allowed-host` (`StringSliceVar`, "only valid with --web"), add it to `webCompanionFlags`, pass it in `listenAndPrintWeb`, and map `webui.ErrInvalidAllowedHost` through `toValidationError`

**Checkpoint**: US1 works alone with the token flow behind a proxy.

## Phase 4: User Story 2 - Sign in with the proxy's tailnet identity (P2)

**Goal**: opt-in identity admission under the FR-008 conditions.
**Independent test**: all four conditions hold: served; any one fails: 401/403.

- [X] T012 [P] [US2] Add identity table tests in `internal/webui/server_test.go` using a custom `RemoteAddr` (httptest handler call, since a real peer is loopback): accepted case; non-loopback peer; trusted-proxy off; unlisted login; empty login; repeated header; different case login accepted; host not allowed; no cookie issued; Origin and Content-Type checks still apply on POST
- [X] T013 [US2] Implement `identityAuthenticated(r)` in `internal/webui/server.go` (loopback peer via `net.SplitHostPort`+`IsLoopback`, allowed host, single non-empty header, `strings.EqualFold` against allowed users) and call it in `authenticate` after the token-bootstrap exemption and before the cookie check
- [X] T014 [P] [US2] Add flag tests in `internal/cli/web_test.go`: `--trusted-proxy` and `--allowed-user` rejected without `--web`; `--trusted-proxy` without `--allowed-host` or without `--allowed-user` exits 2; valid combination starts, prints the warning on stderr and both URLs; output without the flags is byte-identical to today
- [X] T015 [US2] In `internal/cli/web.go` add `trustedProxy`, `allowedUsers` fields, register `--trusted-proxy` and `--allowed-user`, add both to `webCompanionFlags`, pass them to `webui.Options`, map `webui.ErrInvalidTrustedProxy` through `toValidationError`
- [X] T016 [US2] In `listenAndPrintWeb` (`internal/cli/web.go`) print the spoofing warning to stderr and the `https://<first allowed host>/` URL plus the token URL to stdout when trusted-proxy is on; leave the single-line output untouched otherwise

**Checkpoint**: US1 and US2 independently verified.

## Phase 5: User Story 3 - Know the security model from the docs (P3)

- [X] T017 [P] [US3] Amend `specs/627-web-ui/spec.md` FR-016 and SC-006 to describe optional allowed hosts and trusted-proxy while keeping FR-016a unchanged
- [X] T018 [P] [US3] Amend `specs/627-web-ui/contracts/http-api.md` (Host, Origin, cookie `Secure`, identity admission) and cross-link `specs/2011-web-allowed-host-trusted-proxy/contracts/http-api.md`
- [X] T019 [P] [US3] Update `docs/src/content/docs/guides/web-ui.md`: replace the unconditional "Local only" limitation, add a "Serve behind Tailscale Serve" section with the flags, example, Serve versus Funnel, and the Host/SSE behavior stated as not yet verified live
- [X] T020 [P] [US3] Update `docs/src/content/docs/architecture/web-ui.md`: Secure-cookie rationale at the former "TLS rejected" paragraph, plus the admission order (host, log, authenticate with identity, hardening)
- [X] T021 [P] [US3] Update `docs/src/content/docs/deployment/security.md`: allowed host exact-match rule, trusted-proxy spoofing risk from any local process, required `--allowed-user`
- [X] T022 [US3] Add the new flags to the `--web` flag reference in `docs/src/content/docs/` (grep for `--no-browser` to find the table or page) and to the `CLAUDE.md` web gotchas only if a non-obvious rule emerged

## Phase 6: Polish and gates

- [ ] T023 (not run in this change; left open on purpose, it publishes to a tailnet) Live check if Tailscale is available: run `finfocus --web` behind `tailscale serve`, record whether `Host` is forwarded unchanged and whether SSE streams survive; update `research.md` and the docs wording to match (otherwise leave as unverified and say so in the PR)
- [X] T024 Run `go test -race -shuffle=on -count=3 ./internal/webui/... ./internal/cli/` for parallel-safety
- [X] T025 Run `make validate`, `make test`, `make lint` (extended timeout), `make docs-lint`, and `npx markdownlint-cli2` on changed Markdown outside `docs/`
- [X] T026 Run `./bin/finfocus --web --no-browser` with and without the new flags from `../finfocus-demo` and confirm default output unchanged (`make build` first)

## Dependencies

- Phase 2 blocks everything. US1 (T005-T011) then US2 (T012-T016, uses the host set from US1). US3 docs can start after the contract is stable (T018 after T013).
- Within a phase, tests precede implementation; `[P]` tasks touch different files.

## Parallel example (US1)

T005, T006, T007, T010 touch different test sections or files and can be drafted together before T008, T009, T011.

## Strategy

MVP is US1 (token flow behind a proxy). US2 layers identity on top. US3 lands with the code so docs never lag the flags.
