# Feature Specification: Serve the Web Dashboard Behind a Same-Machine HTTPS Proxy

**Feature Branch**: `2011-web-allowed-host-trusted-proxy`
**Created**: 2026-11-08
**Status**: Draft
**Input**: GitHub issue rshade/finfocus#2036 — "web: add --allowed-host and
--trusted-proxy for serving behind tailscale serve"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reach the dashboard through an HTTPS proxy name (Priority: P1)

A user runs `finfocus --web` on a workstation or server and publishes it with
`tailscale serve` (or another HTTPS reverse proxy on the same machine). The
proxy forwards `https://<machine>.<tailnet>.ts.net` to the dashboard. Today
the dashboard rejects every such request because its host and origin checks
accept only its own loopback names. The user names the public host with
`--allowed-host` and opens the printed proxy URL.

**Why this priority**: Without it nothing else works behind a proxy. It is
also useful on its own: the existing per-session token flow keeps working
through the proxy, so no identity feature is needed to reach the dashboard.

**Independent Test**: Start the server with one allowed host, send requests
whose `Host` and `Origin` name that host over HTTPS, and confirm they reach
the dashboard. Send the same requests with a different host and confirm 403.

**Acceptance Scenarios**:

1. **Given** the server runs with `--allowed-host dash.example.ts.net`,
   **When** a request arrives with `Host: dash.example.ts.net`, **Then** the
   host check passes and the normal token and cookie rules apply.
2. **Given** the same server, **When** a state-changing request carries
   `Origin: https://dash.example.ts.net`, **Then** the origin check passes.
3. **Given** the same server, **When** a request arrives with a `Host` that
   is neither loopback nor in the allowed set, **Then** it is refused with
   403, as today.
4. **Given** the proxy marks the request as HTTPS, **When** the token
   bootstrap sets the session cookie, **Then** the cookie is marked `Secure`;
   over plain loopback HTTP it is not, as today.
5. **Given** no `--allowed-host` is passed, **When** any request arrives,
   **Then** behavior is identical to today.

---

### User Story 2 - Sign in with the proxy's tailnet identity (Priority: P2)

A user who serves the dashboard on their tailnet does not want to paste a
token into every browser. They start with `--trusted-proxy`, an allowed host,
and the tailnet logins allowed to use the dashboard. A request that arrives
through the proxy carrying an allowed login is accepted without the session
token.

**Why this priority**: It removes the need for a token-injecting proxy
script, but widens trust, so it is opt-in and layered on User Story 1.

**Independent Test**: Start with `--trusted-proxy`, one allowed host and one
allowed user. Send a request from the loopback peer, with the allowed host
and the allowed login, and no cookie: it is served. Change any one of those
conditions and it is refused with 401 or 403.

**Acceptance Scenarios**:

1. **Given** trusted-proxy mode with allowed host H and allowed user U,
   **When** a loopback peer sends a request for host H with login U and no
   session cookie, **Then** the request is authenticated.
2. **Given** the same server, **When** the login is empty, or names someone
   not in the allowed users, **Then** the request falls back to the token
   rules and is refused with 401 without a valid cookie.
3. **Given** the same server, **When** the request arrives from a
   non-loopback peer, **Then** the identity header is ignored.
4. **Given** trusted-proxy mode is off, **When** a request carries an identity
   header, **Then** the header is ignored.
5. **Given** the server is started with `--trusted-proxy` but no allowed host
   or no allowed user, **Then** startup fails with a validation error (exit 2)
   and no server starts.
6. **Given** trusted-proxy mode starts successfully, **Then** the terminal
   shows a loud warning that any local process can reach the loopback port
   and forge the identity header, plus both URL forms (the proxy URL and the
   token URL).

---

### User Story 3 - Know the security model from the docs (Priority: P3)

An operator reads the web UI guide and the security deployment page and
learns what each option protects against, what it does not, and how
Tailscale Serve differs from Funnel.

**Why this priority**: The change relaxes a documented guarantee; the docs
and the web UI specification must say so, or the stated guarantees become
false.

**Independent Test**: Read the amended spec requirements, HTTP contract, and
the three docs pages; each states the new options and the "local only"
limitation text is no longer unconditional.

**Acceptance Scenarios**:

1. **Given** the amended docs, **When** a reader looks for how to serve behind
   a proxy, **Then** they find the flags, an example, the Serve versus
   Funnel difference, and the local-process spoofing risk.

### Edge Cases

- `--allowed-host` or `--allowed-user` without `--web`: rejected like every
  other web companion flag.
- An allowed host that is a wildcard (`*.ts.net`), an IP literal (IPv4 or
  IPv6, bracketed or not), empty, or contains a scheme, path, or userinfo:
  rejected at startup.
- Mixed case, a trailing dot, or an explicit `:443` port on the flag value or
  the incoming `Host`: matched case-insensitively; a port is optional.
- A host that matches the allowed name only as a suffix or prefix
  (`evil.dash.example.ts.net`, `dash.example.ts.net.evil.com`): refused.
- Several `--allowed-host` values: any one matches.
- `X-Forwarded-Proto` supplied by a client directly on a loopback request:
  ignored without an allowed host; with one, it only sets the cookie's
  `Secure` attribute and never grants access.
- Tailscale Funnel traffic and tagged devices carry no identity header:
  trusted-proxy mode never authenticates them; the token path still works.
- The identity header is repeated or differs in case from the allowed login:
  logins are compared case-insensitively; a repeated header is refused.
- The token URL printed at startup keeps working in every mode.
- Server-sent event streams keep working through the proxy (to be confirmed
  live, as the issue records it as unverified).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `--allowed-host <name>` MUST be accepted any number of times,
  MUST be valid only with `--web`, and MUST lowercase and normalize each value
  (trailing dot removed). A port is optional and, when present, MUST be a
  valid port.
- **FR-002**: Startup MUST fail with a validation error (exit 2) for an
  allowed host that is empty, contains a wildcard, is an IP literal, or is not
  a plain host name with an optional port.
- **FR-003**: The host check MUST accept the loopback names on the server's
  own port (unchanged) and any allowed host. Matching MUST be exact on the
  normalized name; a port in the allowed value MUST then match exactly, and
  one omitted MUST match the request with no port or with the HTTPS default.
  Every other host MUST be refused with 403.
- **FR-004**: The origin check MUST accept the loopback origins (unchanged)
  and `https://<allowed-host>`. Every other origin MUST be refused.
- **FR-005**: With at least one allowed host, the session cookie MUST carry
  `Secure` when the proxy marks the request HTTPS with `X-Forwarded-Proto:
  https` (first value) and MUST NOT otherwise. With no allowed host the header
  is ignored, so the cookie is exactly as before. The header only tightens
  the cookie; it never grants access.
- **FR-006**: The listen address MUST stay loopback only (spec 627 FR-016a).
- **FR-007**: `--trusted-proxy` MUST be opt-in, MUST be valid only with
  `--web`, and MUST require at least one `--allowed-host` and at least one
  `--allowed-user <login>`; otherwise startup fails with exit 2. A blank
  `--allowed-user` is refused, and `--allowed-user` without `--trusted-proxy`
  is refused, so a user list is never silently ignored.
- **FR-008**: With trusted-proxy on, a request MUST be authenticated without
  the session token only when all of these hold: the peer address is
  loopback; the host is an allowed host; the `Tailscale-User-Login` header is
  present exactly once, non-empty, and equal (case-insensitive) to an allowed
  user. Otherwise the existing token rules decide.
- **FR-009**: Identity authentication MUST apply per request; it MUST NOT
  create a session cookie, and MUST NOT bypass the host check, the origin
  check, or the content-type and fetch-metadata hardening.
- **FR-010**: With trusted-proxy off, identity headers MUST be ignored and
  every code path MUST behave identically to the current release.
- **FR-011**: When trusted-proxy starts, the terminal MUST show a warning that
  identity-only mode removes the token barrier against other local processes,
  and MUST print both the proxy URL for the first allowed host and the token
  URL.
- **FR-012**: The token MUST stay out of logs, and identity values MUST NOT
  be logged beyond what the existing request log records.
- **FR-013**: Spec 627 FR-016 and SC-006, the HTTP contract, the web UI guide
  and architecture page, and the security deployment page MUST be amended to
  describe the optional allowed hosts and trusted-proxy mode.

### Key Entities *(include if feature involves data)*

- **Allowed host**: A normalized DNS name, with an optional port, that the
  server accepts as its public name.
- **Allowed user**: A tailnet login permitted to use the dashboard without the
  token in trusted-proxy mode.
- **Proxy-supplied identity**: The `Tailscale-User-Login` header, trusted only
  under the FR-008 conditions.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable - this changes how requests are admitted
  (host, origin, cookie, identity). It adds no view and no data to a view.
- **Web data**: None.
- **Layout fit**: None; the dashboard layout is unchanged.
- **Additions**: none
- **Render evidence**: Not applicable. Existing views are unchanged; an
  end-to-end request through the new host path covers the behavior.
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work. No follow-up
  issue is filed, because there is no web surface.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with a proxy in front of the dashboard reaches it at the
  proxy's HTTPS name, with the printed URL, in one step and with no
  token-injecting script.
- **SC-002**: 100% of automated cases with a foreign host, a foreign origin,
  a near-miss host (prefix, suffix, wildcard, IP literal), or a missing token
  are refused, with and without the new options.
- **SC-003**: 100% of automated cases that violate any single identity
  condition (non-loopback peer, trusted-proxy off, host not allowed, empty,
  repeated, or unlisted login) are refused without a valid token.
- **SC-004**: With none of the new options set, the existing web test suite
  passes unchanged and responses are identical to the current release.
- **SC-005**: Starting with the new options in an invalid combination fails
  before the server binds, 100% of the time, with exit code 2.
- **SC-006**: An operator can find, in the docs, the proxy setup, the
  Serve-versus-Funnel difference, and the spoofing warning without reading
  source code.

## Assumptions

- The proxy runs on the same machine and connects to the dashboard over
  loopback; Tailscale Serve strips client-supplied identity headers and sets
  its own for tailnet traffic only.
- Whether Serve forwards the tailnet `Host` unchanged, and whether event
  streams survive it, is verified live and recorded in the plan; if `Host` is
  rewritten the user lists the rewritten name.
- Requiring `--allowed-user` for trusted-proxy is the decision recorded on the
  issue, so identity-only mode never admits a whole tailnet implicitly.
- The multi-stack dashboard and later steps of the issue's sequence are out of
  scope.
