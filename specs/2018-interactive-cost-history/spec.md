# Interactive cost history

**Status**: Implemented

**Issue**: [#550](https://github.com/rshade/finfocus/issues/550)

## User scenarios and testing

1. With saved deployments and a capable terminal, run `finfocus cost history
   view --stack dev`. Inspect projected monthly costs, zoom in on a spike,
   pan through the range, and jump to its first or last checkpoint.
2. Toggle provider series, cycle the highlighted provider, and compare the
   visible costs against configured thresholds.
3. Show the details of a marked deployment and inspect its message, change
   counts and largest resource cost changes from the preceding checkpoint.
4. Redirect the command or use `--plain` for the original chart. Explicit JSON
   output continues to return the history document.

## Functional requirements

- **FR-001**: Select interactive mode for capable terminals with interactive
  input. Preserve existing flags, date and currency selection, and machine
  output.
- **FR-002**: Show a braille time-series chart with bounded pan and zoom.
  Home and End jump to the first and last snapshots.
- **FR-003**: Toggle provider series with s, cycle focus with Tab, and toggle
  global and relevant provider thresholds with b.
- **FR-004**: Mark annotated deployments by time. Toggle details with d and
  jump to the first nine annotations using number keys. Quit with q or Ctrl+C.
- **FR-005**: Handle empty, single-point, and resized views. Display only
  thresholds in the same currency and monthly period as the plotted costs.

## Web UI Fit

Extend the existing History view. Its heading and controls select the range,
its summary displays thresholds, and its checkpoint table and Diff section
supply the input and detail for deployment drill-down. The existing redacted
routes expose the shared Go history and diff operations. Expose all applicable
thresholds through the history view response. A UX-only follow-up can add
chart interaction and put deployment details next to the selected point.

## Success criteria

- The keyboard controls work on populated history, including a chart at
  80 by 24 and a wider terminal.
- Redirected, plain and structured output cases pass their existing tests.
- CLI and web use matching snapshot, threshold and deployment diff values.
- The interactive model and rendering reach 80 percent test coverage.
