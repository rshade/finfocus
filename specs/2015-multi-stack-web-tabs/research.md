# Research: Multi-Stack Web Dashboard

## R1 - Flag shape

- **Decision**: `StringArrayVar` for `--stack`, as `--allowed-host` does;
  `normalizeStackList` splits each value on commas, trims, drops duplicates,
  and rejects an empty element.
- **Rationale**: `StringSlice` parses `--stack=""` to an empty list, so an
  explicitly empty value would vanish before validation. `StringArray` keeps
  it, so the rule is explicit: a lone empty value is no stack (today's
  behavior), and an empty element beside another value is exit 2.
- **Alternatives**: `StringSliceVar` (loses the empty value; pflag's CSV
  parsing also treats quotes specially).

## R2 - Lazy start

- **Decision**: the server calls `StackOptions.Activate` at most once per
  stack (`sync.Once`), from `POST /api/stacks/<name>/activate`. The CLI passes
  `webStack.start`, made idempotent with a `sync.Once`.
- **Rationale**: construction of a `webStack` starts no plugins (plugins open in
  `pipe.Run`), and `stop()` already tolerates a never-started stack (spec 2013).
  An explicit call is not triggered by probes or the log viewer.
- **Alternatives**: start on first request under the prefix (any stray request
  would start plugins); start on first SSE connect (other APIs would see an
  unstarted stack).

## R3 - Load state

- **Decision**: server-computed `idle | loading | locked | ready | failed`.
  Idle until activated; then `locked` if a passphrase prompt is pending, else
  `ready` if `IsReady`, else `failed` if the pipeline has ended and the session
  holds a recorded failure, else `loading`. `Session.Failed` is that
  ended-with-failure test. `webStack.start` calls `Session.SetPipelineEnded`
  after `pipe.Run` returns.
- **Rationale**: a rejected passphrase goes through `emitError`
  (`Session.Error`) and then `OnPassphraseRequired`, in that order. The session
  keeps the failure until `Session.Ready` clears it, and submitting a
  passphrase clears the prompt flag at once. Testing "a failure is recorded"
  alone would therefore read `failed` while the retry is still loading. A
  retry never ends the pipeline (`loadWithPassphraseRetry` loops inside
  `Run`), so "pipeline ended" separates a retryable stack from a dead one. The
  session already holds these facts; the browser shows state from the server
  and computes nothing. `ready` is tested before `failed` because a failed
  on-demand preview records a failure on a pipeline that has already ended, and
  the stack that loaded still works.
- **Ordering**: the `error` event precedes `passphrase_required`, so the
  browser refreshes tab states on `error`, `passphrase_required`, `snapshot`,
  and `ready`, not on `error` alone. The `error` event also arrives while the
  pipeline is still running, so a refresh at that moment reads `loading`;
  `SetPipelineEnded` therefore publishes a `snapshot` when a failure is recorded,
  which is the signal that the failure is final.

## R3a - Failure paths

- **Decision**: `newWebStack` has no error return and starts no plugins; every
  start or run failure reaches the session through `pipe.OnError` and
  `Session.Error` with the safe label. `Activate` is `func()`.
- **Rationale**: this is how the single-stack path already reports failures
  (`bindWebPipeline`), so a failing stack needs no new channel and cannot abort
  the server.

## R4 - Request prefix

- **Decision**: `base()` keeps its meta-tag meaning. A second module-level
  active stack, set only in multi-stack mode, makes `base()` return
  `/s/<active>`. Root-only calls (`/api/stacks`) pass `{ root: true }`.
- **Rationale**: every fetch, stream, and import already goes through
  `withBase` (spec 2012), so one change covers them.

## R5 - Stream switch

- **Decision**: on a stack change, stop the stream, reset its cached snapshot,
  activate the new stack, then start the stream and mount the view.
- **Rationale**: views read cached stream state on mount; stale rows from the
  prior stack would otherwise flash.

## R6 - Single-stack parity

- **Decision**: `GET /api/stacks` returns an empty list when no stack is
  registered, and the shell enters multi-stack mode only when its page lists two
  or more stacks (R8). A page served under `/s/<name>/` (base non-empty) never
  shows tabs.
- **Rationale**: zero or one stack is byte-for-byte today's behavior.

## R8 - The page carries the stack list

- **Decision**: the root shell carries a `finfocus-stacks` meta tag naming the
  registered stacks in order, only when there are two or more
  (`(*webui.Server).stacksMeta`). The browser reads it at boot, with no request.
  `GET /api/stacks` supplies the live states afterwards.
- **Rationale**: the first design fetched `/api/stacks` before the shell
  started, through a top-level `await`. A hung or slow server would then block
  the whole page, and the existing harnesses, which fake `fetch` as a promise
  that never settles, hung. The base prefix already reaches the page through a
  meta tag for the same reason (the CSP forbids inline script). Boot stays
  synchronous, and a page with no tag is the single-stack page unchanged.
- **Alternatives**: boot as a single-stack page and switch when the fetch
  answers (flashes the wrong page and opens a useless stream on the empty root
  session).

## R7 - Header label scope

- **Decision**: the stack label appears in multi-stack mode only.
- **Rationale**: FR-013 requires single-stack pages to stay unchanged.
