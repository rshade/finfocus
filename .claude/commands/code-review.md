---
allowed-tools: Bash(git diff:*), Bash(git log:*), Bash(git status), Bash(make lint), Bash(make test), Bash(go build:*), Bash(go mod tidy)
description: Perform comprehensive Go code review and fix all identified issues
---

# Code Review and Fix Protocol

## Phase 1: Code Review

Use the **golang-code-reviewer** agent to perform a comprehensive review of
recent changes focusing on:

### Review Criteria

1. **Code Quality and Best Practices**
   - Readability, maintainability, and adherence to Go best practices
   - Proper error handling and return value checking
   - Interface design and abstraction patterns
   - Resource cleanup (defer statements, context handling)

2. **Security Concerns**
   - Potential vulnerabilities or security issues
   - Input validation and sanitization
   - Secret handling and credential management
   - File permissions and subprocess usage

3. **Performance Considerations**
   - Potential performance bottlenecks
   - Memory allocation patterns and efficiency
   - Concurrent code safety and race conditions
   - Algorithm complexity and optimization opportunities

4. **Testing and Coverage**
   - Test coverage and quality assessment
   - Edge case handling in tests
   - Test organization and clarity
   - Mock usage and test isolation

5. **Documentation**
   - Package and exported type documentation
   - Function comment completeness
   - Code comment clarity and necessity
   - README and architecture documentation

6. **FinFocus-Specific Requirements**
   - Cross-repo consistency (spec, core, plugin alignment)
   - Plugin protocol implementation correctness
   - Cost calculation logic accuracy
   - CLI UX and error messaging quality

7. **Go Version Idioms (Go 1.25.6+)**
   - Use Go Go 1.25.6+ language features and standard library improvements
   - Leverage modern Go patterns and best practices
   - Ensure compatibility with Go Go 1.25.6+ runtime behavior
   - Utilize updated standard library APIs and optimizations

### Context for Review

- Current branch: !`git branch --show-current`
- Recent changes: !`git diff HEAD~1`
- Recent commits: !`git log --oneline -5`
- Current git status: !`git status`
- Current linting issues: !`make lint`
- Current test status: !`make test`

**Task**: Launch the golang-code-reviewer agent to analyze the changes and
provide detailed, actionable feedback with file:line references.

---

## Phase 2: Fix Identified Issues

After the golang-code-reviewer agent completes, use the
**finfocus-senior-engineer** agent to systematically fix ALL identified
issues using this protocol:

### 🚨 EXECUTION PROTOCOL

#### Step 1: Problem Comprehension (MANDATORY)

1. **Review Analysis**: Read and understand ALL issues identified by golang-code-reviewer
2. **Create Todos**: Use TodoWrite to create individual todos for EVERY issue
   - Format: "Fix [issue-type] in [file]:[line] - [description]"
   - Include priority and category for each issue
3. **Categorize Issues**:
   - Security issues (highest priority)
   - Correctness/functionality issues
   - Code quality and maintainability
   - Style and formatting
   - Documentation

#### Step 2: Atomic Single-Change Workflow (BLOCKING)

##### ONE ISSUE AT A TIME - NO EXCEPTIONS

For each issue:

1. **ONE CHANGE ONLY**:
   - Mark todo as "in_progress"
   - Fix EXACTLY ONE issue
   - Make ONLY minimal changes required

2. **IMMEDIATE VERIFICATION** (BLOCKING):
    - If you changed imports: `go mod tidy` FIRST
    - Test compilation: `go build ./...`
    - Run validation: `make fmt && make lint && make test`
    - **CRITICAL**: Check exit codes after EACH command - use `echo $?` to verify
    - **CRITICAL**: Verify ALL commands exit with code 0
    - If ANY validation fails → rollback immediately and try different approach
    - NEVER proceed with failing tests or lint errors

3. **ROLLBACK PROTOCOL**:
   - If validation fails → revert immediately
   - Clean up broken files
   - Try different approach
   - NEVER leave codebase broken

4. **COMPLETION**:
   - Mark todo "completed" ONLY after validation passes
   - Move to next issue

#### Step 3: Final Validation

1. **Complete Pipeline**:

   ```bash
   go mod tidy
   go build ./...
   make fmt
   make lint
   make test
   ```

2. **Validation Verification**:
    - **MANDATORY**: Check exit codes after EACH command - use `echo $?` to verify
    - **MANDATORY**: Confirm ALL commands completed with exit code 0
    - **MANDATORY**: Verify no test failures remain
    - **MANDATORY**: Verify no lint errors remain
    - If any step fails → DO NOT complete the review, return to Phase 2

3. **Coverage Check**: Ensure coverage hasn't decreased

4. **Todo Verification**: ALL todos must be "completed"

5. **Commit Changes**: Use conventional commit format

### 🛡️ Success Requirements

- **Problem Comprehension**: Clearly understand each issue
- **Single-Change Discipline**: One issue → verify → next
- **Tool Competence**: Correct use of validation tools
- **Zero New Issues**: No additional problems introduced
- **Complete Resolution**: ALL issues fixed
- **Never Broken**: Codebase must NEVER be left in a broken state (failing tests/lint)

**Task**: Launch the finfocus-senior-engineer agent to systematically fix
all issues identified in Phase 1.
