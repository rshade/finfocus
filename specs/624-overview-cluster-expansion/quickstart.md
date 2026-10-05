# Quickstart: Overview Cluster Expansion

Validation scenarios proving the feature end-to-end. Fixture:
`testdata/overview/state-cluster-expansion.json` (one EKS cluster + two
Deployments + one ConfigMap).

## 1. Projected expansion (no plugins)

```bash
go run ./cmd/finfocus overview \
  --pulumi-state testdata/overview/state-cluster-expansion.json \
  --output json --yes
```

Expect: the cluster row contains `childUrns` with both Deployment URNs; each
Deployment row contains `parentUrn` + `expansionSource: "projected"`; the
ConfigMap is untouched; `summary.projectedMonthly` equals the pre-expansion
total for the same fixture.

## 2. No-regression

```bash
go run ./cmd/finfocus overview \
  --pulumi-state testdata/overview/state-no-changes.json \
  --output table --yes --plain
```

Expect: output byte-identical to `testdata/overview/golden/table-no-changes.txt`
(no expander markers, no footnotes).

## 3. TUI expand/collapse

```bash
go run ./cmd/finfocus overview --pulumi-state testdata/overview/state-cluster-expansion.json
```

Expect: cluster row shows `▸`; press `e` → marker flips to `▾` and two
indented `↳` workload rows appear beneath it; press `←` → children hide.

## 4. Live expansion (requires cluster + plugins)

```bash
finfocus plugin install kubernetes   # provides usage-source + allocator
finfocus overview --stack prod       # stack declares one cluster
```

Expect: cluster expands into `live` namespace rows; when declared workloads
also exist, a `†` footnote states live data was preferred and how many
projected rows were hidden.

## 5. Golden tests

```bash
go test ./internal/cli/ -run TestIntegration_ClusterExpansion -v
go test ./internal/engine/ -run TestCluster -v
go test ./internal/tui/ -run TestOverviewGolden -v
# regenerate goldens intentionally:
UPDATE_GOLDEN=1 go test ./internal/cli/ -run TestIntegration_ClusterExpansion
```
