---
title: "Clef compatibility spike for issue 1728"
description: "Recorded-response evidence and limits before the scorer backend refactor"
layout: "docs"
---

## Result

On 2026-10-18, the existing FinFocus `jevapi.Client` decoded recorded Clef
`noul` and `score` values through go-decide's released adapter without changing
the production client or its request/response types. A base URL change alone
does not work. The complete recording fails because `jevapi.Response.validate`
rejects `choice`. The current scorer only constructs `noul` and `score`
questions; the issue's description of it sending `choice` needs correction.

This is a replay result, ready for review before tasks 2–6. No live call was
made, no credentials were accessed, and no production configuration, plugin
name, registry entry, or release metadata changed.

## Reproduce

From the repository root, with Go 1.27.1 and network access for module downloads:

```sh
bash specs/1728-clef-spike/run.sh
CLEF_SPIKE_BREAK=adapter bash specs/1728-clef-spike/run.sh
```

The first command must pass. The second deliberately removes the adapter and
must fail. The script copies the plugin to a temporary directory, adds the
spike test and `github.com/rshade/go-decide/clefclient@v0.1.0` there, runs it,
then deletes the copy. It does not load credentials or call Cloudflare.
The test transport records the outgoing request and replays fixture bytes.
The `clefspike` build tag keeps this experiment out of normal repository tests.

Recorded outputs are in `output.txt` and `break-output.txt`. Removing the
adapter produced `/v1/systemone` instead of `/clef`, and decoding the wrapped
response failed with `no answers`. Restoring it passed all three subtests.
`CLEF_SPIKE_BREAK` is an experiment switch, not plugin configuration.

## Evidence and limits

| Check | Observed result |
| --- | --- |
| Outgoing request | Same serialized `jevapi.Request`, including state and question criteria; adapter preserves the body |
| Path | `/client/v4/accounts/<account>/ai/run/@cf/cloudflare/clef`, rather than `/v1/systemone` |
| Authentication | Dummy bearer token preserved; validity of a real Cloudflare token not tested |
| Model | Caller sets `model: "clef"`; decoded recording reports `clef` |
| Response envelope | Adapter unwraps Cloudflare's `result` before the existing client decodes it |
| Recorded `noul` | `urgent=0.9905`, recognized by `HasNoul()` |
| Recorded `score` | `severity=2.9571`, recognized by `HasScore()`; input token count 346 |
| Recorded `choice` | Complete unmodified recording rejected with `unsupported type "choice"` |
| Failure handling | Changing only envelope `success` to `false` yields an `APIError` with status 502 |
| Break check | Removing adapter fails path, response decoding, and failure-status checks |

The successful subtest explicitly projects the recording by removing only the
`team` choice answer. All retained answer values and metadata come from the
recording. It is not an independent real two-question response. The failure
subtest is also explicitly a mutation, not a recorded failure response.

This proves client-side serialization, path rewriting and response decoding
for the observed values. It does **not** prove that Cloudflare accepts our
exact scorer prompts and criteria, preserves generated question keys, handles
batches or duplicate questions as expected, or returns correct recommendation
scores. No end-to-end `ScoreRecommendations` test against Clef was run. The
fixture concerns support-ticket questions, not cloud recommendations. A live
run and a recording of the scorer's exact questions remain necessary. All
three question types working unchanged is specifically disproven for this
client, although `choice` support is unnecessary for the current scorer.

## Adapter and follow-up

Use `clefclient.BaseURL(account)` and `clefclient.NewTransport(next)` with the
existing client. The caller still supplies the Cloudflare token, selects
`clef` in every request, and refuses redirects through `http.Client.CheckRedirect`.
The transport refuses `GET /v1/models`; model discovery cannot be reused.
Client error handling, timeout and retry policy also remain the caller's
responsibility. In particular, importing the transport does not import
go-decide's higher-level client's retry policy.

In the temporary plugin module, `go get` added `kataras/jev v0.1.0` and
`go-decide v0.1.0`; `ax-go v0.9.0` was already an indirect dependency. Reusing
the exported transport avoids maintaining a private copy. This spike does not
settle the dependency decision or authorize the gated refactor.

Next evidence: an owner-run, opt-in live test using synthetic recommendations,
followed by a sanitized recording of the exact scorer request and response.
Agents must not read or set credentials. Hosted Jev sends the request to
TypeSafe AI; hosted Clef sends it to Cloudflare. A local deployment could keep
requests on the operator's machine, but self-hosting was not tested here.
Scores are ranking signals, never approval. No calibration claim is made.

## Sources and provenance

- FinFocus baseline: `a9ce50a7938d6193185bc3650f451f84d15ed737`.
- [go-decide v0.1.0 adapter](https://github.com/rshade/go-decide/blob/v0.1.0/clefclient/transport.go)
  and [base URL construction](https://github.com/rshade/go-decide/blob/v0.1.0/clefclient/clefclient.go).
  Tag object observed as `93372a7e2bcbd8b689c0aeab5f88d6841e95b41a`.
- [Original response fixture](https://github.com/rshade/go-decide/blob/v0.1.0/clefclient/testdata/response.json),
  copied byte-for-byte into `testdata/response.json`. SHA-256:
  `bbcb725f593c013de1bb12262513f0c79ef6bcf709750aaba5344c6a998603f6`.
  Its Apache 2.0 license is retained in `testdata/LICENSE`.
- [Upstream research notes](https://github.com/rshade/go-decide/blob/v0.1.0/docs/clef-2026-10-06.md)
  identify the fixture as a real recorded response. This is upstream provenance,
  not a response independently captured during this spike. The local checkout
  at `4bc3386` contains identical fixture bytes and the same provenance statement.
- [Cloudflare announcement](https://blog.cloudflare.com/clef-decision-models/)
  claims Jev API compatibility. Our replay supports only the narrower results
  above; it does not verify the hosted service's current behavior.
