# Tasks: Plugin development skill installed by init and upgrade

**Input**: [spec.md](spec.md), [plan.md](plan.md)

## Phase 1: Installer

- [x] T001 Tests for `Source`, `Args`, and `Install` in
      `internal/pluginskill/install_test.go` (release, prerelease, dirty,
      unprefixed, old and default versions; npx missing; failure with output
      tail; timeout; success paths; environment)
- [x] T002 Implement `internal/pluginskill/install.go`

## Phase 2: Commands (US1, US2, US3)

- [x] T003 Fake installer for `internal/cli` tests (`export_test.go`, `main_test.go`)
- [x] T004 [US1] `plugin init`: `--no-skill`, install after generation, skip on
      `--offline` and `--docker-only`; tests
- [x] T005 [US2] `plugin upgrade`: `--no-skill`, install after apply or when up
      to date, `skill` in JSON, paths in `changed_files`; tests
- [x] T006 [US3] Warning output with the command to run later; tests
- [x] T007 Pass `--no-skill` in `test/integration/plugin/init_test.go`

## Phase 3: Skill (US1)

- [x] T008 Write `agent-skills/finfocus-plugin-dev/` and package the `.skill`
- [x] T009 Remove `.claude/skills/finfocus-plugin/`; list the skill in
      `agent-skills/README.md`

## Phase 4: Scaffold fixes

- [x] T012 `plugin init` templates: `SpecVersion` from the SDK, Pulumi type
      token, no default price, flat JSON manifest in `make install`; unit
      assertions in `internal/cli/plugin_init_test.go`
- [x] T013 `test/integration/plugin/scaffold_build_test.go`: generated project
      tidies, tests, installs, and passes `plugin validate`
- [x] T014 Update the skill text for scaffolds from v0.4.0 and earlier

## Phase 5: Polish

- [x] T010 Document `--no-skill` and the install in the CLI reference
- [x] T011 `make test`, `make lint`, markdownlint on changed Markdown
