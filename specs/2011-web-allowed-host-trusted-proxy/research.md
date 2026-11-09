# Research: Serve the Web Dashboard Behind a Same-Machine HTTPS Proxy

## Decisions

### D1 - Where the policy lives

- **Decision**: Normalize and validate in `webui.New`; the CLI passes raw flag
  values and maps the error to exit 2.
- **Rationale**: One definition shared by CLI and tests; `webui` already owns
  Host and Origin policy.
- **Alternatives**: Validate in the CLI only (tests of `webui` could then build
  an invalid server); rejected.

### D2 - Host matching

- **Decision**: Lowercase, strip one trailing dot, split optional port with
  a single colon. Reject empty, `*`, `/`, `@`, more than one colon,
  bracketed or parsed IP literals (`net.ParseIP`), a last label that starts
  with a digit (`127.1`, `0x7f.0x1`), and a name that is not letters,
  digits, `-`, `.`. Compare the normalized request host exactly. An
  allowed value without a port matches a request with no port or `:443`; an
  allowed value with a port matches that port only.
- **Rationale**: Exact match is the DNS-rebinding defense; suffix or wildcard
  matching was explicitly rejected on the issue.
- **Alternatives**: wildcard allowlist (rejected: rebinding risk); accept IPs
  (rejected: issue).

### D3 - Secure cookie signal

- **Decision**: `X-Forwarded-Proto: https` (first value) sets `Secure`, read
  only when an allowed host is configured so the no-flag cookie is unchanged.
- **Rationale**: Tailscale Serve and common proxies send it. A forged header
  only tightens the cookie, never grants access.
- **Alternatives**: `r.TLS` (always nil here, since the server speaks plain HTTP).

### D4 - Identity auth scope

- **Decision**: Per request, no cookie issued, no bypass of Host, Origin,
  Sec-Fetch-Site, or Content-Type checks. Peer is checked with
  `net.SplitHostPort(r.RemoteAddr)` and `IP.IsLoopback()`.
- **Rationale**: Minimal trust widening; a login header is meaningful only
  from the local proxy. The Host check already ran, so an allowed host is
  required by construction; identity additionally requires it explicitly.
- **Alternatives**: mint a session cookie after identity (rejected: persists
  trust past the allowed-user list changing and past the proxy).

### D5 - Header handling

- **Decision**: Require exactly one `Tailscale-User-Login` value
  (`len(r.Header.Values) == 1`), compare with `strings.EqualFold`
  against the allowed users, constant-time not required (login is not a secret).
- **Rationale**: A repeated header is ambiguous between proxy and client.

## Unverified live (Tailscale Serve)

Not tested in this change; the implementation does not depend on either
answer, and docs must say so rather than claim behavior.

- Whether `tailscale serve` forwards the tailnet `Host` unchanged. If it
  rewrites to the backend address, the user must list the rewritten name or
  the host check refuses it.
- Whether SSE streams (`/api/overview/stream`) survive Serve without
  buffering.

## Known Tailscale facts used (from the issue)

- Serve adds `Tailscale-User-Login`, `-Name`, `-Profile-Pic` for tailnet
  traffic only; Funnel traffic and tagged devices have none; Serve strips
  client-supplied copies.
