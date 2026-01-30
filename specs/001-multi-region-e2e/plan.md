# Implementation Plan: Multi-Region E2E Testing

**Branch**: `001-multi-region-e2e` | **Date**: 2026-01-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-multi-region-e2e/spec.md`

## Summary

Implement multi-region E2E testing infrastructure to validate both projected and actual cost calculations across AWS regions (us-east-1, eu-west-1, ap-northeast-1) with ±5% variance tolerance. This includes User Stories 1-3 (per-region fixtures) and User Story 4 (unified multi-region fixture using YAML runtime).

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: testify v1.11.1, Pulumi Automation API v3.210.0+, existing E2E test framework (`test/e2e/`)
**Storage**: Local filesystem (Pulumi state files, test fixtures in `test/e2e/fixtures/`)
**Testing**: `go test` with `-tags e2e` build constraint, testify assertions
**Target Platform**: Linux, macOS, Windows (CI matrix)
**Project Type**: Single project (Go CLI with test infrastructure)
**Performance Goals**: <5 minutes per region test execution
**Constraints**: ±5% cost variance tolerance, 80% minimum test coverage
**Scale/Scope**: 8 resources per region × 3 regions = 24 resources validated, plus unified fixture

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is test infrastructure that validates plugin behavior - not implementing new plugins
- [x] **Test-Driven Development**: Tests are the feature itself - E2E validation of cost calculations
- [x] **Cross-Platform Compatibility**: E2E tests use Go standard testing which is cross-platform
- [x] **Documentation Synchronization**: quickstart.md and docs/testing/multi-region-e2e.md planned
- [x] **Protocol Stability**: No protocol changes - uses existing gRPC plugin communication
- [x] **Implementation Completeness**: All fixtures and test functions will be complete implementations
- [x] **Quality Gates**: Tests must pass `make lint` and `make test-e2e`
- [x] **Multi-Repo Coordination**: Tests validate plugins from finfocus-plugin repo; no cross-repo code changes

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/001-multi-region-e2e/
├── plan.md              # This file
├── research.md          # Phase 0 output (complete)
├── data-model.md        # Phase 1 output (complete)
├── quickstart.md        # Phase 1 output (complete)
├── checklists/          # Validation checklists
│   └── requirements.md
└── tasks.md             # Phase 2 output (complete with US4)
```

### Source Code (repository root)

```text
test/e2e/
├── fixtures/multi-region/
│   ├── us-east-1/           # US East baseline region fixture (YAML runtime)
│   │   ├── Pulumi.yaml      # 8 resources: 2 EC2, 2 EBS, 2 Network, 2 RDS
│   │   └── expected-costs.json
│   ├── eu-west-1/           # EU West region fixture (YAML runtime)
│   │   ├── Pulumi.yaml      # 8 resources with EU pricing
│   │   └── expected-costs.json
│   ├── ap-northeast-1/      # Asia Pacific region fixture (YAML runtime)
│   │   ├── Pulumi.yaml      # 8 resources with APAC pricing
│   │   └── expected-costs.json
│   └── unified/             # User Story 4 - Unified multi-region fixture
│       ├── Pulumi.yaml      # YAML runtime with explicit provider aliases
│       └── expected-costs.json
├── config.go                # Test configuration with region support
├── validator.go             # Cost validation with tolerance checking
├── multi_region_helpers.go  # Shared utilities for multi-region tests
├── multi_region_projected_test.go  # Projected cost tests
├── multi_region_actual_test.go     # Actual cost tests
└── multi_region_fallback_test.go   # Fallback behavior tests

docs/testing/
└── multi-region-e2e.md      # Documentation for multi-region E2E testing
```

**Structure Decision**: All fixtures use Pulumi YAML runtime for simplicity - no Go code required. Extends existing `test/e2e/` infrastructure with multi-region fixtures and new test files following established patterns.

## User Story 4: Unified Multi-Region Fixture

### Background

User Stories 1-3 test **separate Pulumi programs per region**. User Story 4 tests a **single Pulumi program that deploys resources across multiple regions** - a common real-world pattern.

### Key Differences from Per-Region Fixtures

- **Structure**: Per-region has 3 separate `Pulumi.yaml` files, Unified has 1 `Pulumi.yaml` with all regions
- **Providers**: Per-region uses single implicit AWS provider per fixture, Unified uses explicit provider aliases per region
- **Plan JSON**: Per-region has single region per plan, Unified has multi-region with provider references
- **Cost Attribution**: Per-region by fixture directory, Unified by provider → resource mapping
- **Validation**: Per-region validates per-region totals, Unified validates per-resource + aggregate total

### Technical Approach

**YAML Runtime for All Fixtures**: All multi-region fixtures (per-region and unified) use Pulumi YAML runtime because:

- No Go code required - simpler to maintain and less build complexity
- Provider configuration declared inline in YAML
- Explicit region configuration per resource via `options.provider` (unified) or stack config (per-region)
- Validates FinFocus region autodetection from plan JSON
- Consistent approach across all user stories

**Plugin Discovery Question** (T028):
The `aws-public` plugin must either:

1. **Option A**: Single instance handles all regions (autodetects from plan JSON)
2. **Option B**: Separate instances per region (explicit config required)

Task T035 will determine which approach works via discovery testing.

### Unified Fixture Design

```yaml
# test/e2e/fixtures/multi-region/unified/Pulumi.yaml
name: multi-region-test-unified
runtime: yaml
description: Unified multi-region E2E test fixture

resources:
  # Provider per region
  aws-us-east-1:
    type: pulumi:providers:aws
    properties:
      region: us-east-1

  aws-eu-west-1:
    type: pulumi:providers:aws
    properties:
      region: eu-west-1

  aws-ap-northeast-1:
    type: pulumi:providers:aws
    properties:
      region: ap-northeast-1

  # Resources with explicit provider
  web-us-east-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-0123456789abcdef0
      instanceType: t3.micro
    options:
      provider: ${aws-us-east-1}

  web-eu-west-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-0abcdef123456789a
      instanceType: t3.micro
    options:
      provider: ${aws-eu-west-1}

  web-ap-northeast-1:
    type: aws:ec2:Instance
    properties:
      ami: ami-0fedcba987654321f
      instanceType: t3.micro
    options:
      provider: ${aws-ap-northeast-1}

outputs:
  regions:
    - us-east-1
    - eu-west-1
    - ap-northeast-1
```

### Expected Costs Structure (Unified)

```json
{
  "fixture_type": "unified",
  "total_regions": 3,
  "resources": [
    {
      "resource_name": "web-us-east-1",
      "resource_type": "aws:ec2:Instance",
      "region": "us-east-1",
      "min_cost": 6.66,
      "max_cost": 7.36,
      "cost_type": "projected"
    },
    {
      "resource_name": "web-eu-west-1",
      "resource_type": "aws:ec2:Instance",
      "region": "eu-west-1",
      "min_cost": 7.13,
      "max_cost": 8.47,
      "cost_type": "projected"
    },
    {
      "resource_name": "web-ap-northeast-1",
      "resource_type": "aws:ec2:Instance",
      "region": "ap-northeast-1",
      "min_cost": 7.33,
      "max_cost": 8.83,
      "cost_type": "projected"
    }
  ],
  "aggregate_validation": {
    "total_min_cost": 21.12,
    "total_max_cost": 24.66,
    "tolerance": 0.05
  }
}
```

### Validation Logic for Unified Fixtures

```go
// ValidateUnifiedFixtureCosts validates costs for a unified multi-region fixture
func ValidateUnifiedFixtureCosts(results []CostResult, expected UnifiedExpectedCosts) (*UnifiedValidationResult, error) {
    result := &UnifiedValidationResult{
        PerResourceResults: make([]CostValidationResult, 0, len(expected.Resources)),
    }

    // Validate each resource
    for _, exp := range expected.Resources {
        actual := findResultByName(results, exp.ResourceName)
        if actual == nil {
            return nil, fmt.Errorf("missing cost result for resource %s", exp.ResourceName)
        }

        validation := CostValidationResult{
            ResourceName:    exp.ResourceName,
            ResourceType:    exp.ResourceType,
            Region:          exp.Region,
            ActualCost:      actual.MonthlyCost,
            ExpectedMin:     exp.MinCost,
            ExpectedMax:     exp.MaxCost,
            WithinTolerance: actual.MonthlyCost >= exp.MinCost && actual.MonthlyCost <= exp.MaxCost,
        }
        result.PerResourceResults = append(result.PerResourceResults, validation)
    }

    // Validate aggregate total
    totalCost := sumCosts(results)
    result.TotalCost = totalCost
    result.TotalWithinTolerance = totalCost >= expected.AggregateValidation.TotalMinCost &&
        totalCost <= expected.AggregateValidation.TotalMaxCost

    return result, nil
}
```

## Complexity Tracking

> No constitution violations - no entries required.

## Phase Implementation Summary

### Completed Phases (US1-3)

- **Phase 1: Setup** - Multi-region fixture directories created
- **Phase 2: Foundational** - Config, validation, retry logic implemented
- **Phase 3: User Story 1** - Per-region projected/actual cost tests
- **Phase 4: User Story 2** - Plugin loading verification
- **Phase 5: User Story 3** - Fallback behavior tests
- **Phase 6: Polish** - Documentation, performance validation

### Phase 7: User Story 4 (NEW)

- **T028**: Discovery - Plugin behavior with multi-region plans (Pending)
- **T029**: Create unified fixture directory structure (Pending)
- **T030**: Create unified Pulumi.yaml with YAML runtime (Pending)
- **T031**: Generate expected-costs.json for unified fixture (Pending)
- **T032**: Add UnifiedExpectedCosts structs (Pending)
- **T033**: Implement ValidateUnifiedFixtureCosts (Pending)
- **T034**: Create TestMultiRegion_Unified_Projected test (Pending)
- **T035**: Update docs/testing/multi-region-e2e.md (Pending)

## Open Questions

### Q1: Plugin Configuration for Unified Fixtures

**Status**: To be resolved by T028 (Discovery task)

**Question**: Does `aws-public` plugin automatically detect regions from plan JSON, or require explicit per-region configuration?

**Options**:

- **A**: Single `aws-public` declaration, autodetection from provider resources in plan
- **B**: Explicit `aws-public-us-east-1`, `aws-public-eu-west-1`, etc. declarations

**Resolution approach**: Run unified fixture through `finfocus cost projected` and observe plugin spawning behavior.

**Decision Tree** (T028 outcomes):

```text
T028 Discovery Test
        │
        ▼
┌───────────────────────────────────────┐
│ Run: pulumi preview --json            │
│ Run: finfocus cost projected --debug  │
└───────────────────────────────────────┘
        │
        ▼
    Observe plugin spawning
        │
        ├─── Single aws-public process, all regions priced correctly
        │         │
        │         ▼
        │    ✅ Option A: Proceed with single plugin declaration
        │       - No config changes needed
        │       - Document autodetection behavior
        │
        ├─── Multiple plugin processes spawned per region
        │         │
        │         ▼
        │    ✅ Option B: Use explicit per-region plugin declarations
        │       - Update finfocus.yaml with aws-public-{region} entries
        │       - Document required configuration
        │
        └─── Errors or incorrect region pricing
                  │
                  ▼
             ⚠️ Escalate: Ingestion layer needs enhancement
                - File issue for internal/ingest/pulumi_plan.go
                - Update spec.md with new requirement
                - Block US4 until ingestion fixed
```

### Q2: Region Extraction from Provider Resources

**Status**: Research needed during T028

**Question**: How does FinFocus extract region from provider configuration in plan JSON?

**Current understanding**: Plan JSON includes provider configuration in step structure; ingestion should extract `aws:region` property from provider resources.

## References

- [Spec](./spec.md) - Full feature specification with User Stories 1-4
- [Tasks](./tasks.md) - Implementation task list with Phase 7 for US4
- [Research](./research.md) - Research findings for US1-3
- [Data Model](./data-model.md) - Entity definitions and validation rules
- [Quickstart](./quickstart.md) - Developer guide for running tests
