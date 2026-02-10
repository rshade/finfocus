# Quickstart: Running Plugin Integration Tests

## Prerequisites

- Go 1.25.7 or later
- Make (optional, but recommended)

## Running the Tests

To run the plugin integration tests specifically:

```bash
# Run all plugin integration tests
go test -v ./test/integration/plugin/...

# Run specific command tests
go test -v ./test/integration/plugin/ -run TestPluginInit
go test -v ./test/integration/plugin/ -run TestPluginInstall
go test -v ./test/integration/plugin/ -run TestPluginUpdate
go test -v ./test/integration/plugin/ -run TestPluginRemove
```

## Debugging

To run with verbose output and keep the temporary directories (if the test fails):

```bash
# Note: t.TempDir() automatically cleans up, but you can print the path
go test -v ./test/integration/plugin/ -run TestPluginInstall_Debug
```

## Adding New Tests

1.  **Identify the Scenario**: Is it an Init, Install, Update, or Remove scenario?
2.  **Locate the File**: Open the corresponding `*_test.go` file in `test/integration/plugin/`.
3.  **Add a Test Case**: Add a new entry to the `testCases` slice or create a new `func TestXxx(t *testing.T)`.
4.  **Mocking**: If you need to mock a new registry response, update `setup_test.go` or define a custom handler in your test function.
