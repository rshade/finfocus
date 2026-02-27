# Data Model: Analyzer Quality Cluster

**Branch**: `603-analyzer-quality` | **Date**: 2026-02-24

## Entities

### 1. CheckResult (New)

Represents the outcome of a single verification step in `analyzer check`.

**Location**: `internal/analyzer/check.go`

```go
// CheckResult represents the outcome of a single analyzer setup verification.
type CheckResult struct {
    Name        string `json:"name"`        // Check identifier (e.g., "policy_pack_dir")
    DisplayName string `json:"display_name"` // Human-readable name
    Status      string `json:"status"`       // "pass", "fail", or "skip"
    Message     string `json:"message"`      // Detail message
    Remediation string `json:"remediation,omitempty"` // Actionable fix (only on fail)
}
```

**Status values**:

| Status | Meaning |
|--------|---------|
| `pass` | Check succeeded |
| `fail` | Check failed, remediation provided |
| `skip` | Check skipped due to prerequisite failure |

### 2. CheckReport (New)

Aggregates all check results for a single `analyzer check` invocation.

**Location**: `internal/analyzer/check.go`

```go
// CheckReport aggregates all check results for analyzer setup verification.
type CheckReport struct {
    Checks  []CheckResult `json:"checks"`
    AllPass bool          `json:"all_pass"`
}
```

### 3. InstallResult (Modified)

Extended to include policy pack directory information.

**Location**: `internal/analyzer/install.go`

**New fields**:

```go
type InstallResult struct {
    // ... existing fields ...

    // PolicyPackDir is the path to the policy pack directory (empty if not set up).
    PolicyPackDir string `json:"policy_pack_dir,omitempty"`

    // PolicyPackMethod is "symlink" or "copy" for the policy pack binary.
    PolicyPackMethod string `json:"policy_pack_method,omitempty"`
}
```

### 4. InstallOptions (Modified)

No changes needed. Policy pack setup is the default behavior during install
(per spec assumption: "Policy pack setup is performed by default during
`finfocus analyzer install`").

### 5. CostSummary (Existing — No Changes)

Already correctly handles cost aggregation. The fix for #746 is in the
diagnostic function, not the data model.

### 6. PolicyPackConfig (New — Internal)

Represents the `PulumiPolicy.yaml` content.

**Location**: `internal/analyzer/policypack.go`

```go
// PolicyPackConfig represents the PulumiPolicy.yaml configuration.
type PolicyPackConfig struct {
    Name        string `yaml:"name"`
    Runtime     string `yaml:"runtime"`
    Description string `yaml:"description"`
}
```

**Default values**:

```yaml
name: finfocus
runtime: finfocus
description: FinFocus cost estimation analyzer
```

## Relationships

```text
InstallResult
  └── PolicyPackDir (path to policy pack)
        └── PolicyPackConfig (PulumiPolicy.yaml)
        └── Binary reference (symlink/copy)

CheckReport
  └── []CheckResult (ordered sequence)
        ├── policy_pack_dir → checks PolicyPackDir exists
        ├── pulumi_policy_yaml → checks PolicyPackConfig validity
        ├── binary_in_path → checks exec.LookPath
        └── grpc_smoke_test → checks server starts + responds
```

## State Transitions

### Install Flow

```text
NotInstalled → Install() → Installed (Pulumi plugin + policy pack dir)
Installed    → Install(Force) → Reinstalled (both updated)
Installed    → Uninstall() → NotInstalled
```

### Check Flow

```text
[Start] → CheckPolicyPackDir
  ├── PASS → CheckPulumiPolicyYAML
  │           ├── PASS → CheckBinaryInPATH
  │           │           ├── PASS → CheckGRPCSmokeTest
  │           │           │           ├── PASS → AllPass=true
  │           │           │           └── FAIL → AllPass=false
  │           │           └── FAIL → SkipGRPC → AllPass=false
  │           └── FAIL → SkipBinary, SkipGRPC → AllPass=false
  └── FAIL → SkipYAML, SkipBinary, SkipGRPC → AllPass=false
```

## Validation Rules

### PolicyPackConfig

- `Name` must equal `"finfocus"`
- `Runtime` must equal `"finfocus"`
- `Description` can be any non-empty string

### CheckResult

- `Status` must be one of: `"pass"`, `"fail"`, `"skip"`
- `Remediation` should only be set when `Status == "fail"`

### InstallResult

- `PolicyPackDir` is set when policy pack setup succeeds
- `PolicyPackMethod` mirrors `Method` ("symlink" or "copy")
