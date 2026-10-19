# Research: Recommendation History

## Decision: Call every connected plugin

**Rationale**: finfocus-spec v0.7.7 does not add a capability enum for this
RPC. Plugins that do not implement `RecommendationHistoryProvider` return
`Unimplemented`. Core treats that status as an empty contribution.

**Alternatives considered**: Gate the call on a new capability string. Rejected
because the issue says no capability enum is required, and a host-side table
would drift from the plugin.

## Decision: Actor is an optional field on the existing lifecycle event

**Rationale**: Local files stay readable without a migration when the JSON
field is omitted. Plugin action and reason strings are copied as text so a
plugin-specific value is not forced into `dismissed`, `snoozed`, or
`undismissed`.

**Alternatives considered**: A second event type for plugin rows. Rejected
because table, JSON, and NDJSON already render one event shape.

## Decision: Match ignores actor; named actors win over a blank one

**Rationale**: A local dismiss and the plugin copy of that dismiss are the
same change. The plugin copy is the one that can name a person or machine.
Two different actors are two records and both stay.

**Alternatives considered**: Fuzzy time matching. Rejected because a different
instant is a different lifecycle transition, and the issue says distinct
transitions must not be dropped.

## Decision: Missing times sort last; unreadable times drop that event only

**Rationale**: A missing timestamp is valid and must not panic. Sorting it
after dated events keeps the dated story in order. A timestamp or expiry that
fails validation is malformed. Skipping that event leaves every other event,
including the local record, in the result. The RPC error is reserved for a
failed call, not for one bad row.

**Alternatives considered**: Fail the plugin call when any event is bad.
Rejected because that would hide the valid rows from that plugin and, if the
engine then discarded the response, would make one bad event erase plugin
history that was otherwise usable.

## Decision: Cancellation stops the command; other plugin failures do not

**Rationale**: `Canceled` and `DeadlineExceeded`, and a caller context that is
already done, abort the merge. `Unimplemented`, an empty body, a transport
failure, and any other status are logged and the remaining sources are kept.
Opening plugins uses the existing registry helper. If that open fails, the
command logs the error and continues with the local record. Connections that
were opened are closed before the command returns.

**Alternatives considered**: Fail the command on the first plugin error.
Rejected because local history must stay usable when a plugin is down.

## Decision: Do not write plugin events back to the dismissal file

**Rationale**: The issue says a history query must not mutate the store.
Plugin events exist only in the command output. A later local dismiss still
appends to the local record the way it does today.

**Alternatives considered**: Persist merged events. Rejected because it would
mix plugin actors into the local audit file and change dismiss behavior.
