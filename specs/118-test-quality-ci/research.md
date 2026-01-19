# Research: Test Quality and CI/CD Improvements

**Date**: 2026-01-19
**Branch**: `118-test-quality-ci`

## Research Summary

All technical unknowns have been resolved through codebase analysis.

## Decision 1: Fuzz Test Seed Corpus Format

**Decision**: Add `newState` seed entries using existing pattern in `internal/ingest/fuzz_test.go`

**Rationale**:

- Existing fuzz tests use `f.Add([]byte(...))` pattern
- The `newState` structure is critical for Pulumi preview JSON parsing
- Current corpus lacks `newState` entries which is how Pulumi v3+ provides resource details

**Alternatives Considered**:

- External corpus files in testdata/fuzz - Rejected: Go 1.18+ fuzz prefers inline seed corpus
- Separate fuzz target for newState - Rejected: Same parser, just different input shape

**Implementation Pattern**:

```go
// Add to existing FuzzJSON
f.Add([]byte(`{"steps":[{"op":"create","urn":"test","newState":{"type":"aws:ec2/instance:Instance","urn":"test","inputs":{"instanceType":"t3.micro"}}}]}`))
f.Add([]byte(`{"steps":[{"op":"update","urn":"test","oldState":{"type":"aws:s3/bucket:Bucket"},"newState":{"type":"aws:s3/bucket:Bucket","inputs":{"bucket":"my-bucket"}}}]}`))
```

## Decision 2: Benchmark JSON Structure

**Decision**: Replace `resourceChanges` with `steps` array format

**Rationale**:

- Current benchmarks use legacy `resourceChanges` which the ingest parser doesn't recognize
- Pulumi preview JSON uses `steps` array with `op`, `urn`, `type` fields
- Benchmarks should measure realistic parsing performance

**Alternatives Considered**:

- Keep both formats for comparison - Rejected: Maintains misleading benchmark
- Add new benchmark, keep old - Rejected: No value in benchmarking invalid format

**Implementation Pattern**:

```go
// Replace resource generation
resources[i] = fmt.Sprintf(
    `{"op":"create","urn":"urn:pulumi:dev::test::aws:ec2/instance:Instance::i-%d","type":"aws:ec2/instance:Instance","inputs":{"instanceType":"t3.micro"}}`,
    i,
)
// Replace JSON structure
jsonStr := fmt.Sprintf(`{"steps": [%s]}`, strings.Join(resources, ","))
```

## Decision 3: Cross-Repo Integration Workflow

**Decision**: Create new workflow `.github/workflows/cross-repo-integration.yml`

**Rationale**:

- Existing `nightly.yml` already has failure notification pattern (creates GitHub issue)
- Cross-repo testing requires separate workflow with plugin repository checkout
- `workflow_dispatch` allows manual testing; nightly schedule catches regressions

**Alternatives Considered**:

- Add job to existing nightly.yml - Rejected: Different concerns, cleaner separation
- Use reusable workflow - Rejected: Overkill for single use case

**Key Patterns from Existing Workflows**:

1. Use `actions/checkout@v6` with path separation for multiple repos
2. Use `actions/setup-go@v6` with `go-version: '1.25.5'` and `cache: true`
3. Use `actions/github-script@v8` for issue creation on failure
4. Set `permissions: { contents: read, issues: write }`

## Decision 4: E2E Test Fixtures Location

**Decision**: Use existing `test/fixtures/state/` directory for ActualCost test fixtures

**Rationale**:

- Fixtures already exist: `valid-state.json`, `imported-resources.json`, `no-timestamps.json`
- These fixtures have correct structure with `created`, `modified` timestamps
- No need to create new fixtures - existing ones cover all required scenarios

**Alternatives Considered**:

- Create new fixtures in `test/e2e/fixtures/` - Rejected: Duplicates existing test data
- Generate fixtures dynamically - Rejected: Deterministic fixtures preferred for E2E

**Existing Fixtures Analysis**:

| Fixture                 | Has Timestamps | Has External | Use Case                    |
| ----------------------- | -------------- | ------------ | --------------------------- |
| valid-state.json        | Yes            | No           | Primary actual cost test    |
| imported-resources.json | Yes            | Yes          | External resource handling  |
| no-timestamps.json      | No             | No           | Missing timestamp edge case |

## Decision 5: Test Error Handling Pattern

**Decision**: Use `require.NoError(t, err)` for all fallible operations in test setup

**Rationale**:

- Existing pattern in `output_ndjson_test.go` (line 22) uses this correctly
- `gcp_test.go` (line 19) incorrectly ignores error with `_, _`
- Testify `require` fails fast vs `assert` which continues

**Implementation Pattern**:

```go
// Correct pattern (already in output_ndjson_test.go)
planPath, err := filepath.Abs("../fixtures/plans/aws/simple.json")
require.NoError(t, err)

// Incorrect pattern (in gcp_test.go line 19)
planPath, _ := filepath.Abs("../fixtures/plans/gcp/simple.json")  // BAD
```

## Decision 6: NDJSON Schema Validation

**Decision**: Add field presence checks for cost result schema

**Rationale**:

- Current test only validates JSON syntax (line 35)
- Cost results should have `resource_type` and `currency` at minimum
- Schema validation catches output format regressions

**Implementation Pattern**:

```go
// After JSON unmarshal succeeds
if _, hasCost := obj["monthly_cost"]; hasCost {
    assert.Contains(t, obj, "resource_type", "Missing resource_type field")
    assert.Contains(t, obj, "currency", "Missing currency field")
}
```

## No Outstanding Research Items

All technical decisions have been made. Ready for Phase 1 design artifacts.
