# Data Model: Plugin Init Recorder Fixtures

## Entity: Fixture Source

**Description**: Canonical input data for initialization recordings.

**Fields**:

- `id`: Unique identifier for the fixture source.
- `type`: Plan or state input.
- `provider`: Provider namespace for the plan (if applicable).
- `version`: Release tag or `latest` reference.
- `origin`: Remote URL or local path.
- `checksum`: Optional integrity hash.

**Validation Rules**:

- `type` must be `plan` or `state`.
- `origin` must resolve to a readable file.
- `version` must match available release identifiers when online.

## Entity: Recording Session

**Description**: A single run that captures requests during plugin initialization.

**Fields**:

- `id`: Unique identifier for the session.
- `started_at`: Timestamp when recording begins.
- `completed_at`: Timestamp when recording ends.
- `status`: `initialized`, `fetching`, `recording`, `completed`, `failed`.
- `fixture_sources`: List of fixture source IDs used.
- `output_dir`: Target directory for recorded requests.
- `errors`: List of error summaries (if any).

**Validation Rules**:

- `output_dir` must be writable.
- `status` transitions must follow the defined order.

**State Transitions**:

- `initialized` -> `fetching` -> `recording` -> `completed`
- Any state -> `failed` on unrecoverable error

## Entity: Recorded Request

**Description**: Captured request payloads produced during recorder execution.

**Fields**:

- `id`: Unique request identifier.
- `request_type`: Projected cost, actual cost, recommendations, or plugin info.
- `timestamp`: Capture time.
- `payload_path`: Location of the serialized request payload.
- `session_id`: Recording session reference.

**Validation Rules**:

- `payload_path` must exist and be readable after recording.
- `request_type` must be one of the supported request types.

## Entity: Plugin Init Output

**Description**: Generated project output for new plugin scaffolding.

**Fields**:

- `project_path`: Root directory for the new plugin.
- `testdata_path`: Directory containing recorded requests.
- `recording_session_id`: Session used to produce recorded data.

**Relationships**:

- One `Plugin Init Output` references one `Recording Session`.
- One `Recording Session` uses many `Fixture Sources` and produces many `Recorded Requests`.
