---
title: "Scorer replay fixtures"
description: "Provenance and limits of backend contract fixtures"
layout: "docs"
---

## Clef recording

`clef-response.json` is an unchanged copy of the real recorded response in
[go-decide v0.1.0](https://github.com/rshade/go-decide/blob/v0.1.0/clefclient/testdata/response.json).
Upstream identifies it as a recording in its
[research notes](https://github.com/rshade/go-decide/blob/v0.1.0/docs/clef-2026-10-06.md).
The Apache 2.0 license is retained as `clef-LICENSE`.

SHA-256: `bbcb725f593c013de1bb12262513f0c79ef6bcf709750aaba5344c6a998603f6`.

`TestBackends_RecordedScorerContract` replays its observed `noul` and `score`
values through both backends. The Jev replay unwraps the result envelope;
the Clef replay retains it. Both explicitly remap recorded support-ticket
answer keys to the recommendation question keys produced by the scorer.
The original `choice` answer is omitted because the scorer does not ask
choice questions. These are wire-format projections of a Clef recording,
not independently captured Jev responses or recordings of recommendation
prompts. The shared SDK conformance runner executes against both backends.
The failed-envelope test changes only `success` to `false`.

No fixture here proves hosted recommendation quality or calibration. Live
calls remain opt-in for the operator, and agents must not access credentials.
