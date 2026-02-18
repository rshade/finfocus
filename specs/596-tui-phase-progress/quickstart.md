# Quickstart: Verifying TUI Immediate Launch

**Branch**: `596-tui-phase-progress`

## Build and Run

```bash
make build
```

## Verify Immediate TUI Launch

Run the overview command on any Pulumi stack:

```bash
./bin/finfocus cost overview
```

**Expected behavior**:

1. TUI appears immediately (within 1 second) with a spinner
2. Phase messages update as loading progresses:
   - "Loading stack state..."
   - "Running pulumi preview..."
   - "Starting cost plugins..."
   - "Merging resources..."
3. TUI transitions to progress bar during enrichment
4. TUI transitions to interactive list when complete

## Verify Cancellation

```bash
./bin/finfocus cost overview
# Press 'q' while spinner is showing
```

**Expected**: TUI exits cleanly within 2 seconds.

## Verify Error Handling

```bash
# Run in a directory without a Pulumi project
cd /tmp
finfocus cost overview
```

**Expected**: TUI shows error message and exits cleanly.

## Verify Plain Output Unchanged

```bash
./bin/finfocus cost overview --plain
./bin/finfocus cost overview --output json
```

**Expected**: Output is identical to the behavior before this change.

## Run Tests

```bash
go test ./internal/tui/... -v -run TestOverview
go test ./internal/cli/... -v -run TestOverview
make test
make lint
```
