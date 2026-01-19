# Quickstart: Plugin Init Recorder Fixtures

## Goal

Enable `finfocus plugin init` to fetch canonical fixtures, run recorder-backed validation, and output recorded request fixtures for new plugins.

## Prerequisites

- Access to the finfocus repository fixtures when online.
- Recorder plugin available in the local plugin registry.

## Implementation Outline

1. Extend `plugin init` to prepare a recording session directory under the new plugin's `testdata` output.
2. Fetch canonical plan and state fixtures (or use offline fixtures when `--offline` is set).
3. Configure recorder output to target the session directory.
4. Execute core commands for projected cost, actual cost, and recommendations.
5. Verify recorded request files exist and summarize results for the user.

## Validation Checklist

- Offline mode completes without network access.
- Recorded request files are produced for each required request type.
- Errors include actionable remediation guidance.
