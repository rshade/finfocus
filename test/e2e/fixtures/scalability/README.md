# Scalability Fixture

Pulumi preview plans of increasing size for measuring `finfocus cost projected`
end to end, from plan ingestion through output, against the real binary.

## Contents

| Path | Purpose |
| --- | --- |
| `cmd/generate/` | Deterministic plan generator (standard library only) |
| `snapshots/plan-N.json` | Checked-in plans for N = 10, 50, 100, 500, 1000 |
| `generate-snapshots.sh` | Regenerates every snapshot |

Each plan holds 17 fixed AWS resources (VPC, three subnets, security group,
internet gateway, route table, route, three route table associations, IAM
role and policy attachment, Lambda function, S3 bucket, RDS subnet group, and
RDS instance), N `t3.micro` EC2 instances spread across `us-east-1a`-`c`, and
a 20 GiB `gp3` EBS volume for two of every five instances. It also carries the
`pulumi:pulumi:Stack` and `pulumi:providers:aws` steps a real preview has;
those are not priced.

| N | EC2 | EBS | Fixed | Priced resources | Plan steps |
| --- | --- | --- | --- | --- | --- |
| 10 | 10 | 4 | 17 | 31 | 33 |
| 50 | 50 | 20 | 17 | 87 | 89 |
| 100 | 100 | 40 | 17 | 157 | 159 |
| 500 | 500 | 200 | 17 | 717 | 719 |
| 1000 | 1000 | 400 | 17 | 1417 | 1419 |

Steps use the shape `pulumi preview --json` emits: type and inputs live under
`newState`, outputs that are unknown at preview time are the Pulumi unknown
sentinel, and cross-resource references are listed in `propertyDependencies`.

## Regenerating

Run `./generate-snapshots.sh` after changing the generator. The generator's
`TestSnapshotsAreCurrent` fails when a checked-in snapshot no longer matches
its output; CI runs it on every pull request
(`go -C test/e2e test ./fixtures/scalability/...`). `internal/ingest` parses
every snapshot in `make test`.

## Tests

`test/e2e/scalability_test.go` (build tag `e2e`) runs `cost projected` on each
snapshot with the projected-cost cache off (`--cache-ttl 0`), logs total,
per-instance, and per-resource wall time, kills a run that exceeds its
wall-time bound, and checks the JSON, NDJSON, and table output at N = 100. It
needs no cloud credentials:

```bash
make build
cd test/e2e
FINFOCUS_BINARY="$PWD/../../bin/finfocus" go test -tags e2e -run '^TestScalability' -v .
```

Costs depend on which plugins are installed, so the tests check row counts
and output structure, not prices.
