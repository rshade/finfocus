# Data Model: Test Quality and CI/CD Improvements

**Date**: 2026-01-19
**Branch**: `118-test-quality-ci`

## Overview

This feature modifies test infrastructure rather than production data models. The primary data structures are test fixtures and workflow configurations.

## Test Fixture Schemas

### Pulumi Plan JSON (for Fuzz Tests)

```json
{
  "steps": [
    {
      "op": "create|update|delete|same|replace",
      "urn": "urn:pulumi:stack::project::type::name",
      "type": "provider:module/resource:Resource",
      "oldState": { /* optional, for updates */ },
      "newState": {
        "type": "provider:module/resource:Resource",
        "urn": "string",
        "inputs": {
          "key": "value"
        }
      }
    }
  ]
}
```

**Validation Rules**:

- `steps` array is required (may be empty)
- `op` must be one of: create, update, delete, same, replace
- `urn` follows Pulumi URN format
- `newState` contains resource details for create/update operations

### Pulumi State JSON (for ActualCost E2E)

Uses existing fixtures in `test/fixtures/state/`:

| Field                          | Type   | Required | Description               |
| ------------------------------ | ------ | -------- | ------------------------- |
| version                        | int    | Yes      | State format version (3)  |
| deployment.manifest.time       | string | Yes      | ISO 8601 timestamp        |
| deployment.resources           | array  | Yes      | Resource list             |
| deployment.resources[].urn     | string | Yes      | Resource URN              |
| deployment.resources[].type    | string | Yes      | Resource type             |
| deployment.resources[].custom  | bool   | Yes      | Is custom resource        |
| deployment.resources[].external| bool   | No       | Is imported resource      |
| deployment.resources[].created | string | No       | ISO 8601 creation time    |
| deployment.resources[].modified| string | No       | ISO 8601 modification time|
| deployment.resources[].inputs  | object | No       | Resource inputs           |
| deployment.resources[].outputs | object | No       | Resource outputs          |

### Cost Result JSON (NDJSON Output)

```json
{
  "resource_type": "aws:ec2/instance:Instance",
  "resource_name": "web-server",
  "monthly_cost": 50.00,
  "currency": "USD",
  "notes": "optional explanation"
}
```

**Validation Rules**:

- `resource_type` is required
- `currency` is required (ISO 4217 code)
- Either `monthly_cost` or `actual_cost` present for cost results

## Workflow Configuration Schema

### Cross-Repo Integration Workflow

```yaml
on:
  workflow_dispatch:
    inputs:
      plugin_ref:
        type: string
        default: 'main'
      core_ref:
        type: string
        default: 'main'
  schedule:
    - cron: '0 2 * * *'
```

**Validation Rules**:

- Schedule must use valid cron syntax
- Input refs must resolve to valid git refs (branch, tag, SHA)

## State Transitions

### Workflow Execution States

```text
Pending → Running → Success
                 → Failure → Issue Created
```

### Test Fixture Lifecycle

Test fixtures are static JSON files - no state transitions. They are:

1. Created once during feature development
2. Read during test execution
3. Never modified at runtime

## Relationships

```text
Fuzz Test ──uses──> Seed Corpus Entries (inline)

Benchmark Test ──generates──> Synthetic Plan JSON

ActualCost E2E Test ──reads──> test/fixtures/state/*.json
                    ──validates──> NDJSON Output Schema

Cross-Repo Workflow ──checks out──> finfocus-core
                    ──checks out──> finfocus-plugin-aws-public
                    ──on failure──> Creates GitHub Issue
```

## No New Database Entities

This feature does not introduce persistent storage. All data is either:

- Inline seed corpus (fuzz tests)
- Generated at runtime (benchmarks)
- Static fixture files (E2E tests)
- Workflow configuration (YAML)
