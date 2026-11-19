# Data model

- `history.ViewDocument` holds the selected stack, its currency-filtered
  snapshots, matching deployment annotations, and any currency warning.
- `history.CostSnapshot` contains a deployment version, UTC timestamp,
  monthly cost, and stored provider and resource amounts.
- `history.CostAnnotation` links a deployment version to its message and
  operation counts. The detail view compares with the preceding snapshot
  in the selected history using `history.DiffSnapshots`.
- `history.ChartBudget` holds a monthly threshold amount, its formatted
  display value, and an optional normalized provider. An empty provider
  means the global threshold.
- The terminal model keeps its cursor, visible span, toggles, dimensions and
  focused provider in memory for the duration of the view.
