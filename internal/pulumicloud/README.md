---
title: "Pulumi Cloud client"
description: "Identity-only historical state client for a resolved Pulumi session"
layout: "docs"
---

## Responsibility

This package reads one Pulumi Cloud backend using the session discovered by
`internal/esc`. It authenticates only to that backend and follows no redirects.
Tokens come from the existing Pulumi login or `PULUMI_ACCESS_TOKEN`.

## SDK and transport

Requests go through the official `pulumi-cloud-sdk` `apiclient.CloudClient`.
Its `Executor` (`restClient.execute`) owns everything the SDK does not:

- the `Authorization: token` header, set only for the configured backend;
- the no-redirect `http.Client`, so the token never reaches another host;
- backend validation: `https`, or `http` on a loopback host only;
- up to three attempts on `429` and `503`, honoring `Retry-After`;
- the `ETag` of the first update page and `304` as `UpdateList.NotModified`;
- the 256 MiB export bound, applied before the SDK reads the body.

`serviceError` maps statuses to `ErrUnauthorized`, `ErrForbidden`,
`ErrNotFound`, and `RateLimitError`. Error text never carries the token,
userinfo, query, or a response body. Do not add a second retry or credential
rule outside the `Executor`.

## Example

```go
session, fail := esc.DiscoverSession(os.Getenv, "", "dev")
if fail != nil {
    return
}
client := pulumicloud.NewClient(session, nil)
stack := pulumicloud.StackRef{Org: session.Org, Project: "demo", Stack: "dev"}
updates, err := client.ListUpdates(ctx, stack, "")
```

`example_test.go` runs `DecodeIdentities` on an export and checks its output.

## Endpoints

- `GET /api/user`: check availability.
- `GET /api/stacks/{org}/{project}/{stack}/updates`: list update metadata with
  pages starting at 1 and `pageSize=100`.
- `GET /api/stacks/{org}/{project}/{stack}/export/{version}`: export a snapshot.

Exports are decoded into resource URN, cloud ID, type, and provider only.
Properties, inputs, outputs, and secret envelopes are never retained, logged,
or cached. Only custom resources with nonempty IDs outside `pulumi:` are kept.
The body size limit is 256 MiB. Cache persistence belongs to `internal/history`;
import orchestration belongs to `internal/cloudhistory`.
