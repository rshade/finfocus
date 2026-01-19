# Research Notes: Plugin Init Recorder Fixtures

## Decision: Reuse existing GitHub download patterns for fixtures

**Rationale**: The registry installer already handles GitHub release downloads with timeouts, validation, and error reporting. Reusing the same client keeps network behavior consistent with the rest of the CLI.

**Alternatives considered**: Direct `net/http` downloads without shared retry/timeout handling.

## Decision: Configure recorder output via environment variables

**Rationale**: The recorder plugin already reads `FINFOCUS_RECORDER_OUTPUT_DIR` and `FINFOCUS_RECORDER_MOCK_RESPONSE`. Using the existing env-driven config avoids adding new configuration surface area.

**Alternatives considered**: Adding new CLI flags or a temporary config file to drive recorder settings.

## Decision: Offline mode uses local fixture files under repository fixtures

**Rationale**: The repository already maintains canonical fixture files under `test/fixtures/` and `test/e2e/fixtures/`. Using these files avoids embedding fixtures into the binary while still providing reliable offline inputs.

**Alternatives considered**: Bundling fixtures into the binary or requiring users to supply custom fixture paths.
