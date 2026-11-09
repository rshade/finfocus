# HTTP Contract Delta: allowed hosts and trusted proxy

This amends `specs/627-web-ui/contracts/http-api.md`. Routes, bodies, and
status codes of every endpoint are unchanged. Only admission changes. This
feature adds no endpoint and calls no CLI Go method (Web UI Fit: Not
applicable).

## Host header

| Server flags | Accepted `Host` |
| --- | --- |
| none | `127.0.0.1:<port>`, `localhost:<port>` |
| `--allowed-host H[:p]` | the above, plus normalized `H` (exact). No port in the flag: request port absent or `443`. Port in the flag: that port only. |

Anything else: `403 {"error":"host header is not this server","code":"forbidden"}`.

## Origin header

Accepted: `http://127.0.0.1:<port>`, `http://localhost:<port>`, and
`https://H` (`https://H:p` when the flag carries a port other than 443). The
host part matches like `Host` (case-insensitive, trailing dot allowed, since a
browser keeps it in the origin). An explicit `:443`, userinfo, a path, a query,
or a fragment is refused.
Anything else: `403 forbidden`.

## Cookie

`Set-Cookie: finfocus_session_<port>=<token>; HttpOnly; SameSite=Strict; Path=/`
gains `Secure` when an allowed host is configured and `X-Forwarded-Proto` is
`https`. With no allowed host the header is ignored.

## Authentication with `--trusted-proxy`

A request is authenticated without the cookie when all hold: peer is
loopback; `Host` is an allowed host; `Tailscale-User-Login` appears exactly
once and equals (case-insensitive) an `--allowed-user`. Otherwise the cookie
rules apply: `401 unauthorized`. No cookie is issued for identity requests;
Origin, `Sec-Fetch-Site`, and Content-Type checks still run.

Without `--trusted-proxy` the header is ignored.
