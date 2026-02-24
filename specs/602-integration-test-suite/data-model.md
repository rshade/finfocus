# Data Model: Integration Test Suite Expansion

**Feature**: #602 Integration Test Suite Expansion
**Date**: 2026-02-22

## Test Entities

### Mock Plugin Scenarios (Extended)

Extends `test/mocks/plugin/config.go` with new scenario types:

| Scenario | Behavior | Tests |
|----------|----------|-------|
| `CrashMidRPC` | Exit process during gRPC handler execution | Plugin crash recovery (US-1) |
| `SleepBeyondDeadline` | Sleep longer than context deadline (15s) | Timeout handling (US-1) |
| `RapidCrash` | Crash on first request, succeed on retry | Recovery after crash (US-1) |
| `PartialFailure` | Error for type A, succeed for type B | Analyzer partial failures (US-3) |

### Error Injection Types (Extended)

Extends `test/mocks/plugin/config.go` error injection:

| Type | gRPC Behavior | Expected Result |
|------|---------------|-----------------|
| `ExitMidCall` | `os.Exit(1)` during handler | `ErrCodePluginError` + actionable message |
| `DeadlineExceeded` | `time.Sleep(deadline + 5s)` | `ErrCodeTimeoutError` in JSON output |
| `MissingBinary` | Binary path does not exist | Error containing file path |

### Cache Test Fixtures

| Fixture | Content | Purpose |
|---------|---------|---------|
| Corrupted DB | Random bytes (128 bytes) | Corruption recovery (US-2) |
| Valid projected entry | `{"monthly_cost": 10.50, "currency": "USD"}` | Cache hit verification |
| Expired entry | Entry with TTL in the past | TTL expiration verification |

### Config Test Directory Trees

```text
# Precedence test fixture
tmp/
├── global/                    # Simulates ~/.finfocus/
│   └── config.yaml            # budget.limit: 50
├── project/                   # Simulates Pulumi project root
│   ├── Pulumi.yaml            # Project marker
│   └── .finfocus/
│       └── config.yaml        # budget.limit: 100
└── project/subdir/            # CWD for walk-up test
```

### Synthetic Stack Resources

For analyzer 100-resource test (US-3):

```go
// Generated programmatically
resources := make([]*pulumirpc.AnalyzerResource, 100)
for i := range resources {
    resources[i] = &pulumirpc.AnalyzerResource{
        Type: fmt.Sprintf("aws:ec2/instance:Instance"),
        Properties: &structpb.Struct{...},
        Name: fmt.Sprintf("test-resource-%d", i),
        Urn: fmt.Sprintf("urn:pulumi:stack::project::aws:ec2/instance:Instance::test-%d", i),
    }
}
```

### Synthetic Large Plans

For engine concurrency test (US-5):

```go
// 500-resource plan for -j1 vs -j8 comparison
plan := generateSyntheticPlan(500, []string{
    "aws:ec2/instance:Instance",
    "aws:s3/bucket:Bucket",
    "aws:rds/instance:Instance",
})
```

## State Machines

### TUI ViewState Transitions (Test Coverage Map)

```text
Initializing ──[OverviewDataReadyMsg]──→ Loading
    │                                        │
    │    ┌──[OverviewAllResourcesLoadedMsg]───┘
    │    ▼
    │  List ←──[Escape]──── Detail
    │    │                    ▲
    │    └───[Enter]──────────┘
    │    │
    │    └───['q']──→ Quitting
    │
    └──[OverviewInitErrorMsg]──→ Error
```

**Test Assertions Per Transition**:

| From | To | Trigger | Assert |
|------|----|---------|--------|
| Initializing | Loading | `OverviewDataReadyMsg` | `model.viewState == ViewStateLoading` |
| Loading | List | `OverviewAllResourcesLoadedMsg` | `model.viewState == ViewStateList` |
| List | Detail | `tea.KeyMsg{Type: tea.KeyEnter}` | `model.viewState == ViewStateDetail` |
| Detail | List | `tea.KeyMsg{Type: tea.KeyEscape}` | `model.viewState == ViewStateList` |
| List | Quitting | `tea.KeyMsg{Runes: []rune{'q'}}` | `tea.Quit` cmd returned |
| Any | Error | `OverviewInitErrorMsg` | `model.viewState == ViewStateError` |

## Relationships

### Test File to Subsystem Mapping

| Test File | Subsystem | Build Tag | Priority |
|-----------|-----------|-----------|----------|
| `test/integration/plugin_resilience_test.go` | Plugin Host | integration | P1 |
| `test/integration/cache_system_test.go` | Cache | integration | P1 |
| `test/integration/analyzer_concurrency_test.go` | Analyzer | integration | P2 |
| `test/integration/config_precedence_test.go` | Config | integration | P2 |
| `test/integration/concurrency_correctness_test.go` | Engine | integration | P2 |
| `test/integration/trace_propagation_test.go` | CI Tags | integration (promoted) | P3 |
| `test/integration/tui_state_machine_test.go` | TUI | integration | P3 |
