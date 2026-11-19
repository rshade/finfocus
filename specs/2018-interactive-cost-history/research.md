# Research

## Chart library

Use ntcharts v2's `TimeSeriesLineChart` with the repository's Bubble Tea v2
runtime. Issue #550 assumed the v1 runtime, which FinFocus upgraded in #843.
The issue owner approved using ntcharts v2.

## History data and budgets

Reuse the currency-selected history document and the existing checkpoint diff.
No migration or background collection is needed. Resolve provider budget
aliases using the same precedence as the budget engine. Plot only monthly
thresholds denominated in the document's currency.

## Web delivery

The History view already renders checkpoint history and deployment diffs.
Extend its existing API response with applicable thresholds. Chart navigation
and threshold controls can use these values in a UI-only follow-up.
