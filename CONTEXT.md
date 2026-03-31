# finfocus Project Context & Boundaries

This document defines the technical guardrails and architectural scope of the `finfocus` project. It serves as the primary reference for maintaining system integrity and preventing scope creep or architectural hallucinations.

## Core Architectural Identity
**Stateless CLI Orchestrator for Pulumi Infrastructure Cost Estimation**

`finfocus` is a lightweight, agnostic engine designed to bridge Pulumi infrastructure definitions with pricing data. It functions as a **transformer and aggregator**, not a primary data source. Its primary roles are:
1.  **Ingestion**: Extracting resource metadata from multiple sources:
    *   Planned changes (`pulumi preview --json`).
    *   Current infrastructure state (`pulumi state export` or via `stdin`).
    *   Real-time analysis via the Pulumi Analyzer plugin interface.
2.  **Orchestration**: Routing resource descriptors to specialized gRPC plugins or local YAML pricing specifications.
3.  **Aggregation**: Normalizing and summing costs across providers, services, and time periods.
4.  **Presentation**: Displaying results via CLI, JSON, or interactive TUI.

## Technical Boundaries ("Hard No's")
To maintain its role as a lightweight orchestrator, the following are explicitly **out of scope**:
*   **Direct Cloud API Calls**: The core engine MUST NOT call cloud provider pricing APIs (e.g., AWS Price List API) or usage APIs directly. All provider-specific logic belongs in plugins.
*   **Required Persistent State**: The core execution model is stateless — every command MUST produce correct results without any prior local state. However, finfocus uses **optional local persistence** (BoltDB) as a performance and accuracy aid:
    *   **Cost cache** (`~/.finfocus/cache/cache.db`): Avoids redundant plugin queries. Safe to delete; results rebuild on next run.
    *   **Resource history** (`~/.finfocus/history/history.db`): Tracks resource identity across billing periods for accurate month-long actual cost queries. Safe to delete; accuracy degrades for past periods but current queries still function.
    *   **Recommendation dismissals** (`dismissed.json`): Persists user dismissal decisions.
    *   The distinction: finfocus **functions correctly** without any persisted state, just with **degraded performance** (no cache) or **reduced accuracy** (no history for replaced/destroyed resources). No persistent state is a hard prerequisite for any command to succeed.
*   **Infrastructure Management**: It is a "read-only" tool. While it may invoke `pulumi` CLI commands (like `state export`) to read infrastructure definitions, it MUST NOT perform `pulumi up`, `pulumi destroy`, or any operation that modifies cloud state.
*   **Baked-in Provider Logic**: The core engine MUST NOT contain hardcoded logic for specific cloud services (e.g., "how to calculate S3 pricing"). This logic is strictly delegated to plugins or YAML specs.
*   **Financial Accounting**: The tool handles cost *estimation* and *projection*. It is NOT a ledger, invoice matching system, or tax calculation engine.

## Data Source of Truth
Accuracy and data ownership are distributed as follows:
*   **Infrastructure Schema**: Multiple authorities define what resources exist and their configuration:
    *   **Planned**: `pulumi preview --json` output.
    *   **Deployed**: `pulumi state export` (JSON) or direct state inspection.
    *   **Active Analysis**: The Pulumi Analyzer plugin interface.
*   **Pricing Data**: 
    *   **External**: Responsibility lies with the gRPC plugins (e.g., `finfocus-provider-aws`).
    *   **Local**: Responsibility lies with the YAML files in the `specs/` directory (e.g., `aws-ec2-t3.medium.yaml`).
*   **Normalization**: The core engine defines the `proto` and `internal/engine` types which serve as the "Contract of Truth" for how cost data must be returned.

## Interaction Model
*   **Inbound**: 
    *   CLI Flags and Arguments.
    *   Filesystem (Reading Pulumi JSON plans, state files, and Local Specs).
    *   Standard Input (`stdin`) for piped Pulumi state or plans.
    *   Subprocess Execution (invoking `pulumi state export` on behalf of the user).
*   **Outbound**: 
    *   **gRPC**: Communication with provider plugins and as a Pulumi Analyzer service.
    *   **Standard Streams**: stdout (Results), stderr (Logs/Errors).
*   **User Interface**: 
    *   Static text/table output.
    *   Interactive terminal UI (TUI) for data exploration.

## Verification
Any proposed feature that introduces provider-specific API clients, persistent databases, or direct infrastructure modification should be rejected as a violation of this project's core boundaries.
