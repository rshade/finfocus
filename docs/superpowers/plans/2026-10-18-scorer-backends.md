---
title: "Backend-neutral scorer implementation plan"
description: "Implementation and verification for issue 1728 after spike review"
layout: "docs"
---

## Goal and architecture

Implement issue #1728 using its approved tasks and the evidence in
`specs/1728-clef-spike/README.md`. Preserve scoring questions and behavior,
move the wire client to `internal/backends/jev`, and reuse it through
`go-decide/clefclient@v0.1.0` for Cloudflare. Execute in this session.

## Global constraints

- Plugin/module/binary name: scorer; default backend: jev.
- Generic environment variables: SCORER_BACKEND, SCORER_API_KEY,
  SCORER_BASE_URL, SCORER_MODEL, SCORER_TIMEOUT, SCORER_BATCH_SIZE,
  SCORER_DUPLICATE_THRESHOLD. Clef account: SCORER_CLEF_ACCOUNT_ID.
- Backend URL defaults are chosen after selection; Clef accepts only model clef.
- Missing credentials leave scoring unauthenticated. No agents access secrets.
- Keep old registry entry and all historical releases/tags until scorer assets
  exist publicly. Verify installs against controlled release fixtures.
- Keep scoring implementation unchanged except import/package/plugin naming.
- New component starts at 0.1.0. Do not advertise old Jev version as scorer.

## Task 1: Backend and configuration contract

- [x] Add failing backend-selection tests, including missing/bad account,
  invalid backend, incompatible model, unsafe URL, NaN threshold, and defaults.
- [x] Rename module/directory and wire client package; add
  `clef.New(jev.Config, account string) (*jev.Client, error)` using the shared
  client and exported adapter, preserving cancellation and refusing redirects.
- [x] Add `newBackend(Config) (scoring.Backend, error)` for plugin/live tests;
  update settings and neutral environment parsing. Run plugin tests.

## Task 2: Recorded scorer tests and live opt-in

- [x] Replay the recorded response behind both backends through the real scorer;
  explicitly identify answer projection/remapping rather than claiming prompts
  were captured live. Cover noul, score, duplicate mapping and provider model.
- [x] Test request/auth/path, failed envelope, redirects, cancellation and missing
  credentials. Live tests require explicit SCORER_LIVE_TEST=1 and skip without
  credentials; model listing applies only to Jev.
- [x] Break each backend's routing/envelope mapping and verify tests fail;
  restore and run race tests.

## Task 3: Rename integration and registry transition

- [x] Rename build/install/test/lint targets, CI references, release-please
  component and version metadata. Retain legacy registry entry.
- [x] Add scorer registry entry and a clean-home list/install/list test using
  fixture release assets. Cover old jev installation behavior and tag filtering.
- [x] Check plugin boundaries, registry tests and build-scorer.

## Task 4: Documentation, verification and review

- [x] Update README, plugin docs and install/upgrade skills, with migration
  tables, third-party data disclosure and ranking-only semantics.
- [x] Run make lint, make test, make docs-lint and module tidy checks. Record
  commands and material external blockers. Review all changed code and docs.
- [x] Prepare breaking Conventional Commit subject/footer and final handoff;
  do not publish releases or remove legacy entry before assets exist.

## Review focus

Account path injection, redirects carrying credentials, mismatched model
selection, accidentally running live tests, and registry tags selecting the
core release. Tests above must cover these conditions.

## Progress

- Starting from a clean tracked tree with the uncommitted spike artifacts.
- User explicitly authorized full implementation after reviewing spike result.

- Backend/config tests first failed against the old implementation, then passed.
- Both backend mutations failed the recorded scorer test, then sources restored
  and the complete plugin race suite passed.
- Clean-home registry install/list and legacy reinstall/coexistence tests passed.
- Actual CLI available list and source install/list passed in a temporary home.
- Independent final review found no actionable regressions.
- Full `make test`, `make docs-lint`, and
  `make lint GOLANGCI_LINT=golangci-lint` passed. The lint override selects
  mise's pinned 2.15.0 from PATH instead of the stale home-directory binary.
  The original exhaustive analyzer configuration was retained unchanged.
- All new non-site Markdown passed targeted markdownlint; shellcheck passed
  for the spike runner, and the scorer module passed `go mod tidy -diff`.
- Public scorer release installation and live Cloudflare verification require
  release assets and owner-run credentials, respectively; neither was claimed.
- Fixture ruling: replay observed Clef answer values through both wire formats,
  explicitly documenting key remapping and envelope projection. No independently
  captured Jev response or recommendation-prompt capture was available.
