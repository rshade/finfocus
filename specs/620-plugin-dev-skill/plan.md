# Implementation Plan: Plugin development skill installed by init and upgrade

**Branch**: `620-plugin-dev-skill` | **Spec**: [spec.md](spec.md)

## Summary

Add the `finfocus-plugin-dev` agent skill and a small `internal/pluginskill`
package that installs it, with `finfocus-plugin-upgrade`, through
`npx skills add`. `plugin init` and `plugin upgrade` call the package after
their own work and treat every failure as a warning.

## Technical Context

- Go 1.27.1, Cobra, ax-go (`ax.Perform` rehearse/commit), `golang.org/x/mod/semver`
- External tool: `skills` npm package, pinned to 1.7.0, run through `npx`
- Testing: testify; the installer's process runner and PATH lookup are
  injected, and `internal/cli` tests replace the installer in `TestMain`

## Constitution Check

- No stubs or TODOs; the install is complete or reported as skipped.
- Tests cover every skip and failure path without the network.
- No change to `.golangci.yml`.

## Design

### `internal/pluginskill`

- `Source(version)` returns the tag URL for a release newer than `v0.4.0`,
  else the `main` URL.
- `Args(version)` returns the full argv. `Command(version)` joins it for
  display.
- `Installer{Run, LookPath, Timeout}` with `Install(ctx, dir, version) Result`.
  `New()` returns one backed by `os/exec`. `Skipped(version, reason)` builds
  the result for `--no-skill`, `--offline`, and dry runs.
- `Result{Installed, Source, Command, Paths, Warning}` is the JSON shape.

### `internal/cli`

- `skillInstaller` interface and package variable; `export_test.go` exposes a
  setter and `main_test.go` installs a fake for the whole package.
- `plugin init`: `--no-skill`; install after `generateAll` (and fixtures);
  print the result.
- `plugin upgrade`: `--no-skill`; install in the commit path (up to date or
  applied); add `Skill` to the result; append paths to `Changed`.

### Skill content

`agent-skills/finfocus-plugin-dev/` replaces `.claude/skills/finfocus-plugin/`.

## Project Structure

```text
agent-skills/finfocus-plugin-dev/        # new skill
internal/pluginskill/                     # installer
internal/cli/plugin_init.go               # --no-skill, install call
internal/cli/plugin_upgrade.go            # --no-skill, install call, JSON
internal/cli/plugin_skill.go              # shared printing + installer var
test/integration/plugin/init_test.go      # pass --no-skill
```

## Complexity Tracking

None.
