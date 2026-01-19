# Contracts: Test Quality and CI/CD Improvements

**Date**: 2026-01-19
**Branch**: `118-test-quality-ci`

## Overview

This feature does not introduce new API contracts. It modifies test infrastructure and CI/CD workflows.

## Relevant Existing Contracts

The following existing contracts are relevant to the test implementations:

### 1. Cost Result Output (NDJSON)

**Location**: `internal/engine/output.go`

The NDJSON output format is validated by the enhanced E2E test:

```json
{
  "resource_type": "string (required)",
  "monthly_cost": "number (optional)",
  "actual_cost": "number (optional)",
  "currency": "string (required, ISO 4217)"
}
```

### 2. Pulumi State Format

**Location**: `internal/ingest/state.go`

The state file format parsed by the ActualCost command:

```json
{
  "version": 3,
  "deployment": {
    "resources": [{
      "urn": "string",
      "type": "string",
      "created": "ISO 8601 timestamp (optional)",
      "modified": "ISO 8601 timestamp (optional)",
      "external": "boolean (optional)"
    }]
  }
}
```

### 3. Pulumi Plan Format

**Location**: `internal/ingest/pulumi_plan.go`

The plan JSON format parsed by benchmarks and fuzz tests:

```json
{
  "steps": [{
    "op": "create|update|delete|same|replace",
    "urn": "string",
    "type": "string",
    "newState": {
      "inputs": {}
    }
  }]
}
```

## No New API Endpoints

This feature does not create new REST/GraphQL endpoints. All changes are internal to the test infrastructure.
