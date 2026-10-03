# Testing a FinFocus plugin

## Unit tests

`make test` runs `go test -v ./...`. The scaffold's `calculator_test.go` uses
the SDK's in-process harness:

```go
plugin := NewCalculator()
tp := pluginsdk.NewTestPlugin(t, plugin)
res := pluginsdk.CreateTestResource("aws", "aws:ec2/instance:Instance", map[string]string{
    "instanceType": "t3.micro",
})
resp := tp.TestProjectedCost(res, false) // second arg: expect an error
```

`NewTestPlugin` starts a loopback gRPC server and registers cleanup.
`TestName`, `TestProjectedCost`, and `TestActualCost` exist. Remember that
`TestActualCost` with `expectError=false` fails if there are no results.
`NewTestServer(t, plugin)` gives a raw `CostSourceServiceClient` via
`Client()` when you need other RPCs. `CreateTestResource` puts inputs in
`Tags` but leaves `Sku` and `Region` empty; set them on the descriptor (or
use `NewResourceDescriptor` with `WithSKU` and `WithRegion`) when your code
reads those fields.

The repo's CLAUDE.md or contributing guide may require testify; follow it.

Cover: every resource type, an unknown type, missing SKU, missing region,
other-region `Supports`, a `$0` free resource, and sparse `ref.*` input.

## Conformance

```go
func TestConformance(t *testing.T) {
    result, err := pluginsdk.RunStandardConformance(NewCalculator())
    require.NoError(t, err)
    pluginsdk.PrintConformanceReport(t, result)
    assert.Zero(t, result.Summary.Failed)
}
```

Levels: `RunBasicConformance` (name, Supports, basic projected cost),
`RunStandardConformance` (adds error handling, 24 h range, 10 concurrent
requests), `RunAdvancedConformance` (adds latency limits, 50 concurrent
requests, 30 day range). `ConformanceResult` has a `Summary` with `Total`, `Passed`, and `Failed`; confirm in the
SDK source for your version.

## Integration against real finfocus

1. `make build install`.
2. Create a plan from a Pulumi project:
   `pulumi preview --json > plan.json`. Core's repo ships sample plans such
   as `examples/plans/aws-simple-plan.json`.
3. `finfocus plugin list` and `finfocus plugin validate`.
4. `finfocus cost projected --pulumi-json plan.json --debug`. Expect a row
   per resource with your plugin as the adapter and no `VALIDATION:` notes.
5. Add `--output json` to check `billingDetail` and numbers.

If the plugin is not called, check `Supports` (reason is logged at debug),
the install path and version directory, and that `plugin list` shows the
spec version rather than `N/A`.

## Before finishing

`make test`, `make lint`, `make build`. Do not commit unless asked.
