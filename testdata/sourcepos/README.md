# Source position fixtures

Recorded Pulumi state and preview output used to test declaration-location
resolution (`internal/sourcepos`). Each runtime has its own directory.

## nodejs

Captured 2026-11-16 with Pulumi CLI v3.272.0 and `@pulumi/pulumi` 3.276.0
(Node.js 26) on a throwaway local backend, so no cloud credentials are needed.
Resources are `pulumi.dynamic.Resource` instances (`pulumi-nodejs:dynamic:Resource`),
which exercise location recording but are not priced by any plugin; pricing
for the annotation acceptance fixture is covered separately.

| Path | Contents |
| --- | --- |
| `program/` | Source as deployed (`state.json`) |
| `program-edited/` | Source after inserting two lines above `web`, changing its size, and adding `extra` (`preview.json`) |
| `vendor-lib/` | A dependency outside the project root, linked into `node_modules` |
| `pack-lib/` | A dependency installed from a tarball into `node_modules` |
| `state.json` | `pulumi stack export` of the deployed program |
| `preview.json` | `pulumi preview --json --show-sames` of the edited program |

Reproduce, from a scratch directory with `shop/` holding `program/`:

```bash
export PULUMI_BACKEND_URL=file://$PWD/backend PULUMI_CONFIG_PASSPHRASE=''
export NODE_PATH=$PWD/shop/node_modules
cd shop && npm install @pulumi/pulumi@3 ../vendor-lib ../pack-lib/packed-lib-1.0.0.tgz
pulumi stack init dev && pulumi up --yes --skip-preview
pulumi stack export > state.json
# apply program-edited/ over shop/, then:
pulumi preview --json --show-sames > preview.json
```

(`npm pack` in `pack-lib/` makes the tarball.) The recording strips the
Pulumi CLI diagnostics, which name local install paths.

### Recorded positions

| Resource | `sourcePosition` | Next project frames |
| --- | --- | --- |
| `web` | `lib/box.js#14,5` | `index.js#6,13` |
| `worker-a/b/c` | `lib/box.js#14,5` | `index.js#7,33` |
| `edge` (component) | `index.js#8,14` | |
| `edge-left` | `lib/box.js#14,5` | `lib/pair.js#7,17`, `index.js#8,14` |
| `vendored` | `../vendor-lib/index.js#4,5` (outside the root) | `index.js#9,18` |
| `packed` | `node_modules/packed-lib/index.js#4,5` (inside the root) | `index.js#10,16` |
| `shop-dev` (stack) | `node_modules/@pulumi/runtime/stack.ts#38,19` | |

In the preview, with `--show-sames`, `edge` moves from `index.js#8,14` to
`index.js#10,14` and `packed`'s caller from `index.js#10,16` to `#12,16`.

Two facts follow that the resolution rule accounts for:

- The Pulumi SDK's own frames are recorded as `node_modules/@pulumi/...`
  paths inside the project root, not absolute paths outside it. Frames under a
  `node_modules` or `site-packages` directory are dependencies, never the
  user's declaration, so the resolver skips them.
- A resource class defined in the user's project records its own constructor
  line (`lib/box.js#14,5`) as the innermost frame, so every instance shares it.
  The rule (the innermost project frame, FR-002) does not guess past it, so
  the expected location of `web`, `worker-*` and `edge-*` is that constructor
  line. A resource created by a helper function or a direct `new` resolves to
  the line that made it.

## Expected locations

Each runtime directory holds `expected.json`: for every resource, in
`state.json` (resolved against `program/`) and `preview.json` (resolved against
`program-edited/`), the location the resolver must return, or `null`. The
resolver tests read these files. They apply the rule in
`specs/2017-vscode-cost-extension/research.md` (R1) to the recorded frames.

## go

Captured 2026-11-17 with Pulumi CLI v3.272.0, `pulumi-sdk` v3.276.0 and
`pulumi-command` Go SDK v1.3.0 (Go 1.27.4) on a throwaway local backend, using
`command:local:Command` and a component resource (`shop:index:Pair`).
`program/` is the deployed source and `program-edited/` inserts two lines above
`web`, changes its command, and adds `extra`.

The program is built by hand with `go build -trimpath -o srcposgo .` and run
through `runtime.options.binary: srcposgo` (the fixture's `Pulumi.yaml` omits
the option), because the language host here picked a Go toolchain that did not
match. `-trimpath` is how the host builds, so frames are module paths:

| Resource | `sourcePosition` |
| --- | --- |
| `web` | `../../../../../../../srcposgo/main.go#12` |
| `edge` | `../../../../../../../srcposgo/main.go#16` |
| `edge-left/right` | `../../../../../../../srcposgo/lib/pair.go#18` |
| `srcposgo-dev` (stack) | `.../pulumi/sdk/v3@v3.276.0/go/pulumi/run.go#104`, with `srcposgo/main.go#11` further out in its stack trace |

The count of `../` equals the depth of the project directory. There is no
column. Dependency and standard-library frames carry a module path that is not
`srcposgo/`, and are dropped.

```bash
export PULUMI_BACKEND_URL=file://$PWD/backend PULUMI_CONFIG_PASSPHRASE='' GOTOOLCHAIN=local
cd shop && go build -trimpath -o srcposgo .     # Pulumi.yaml: runtime.options.binary: srcposgo
pulumi stack init dev && pulumi up --yes --skip-preview && pulumi stack export > state.json
# apply program-edited/, rebuild, then:
pulumi preview --json --show-sames > preview.json
```

## python

Captured 2026-11-17 with Pulumi CLI v3.272.0, `pulumi` 3.276.0 and
`pulumi-command` 1.3.0 (Python 3.14, a `venv` inside the project) on a local
backend. `program/` and `program-edited/` follow the Go layout. The virtualenv
is not recorded; the stack resource's position is
`venv/lib/python3.14/site-packages/pulumi/runtime/stack.py#235`, inside the
root, and resolves to none. Positions carry a line and no column:

| Resource | `sourcePosition` |
| --- | --- |
| `web` | `__main__.py#5` |
| `edge` | `__main__.py#6` |
| `edge-left/right` | `lib/pair.py#9` (next frame `__main__.py#6`) |

The language host's own frames are absolute paths under the home directory
(`pulumi-language-python-exec#188`), which are made relative to the root as a
`../` run and dropped. The recording replaces the local username with `user`.

```bash
python3 -m venv shop/venv && shop/venv/bin/pip install pulumi pulumi-command
# Pulumi.yaml: runtime.options.virtualenv: venv
export PULUMI_BACKEND_URL=file://$PWD/backend PULUMI_CONFIG_PASSPHRASE=''
cd shop && pulumi stack init dev && pulumi up --yes --skip-preview && pulumi stack export > state.json
# apply program-edited/, then:
pulumi preview --json --show-sames > preview.json
```

## yaml

Captured 2026-11-17 with Pulumi CLI v3.272.0 and the `pulumi-yaml` language
host bundled with it, using two `command:local:Command` resources. Every
recorded frame is inside the language host
(`home/runner/work/pulumi-yaml/.../run.go#1028`), never `Pulumi.yaml`, so no
resource has a location. `program-edited/` changes the `web` command.

## dotnet

Pending: no .NET SDK is installed on the capture machine. Support is not
claimed; see `validation.md`.
