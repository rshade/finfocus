# Contract: `cost cluster` capacity split

## JSON

`--output json` adds a top-level `capacity` array, sorted by `total_cost`
descending then `capacity_type`. It is `[]` when there are no rows, never
`null`.

```json
{
  "capacity": [
    {
      "capacity_type": "on-demand",
      "cpu_cost": 50.0,
      "mem_cost": 5.0,
      "total_cost": 55.0,
      "idle_cost": 3.0,
      "nodes": 2,
      "rows": 4
    },
    {
      "capacity_type": "spot",
      "cpu_cost": 30.0,
      "mem_cost": 5.0,
      "total_cost": 35.0,
      "idle_cost": 3.0,
      "nodes": 1,
      "rows": 2,
      "notes": ["spot node priced on-demand"]
    },
    {
      "capacity_type": "unlabeled",
      "cpu_cost": 10.0,
      "mem_cost": 0.0,
      "total_cost": 10.0,
      "idle_cost": 1.0,
      "nodes": 1,
      "rows": 2
    }
  ]
}
```

`notes` is omitted when empty. The `total_cost` values sum to `total`, and the
`idle_cost` values sum to `idle` (absent when namespace-scoped).

## NDJSON

The `summary` line carries `capacity`.

## Table

Printed after the group table when at least one group is not `unlabeled`:

```text
CAPACITY TYPE  NODES  TOTAL  SHARE  IDLE  NOTES
```

`SHARE` is `total_cost` over the cluster total.
