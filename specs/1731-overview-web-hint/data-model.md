# Data Model: Overview Web Hint

The feature stores nothing and sends nothing to plugins. Its state is two
in-memory values in the existing overview model and the hand-off value built
from existing parameters.

## OverviewModel (existing, `internal/tui`)

| Field | Type | Meaning |
| --- | --- | --- |
| `webRequested` | `bool` | Set when `w` is pressed in the list view. Read after the program exits. |

- **Accessor**: `WebRequested() bool`, next to `Err()`.
- **Transitions**: false until the list view handles `w`; then true and the
  model enters `ViewStateQuitting` and returns `tea.Quit`. It is never reset,
  because the program ends.
- **Validation**: set only from `handleListKeypress`, which is unreachable
  while the filter input, passphrase prompt, or detail view has focus.

## Footer hint (derived, not stored)

Computed at render time from `m.width`:

| Width available | Text |
| --- | --- |
| Long form fits | `[w] web dashboard · finfocus --web` |
| Only short form fits | `[w] web` |
| Neither fits | line omitted |

The line is always at most the terminal width, so it never wraps.

## Web hand-off (existing type, `internal/cli`)

`webFlagValues` built from `overviewParams`:

| `webFlagValues` | From `overviewParams` |
| --- | --- |
| `web` | `true` |
| `pulumiJSON` | `pulumiJSON` |
| `pulumiState` | `pulumiState` |
| `stack` | `stack` |
| `from`, `to` | `fromStr`, `toStr` |
| `adapter` | `adapter` |
| `filter` | `filter` |
| `port` | `0` |
| `noBrowser` | `false` |

No new persisted state, config key, or environment variable.
