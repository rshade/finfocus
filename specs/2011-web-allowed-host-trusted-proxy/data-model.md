# Data Model: Allowed Hosts and Trusted Proxy

In-memory configuration only; nothing is persisted.

## `webui.Options` additions

| Field | Type | Rule |
| --- | --- | --- |
| `AllowedHosts` | `[]string` | Each normalized (lowercase, no trailing dot); optional port; no wildcard, IP literal, scheme, path, or userinfo. Duplicates collapse. |
| `TrustedProxy` | `bool` | Requires at least one allowed host and one allowed user. |
| `AllowedUsers` | `[]string` | Non-empty, trimmed, compared case-insensitively. |

## Derived server state

- `allowedHosts`: set of `{name, port}` where port is empty or `1-65535`.
- `allowedUsers`: lowercase set.

## Validation outcomes

- Invalid host, or `TrustedProxy` without hosts or users: `New` returns an
  error wrapping `ErrInvalidAllowedHost` or `ErrInvalidTrustedProxy`; the CLI
  reports exit 2 before binding.
