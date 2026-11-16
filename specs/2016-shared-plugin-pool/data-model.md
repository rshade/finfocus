# Data Model: Shared Plugin Processes Across Web Stacks

All state is in memory, inside one `finfocus --web` process.

## Pool

| Field | Meaning |
| --- | --- |
| `ctx`, `cancel` | Owner lifetime. Cancelling kills any launch still in progress. |
| `entries` | `map[poolKey]*poolEntry`, the live entries. |
| `closed` | Set by `Close`; later `Acquire` calls fail with `ErrPoolClosed`. |
| `advertised` | `GetPluginInfo` settings per `binaryID`, learned from the first launch that answered. |
| `learning` | Per `binaryID`, closed when the launch that will teach its settings ends or is abandoned. |

## poolKey

| Field | Meaning |
| --- | --- |
| `path` | Plugin binary path. A new version is a different key. |
| `binary` | `binaryID`: path plus size and modification time, so a same-version reinstall is a different key. |
| `kind` | `offerKey` before the binary's settings are known, `finalKey` after. |
| `env` | Offer key: hex SHA-256 of the launch pairs, override names, and the stack's whole ESC offer (value, reference, revision). Final key: hex SHA-256 of the final pairs, override names, and delivered digest. |

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
