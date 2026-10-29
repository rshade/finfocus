# Quickstart: Dependency Blast Radius

How to see the feature working once it is built, and how to check the two
facts the design still depends on.

## Prerequisites

- `make build`, then a populated project: `initial-30` or `../finfocus-demo`
  (stack `dev`). Both use an empty passphrase:
  `PULUMI_CONFIG_PASSPHRASE=''`.
- Never commit the `Pulumi.dev.yaml` change that attaching an ESC
  environment makes, and run `pulumi destroy` after any deployed check.

## 1. Terminal

```bash
PULUMI_CONFIG_PASSPHRASE='' ./bin/finfocus overview
```

Select a resource that others use (a database, a VPC subnet), press Enter.
The detail view shows a BLAST RADIUS section after the cost sections:
dependents indented by distance, each with status and monthly cost, then the
count and total. Select a leaf resource: the section says nothing depends on
it.

## 2. Confirm the component rule against real state

Before relying on research R1/R2, check a real export:

```bash
pulumi stack export --file /tmp/state.json
jq '[.deployment.resources[] | select(.dependencies)] | .[0:3]' /tmp/state.json
```

Find a resource declared with `dependsOn: [someComponent]` and confirm its
`dependencies` lists the component URN. If it lists the component's children
instead, research R2 simplifies and the ancestor walk can be dropped.

## 3. Web

```bash
PULUMI_CONFIG_PASSPHRASE='' ./bin/finfocus --web --no-browser --port 8080
```

Open the link printed on stderr, select the same resource in the Overview
table, and compare the dependents, count and total with the terminal. Render
the panel with playwright-mcp (screenshots go under `.playwright-mcp/`) and
record the stack and date in the spec's Web UI Fit "Render evidence" line.

Stop the server with an anchored pattern:
`pkill -f '^<repo-root>/bin/finfocus --web'`.

## 4. Mid-change cases

State-only: `finfocus overview --state-only --pulumi-state /tmp/state.json`,
then press `p` to run a preview and watch the section update. Use a preview
that creates one resource, deletes another, and includes a resource type no
plugin prices; the section must mark all three and leave the deleted and
unpriced ones out of the total.

## 5. Provider edges (ISS-001)

On `initial-30` (stack `dev`), run the web dashboard as in step 3 and select
`civoCluster`. The radius lists the `civo-k8s` provider at distance 1 and the
seven Kubernetes resources (Namespace, Secret, Deployment, Service, Helm
Release, ClusterRoleBinding, Ingress) beneath it: 8 dependents, all unpriced
today. Before the fix it listed 1. Selecting `civo-k8s` lists the seven, and
selecting a default provider (`default_<version>`) lists every resource it
manages. Compare one of them in the terminal (`finfocus overview`, Enter).

## 6. Tests

```bash
go test -run 'BlastRadius|DependencyGraph' ./internal/engine/ ./internal/stackload/
go test -run 'TestGolden' ./internal/tui/...      # UPDATE_GOLDEN=1 to regenerate
go test -race -shuffle=on -count=3 ./internal/engine/... ./internal/stackload/...
make lint && make test
```
