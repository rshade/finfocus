---
title: "Carbon Plugin Validation"
description: "Build, numerical, transport, and clean-home validation results."
layout: "docs"
---

## Prerequisites

Use the tool versions in `mise.toml`. No AWS credentials, Kubernetes cluster,
Jev key, or cloud infrastructure are required. Run from the repository root.
All commands below were run from the repository root after implementation.

## Build and Model Checks

```bash
make build build-carbon
make test-carbon
go test ./internal/engine/... -run 'Test.*Carbon' -count=1
go test ./internal/proto/... -run 'Test.*Carbon' -count=1
```

Expect known-instance numerical fixtures to pass in two regions at 0%, 50%, and
100% utilization. Unknown data must decline rather than return invented values.
The carbon package reports 96.3% statement coverage; `EstimateResource` reports
94.4%, with its only uncovered branch handling malformed embedded JSON. The embedded
asset is immutable and separately checked for valid decoding and complete coverage.
The RPC implementation reports 100% coverage.

## Isolated Installation and Transport

```bash
carbon_home=$(mktemp -d)
make install-carbon CARBON_INSTALL_DIR="$carbon_home/plugins/carbon/v0.0.0"
FINFOCUS_HOME="$carbon_home" ./bin/finfocus plugin list --output json
```

Expect the manifest and handshake version to agree and capabilities to include
carbon without monetary projected-cost capability. The integration test builds
the real binary, installs into a temporary plugin home, invokes the production
transport, and checks carbon equivalencies alongside a monetary fixture source.
It also verifies that carbon-only configuration retains `NO_COST_DATA`.

```bash
go test ./test/integration/plugin/... -run TestCarbonPlugin -count=1
```

## Required Repository Gates

```bash
make validate
make test
make lint
make test-race
make docs-lint
make check-plugin-boundaries
```

Validation passed for `make validate`, `make test`, `make test-race`, `make docs-lint`,
changed-Markdown lint, nested-module tidy/vet, plugin conformance and integration,
and five platform builds (Linux amd64/arm64, macOS amd64/arm64, Windows amd64).
`make lint` passed with the pinned golangci-lint 2.15.0; the default Makefile path
pointed to an older Go 1.26-built binary and could not load this Go 1.27.1 module.
The boundary check passed as part of lint.

`govulncheck` reports Go 1.27.1 standard-library vulnerabilities fixed in Go 1.27.2;
the unchanged main branch reports 13 standard-library findings, while carbon reports
8 reachable standard-library findings and 5 imported-package findings without call
traces. It reports no vulnerable required modules. The Go pin is repository-wide and
outside this issue scope.
