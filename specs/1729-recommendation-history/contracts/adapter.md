# Contract: Recommendation history adapter

The core client adapter exposes `GetRecommendationHistory` on
`CostSourceClient`. Callers pass a context and a recommendation id. The
adapter does not add credentials, tags, or any field other than that id.

## Request

- Empty or missing id: the adapter returns `InvalidArgument` and does not
  call the plugin.
- The context is the context passed to the plugin call.

## Response

- Each plugin event keeps action, reason, note, actor, time, and expiry.
- A missing time or expiry stays empty.
- A nil event, or an event whose time or expiry is present but invalid, is
  skipped. Sibling events are returned.
- An empty event list is success.
- A nil RPC response is an error.
- Plugin status codes are wrapped and remain visible to `status.Code`,
  including `Unimplemented`.

## Callers that are not plugins

Test doubles that do not care about history return `Unimplemented`. The engine
treats that as no plugin events.
