# Quickstart: Cluster Allocation Web Hook

## Unit and parity tests

```bash
go test ./internal/engine/ -run 'ClusterReport|ClusterInputError|ClusterPluginMissing'
go test ./internal/config/ -run ParseAllocationPolicy
go test ./internal/viewmodel/ -run Cluster
go test ./internal/webui/ -run Cluster
go test ./internal/cli/ -run 'CostCluster|WebCluster'
```

## Manual check against a live dashboard

1. Start the dashboard from a Pulumi project:
   `PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port 8765`.
2. Open the printed link once so the browser holds the session cookie.
3. From the browser console:

   ```js
   await (await fetch('/api/cluster/query', {method: 'POST',
     headers: {'Content-Type': 'application/json'},
     body: JSON.stringify({groupBy: 'namespace'})})).json()
   ```

4. Without cluster plugins installed, the answer is
   `status: "unavailable"` naming `usage_stats`. With the kubernetes plugin
   and a reachable cluster, compare it with
   `finfocus cost cluster --output json` from the same directory.
5. Stop the server with an anchored `pkill -f '^<path>/finfocus --web'`.

## Render evidence

2026-11-19, against the local `oc-e2e` kind cluster (one node, temporarily
labeled `m5.large` in `us-east-1` for pricing and unlabeled afterward), with
an isolated `FINFOCUS_HOME` holding the kubernetes plugin v0.1.13 and
aws-public v0.2.5:

- `finfocus --web --no-browser --port 18766 --pulumi-state
  testdata/overview/state-cluster-expansion.json` rendered the Overview with
  the EKS cluster row, its expansion caret, and the live-cluster footnotes.
  The header has nine view links and no Cluster link yet, which is where
  the follow-up view goes.
- From that page, `POST /api/cluster/query` (`{}` and `groupBy: "node"`)
  and `POST /api/cluster/policy` matched `finfocus cost cluster --output
  json`, `--group-by node --output json`, and `--show-policy --output json`
  from the same home and kubeconfig, field for field once `status` and
  `display` were removed: total $70.08, idle $52.77, eight namespace groups
  including `__idle__`.
- `groupBy: "daily"` answered 400 `invalid_cluster_query` with the CLI's
  message.
