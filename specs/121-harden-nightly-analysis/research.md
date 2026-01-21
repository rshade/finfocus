# Research Findings - Harden Nightly Analysis Workflow

**Status**: Complete

## 1. Dependency Versions

**Decision**: 
- Pin `@opencode/cli` to **1.0.128**.
  - *Note*: Found version `1.0.128` used in `opencode-code-review.yml`. Adopting this as the stable pinned version instead of the placeholder.

- Pin `actions/checkout` to **v6**.
  - Verified: Multiple workflows (`ci.yml`, `nightly.yml`, etc.) already use `actions/checkout@v6`. This is the established project standard.

**Rationale**:
- **@opencode/cli**: Registry access is restricted/mocked. Will proceed with placeholder approach.
- **actions/checkout**: `v6` is pervasively used in the repo.

## 2. Permissions

**Decision**: Use `permissions: contents: read` (plus potentially `packages: read` or specific repo access).

**Rationale**:
- `GITHUB_TOKEN` needs explicit scope to read other repositories if they are private/internal.
- If target repos are public, standard token might suffice.

**Action Items**:
- Use `actions/checkout@v6` in the implementation.
- Use `npm install -g @opencode/cli@1.0.0`.
