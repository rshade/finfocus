# Data Model: Shared Plugin Processes Across Web Stacks

All state is in memory, inside one `finfocus --web` process.

## Pool

| Field | Meaning |
| --- | --- |
| `ctx`, `cancel` | Owner lifetime. Cancelling kills any launch still in progress. |
| `entries` | `map[poolKey]*poolEntry`, the live entries. |
| `closed` | Set by `Close`; later `Acquire` calls fail with `ErrPoolClosed`. |

## poolKey

| Field | Meaning |
| --- | --- |
| `path` | Plugin binary path. A new version is a different key. |
| `binary` | Size and modification time of the file, so a same-version reinstall is a different key. |
| `env` | Hex SHA-256 of the launch pairs, override names, and the stack's ESC offer (value, reference, revision) for the plugin. |

## poolEntry

| Field | Meaning |
| --- | --- |
| `ready` | Closed when the launch finishes (success or failure). |
| `client` | The launched client; its `Close` stops the process. |
| `err` | Launch error, shared with every waiter. |
| `holds` | Requesters waiting plus holders that have not released. |
| `stop` | Cancels the entry's launch context. |

### State transitions

```text
launching --(launch ok, holds > 0)--> running --(last release)--> closed
launching --(launch ok, holds = 0 or pool closed)--> closed
launching --(launch error)--> failed (removed; waiters get err)
launching --(every requester gave up)--> abandoned (removed from lookup; closed when the launch ends)
running --(Name probe fails on acquire)--> evicted (removed; closed on last release)
running --(Pool.Close)--> closed
```
