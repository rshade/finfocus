---
title: "Pick a Roadmap Issue"
description: Choose and claim one FinFocus roadmap issue, implement and verify it, prepare a pull request, and release the claim.
layout: "docs"
---

Choose exactly one issue in `rshade/finfocus`, take it through implementation
and verification, and prepare a pull request. Stop after that issue. Adapted
from ax-go's `pick-issue` workflow for FinFocus's commands and repository rules.

An invocation to work an issue includes the claim and release comments below.
A request only to recommend an issue, or to edit this workflow, does not.
Respect the user's requested scope and existing authorization throughout.

## Phase 0 — Preflight

Read [AGENTS.md](../../AGENTS.md), [CLAUDE.md](../../CLAUDE.md),
[CONTEXT.md](../../CONTEXT.md), and the
[constitution](../../.specify/memory/constitution.md). Check for additional
instructions in the directories you will touch.

```bash
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
REPO=rshade/finfocus
gh auth status
git remote -v
git status --short
git worktree list
git fetch origin
```

Confirm `origin` points to FinFocus before using `origin/main`. Work in an
isolated worktree; preserve existing tracked and untracked changes in the
original checkout. Never stash someone else's work or stage files with
`git add .` or `git add -A`.

## Phase 1 — Choose and claim

### Inspect current work and candidates

```bash
gh label list --repo "$REPO" --limit 200 --json name,description
gh issue list --repo "$REPO" --state open --label roadmap/current --limit 100 \
  --json number,title,body,labels,assignees
gh pr list --repo "$REPO" --state open --limit 100 \
  --json number,title,headRefName,body
```

If a result reaches its limit, retrieve the remaining pages before concluding
that work is unclaimed. Check issues carrying `processing:roadmap` if that
label exists, and inspect their claim comments as described below.

FinFocus uses `roadmap/current`, `roadmap/next`, `roadmap/future`, and
`effort/small`, `effort/medium`, `effort/large`. Discover other labels at runtime.
There is no required `area:*` lane taxonomy: inspect issue bodies and relevant
files to identify overlap with active claims and open PRs. Pay particular
attention to shared engine types, CLI wiring, plugin protocols, `go.mod`,
`go.sum`, and shared fixtures. An absent lane label does not prove independence.

Exclude claimed issues, work already covered by an open PR, blocked issues,
and umbrella issues whose children should be implemented separately. Use
[ROADMAP.md](../../ROADMAP.md) for context, but confirm issue state on GitHub.
Cross-repository items may depend on finfocus-spec or a plugin release; do not
implement provider pricing logic or new protocol definitions in core to bypass
those dependencies.

If the user provided an issue number, inspect that issue directly, including
its state, labels, comments, dependencies, and linked PRs. An explicit number
can select an issue outside `roadmap/current`, but cannot override another
worker's claim. Report any overlap before implementation.

Otherwise present a table of eligible issues: number, type, affected packages,
priority (if supplied), effort, and title. Order by explicit priority, then
`spec-first`, `bug`, and maintenance work. Mark missing metadata as unknown.
Ask the user to choose unless they already asked you to choose autonomously.

If no eligible `roadmap/current` issues remain, report whether the queue is
empty, blocked, or claimed. Ask before widening to `roadmap/next` unless the
user already authorized it. Do not change roadmap labels to promote work.

### Claim and verify ownership

`processing:roadmap` is an advisory coordination label. GitHub label mutations
are not atomic locks. If the label is absent from the repository, create it
when actually claiming an issue:

```bash
gh label create processing:roadmap --repo "$REPO" --color D93F0B \
  --description "Issue claimed by a pick-issue worker"
```

If creation reports that the label already exists, re-read it and proceed;
other failures must be resolved before claiming.

Use a unique token for this invocation and keep it available across shell
calls. Set `N` to the chosen issue number. Before posting, read all issue
comments and labels. An unreleased claim is active regardless of age; a label
without a claim is ambiguous. Report either case rather than stealing it.

```bash
CLAIM="claim: $(hostname)/$$-$(date -u +%s)-$(openssl rand -hex 8)"
gh issue comment "$N" --repo "$REPO" --body "$CLAIM"
gh issue edit "$N" --repo "$REPO" --add-label processing:roadmap
gh api --paginate "repos/$REPO/issues/$N/comments" \
  --jq '.[] | {id, created_at, body}'
```

A claim comment is a body starting with `claim:`. A release comment has the
exact body `release: <full claim body>`. Among claims without matching releases,
the earliest comment wins; use the numeric comment ID to break timestamp ties.
Re-read comments and labels before implementation. Also recheck overlapping
claims on other issues; this protocol does not atomically reserve packages.

If another worker wins, post a release for your own token and return to the
chooser. **Never remove the winner's label.** If any claim operation fails,
inspect the resulting state before retrying; do not assume ownership or post
another claim token. Do not expire an old claim automatically. A stale claim
requires an explicit handoff or user resolution.

### Reconcile the issue with the repository

Read the full issue and comments, locate the cited code with `rg`, and check
relevant commits and tests before routing it. Verify referenced dependency
issues and PRs. Old file paths and line numbers are evidence to investigate,
not implementation instructions.

If the work is already complete, report the implementing commits and tests.
Close the issue only when authorized, then release your claim and stop.

## Phase 2 — Create a worktree and choose the route

```bash
WORKTREE="$(dirname "$ROOT")/finfocus-$N"
git worktree add "$WORKTREE" -b "issue-$N" origin/main
cd "$WORKTREE"
```

If the branch or path exists, inspect it and resume only when it belongs to
this issue; otherwise choose a unique path and branch. Never overwrite it.
Record the starting commit for review before Spec Kit changes the branch name.

| Issue scope | Route |
| --- | --- |
| `spec-first`, or a feature requiring design and acceptance criteria | Spec Kit, then implementation |
| Focused `bug`, `maintenance`, `refactor`, `testing`, `ci`, or documentation work | Direct implementation with appropriate tests |
| Umbrella issue or unresolved cross-repository prerequisite | Report the blocker; offer a bounded child issue |

Read the body rather than routing solely by label. If a purported small fix
requires a new interface or broader behavior, reassess its scope and whether
an existing specification covers it. FinFocus does not inherit ax-go's rule
that every runtime change requires a new spec.

### Spec Kit route

Look for an existing feature under `specs/` and resume its branch and artifacts
where appropriate. For a new feature, run specification creation **inside the
worktree**. The command creates a numbered feature branch; use the branch and
paths it returns for all later work, including the PR.

1. [speckit.specify](speckit.specify.md): create `spec.md`.
2. [speckit.clarify](speckit.clarify.md): resolve material ambiguity if needed.
3. [speckit.plan](speckit.plan.md): create `plan.md` and supporting design.
4. [speckit.tasks](speckit.tasks.md): create `tasks.md`.
5. [speckit.analyze](speckit.analyze.md): reconcile requirements, design, and
   tasks; address valid findings before implementation and rerun after changes.
6. [speckit.implement](speckit.implement.md): implement and update task status.

Follow FinFocus's feature-creation script: let it discover the next number;
do not hardcode `--number`. Check active feature branches and claims for
numbering collisions. Coordinate concurrent specification creation instead
of committing a specification on `main` to reserve a number.

Keep core provider-agnostic, preserve operation without prior local state,
and coordinate protocol changes through finfocus-spec. Update docs and spec
status alongside the implementation.

## Phase 3 — Verify and review

Run the repository's gates from the implementation worktree:

```bash
make validate
make test
make lint
```

For Go changes, run race detection (`make test-race`) and tests covering the
changed behavior. `make test` covers `internal/` and `pkg/`; it does not run
the integration suite or every module. Add relevant integration, conformance,
or recorder-plugin checks when those boundaries change. Follow the setup and
authorization requirements for E2E tests involving real infrastructure.

For Markdown changes, run `make docs-lint` and lint changed Markdown outside
`docs/` explicitly, since the docs target excludes it. Use testify assertions
in Go tests and render and inspect affected TUI views when presentation changes.

Review the implementation against the recorded base commit and issue
acceptance criteria. Use the available `code-review` skill (or the local
[code-review command](code-review.md)); fix substantiated findings. If `scout`
is available, report its improvement opportunities without expanding this
issue's scope. Spec analysis does not replace review of the implemented diff.

Report actual failures and unavailable tools. Do not import ax-go's build-tag
matrix, surface/size gates, or assumptions about environmental lint failures.

## Phase 4 — Prepare the commit and pull request

[CLAUDE.md](../../CLAUDE.md) reserves commits for the user. Unless the user has
explicitly superseded that rule, complete the implementation, verification,
and review, then provide the exact file list, a Conventional Commit message,
and a PR title and body for the handoff. Explain this repository rule when it
prevents committing. Do not stop before the change is ready to review.

If the user authorizes committing, stage only the named files and commit with
the prepared message. Include `Closes #N` and the spec directory when relevant;
use a breaking-change marker when warranted. Preserve the current session's
authorization for pushing and opening a PR rather than asking again.

The PR body should describe the problem, resulting behavior, linked issue,
spec if any, and validation results. Store the body in a temporary file outside
the worktree and pass it with `--body-file`.

```bash
BRANCH="$(git branch --show-current)"
git push -u origin "$BRANCH"
gh pr create --repo "$REPO" --base main --head "$BRANCH" \
  --title "<conventional commit subject>" --body-file "$PR_BODY_FILE"
```

Set `PR_BODY_FILE` to the prepared file and replace the title before execution.
Check for an existing PR before creating one. Leave the issue open for the
closing reference to resolve on merge. Stop at the PR; merging requires the
user's instruction.

## Phase 5 — Release or retain the claim

After opening the PR, or explicitly abandoning the attempt, record the PR or
remaining work in an issue comment. Re-read ownership before cleanup. Remove
the label only if you still own the winning claim and no other unreleased
claim needs it, then post the exact release marker:

```bash
gh issue edit "$N" --repo "$REPO" --remove-label processing:roadmap
gh issue comment "$N" --repo "$REPO" --body "release: $CLAIM"
```

If another unreleased claim exists, leave the label and release only your own
token. A losing worker must never remove the label. Verify the final comments
and label state; report cleanup failures instead of claiming release succeeded.

While waiting for a user decision or manual commit, retain the claim and say
why. Keep any worktree with uncommitted or unpushed work. Remove a completed
worktree only after confirming its work is preserved remotely and it is clean;
run removal from the original checkout and never force it.

## Phase 6 — Report and stop

Report the issue and selection reason, route and spec directory if any,
validation and review results, branch and worktree, PR URL or prepared handoff,
and whether the claim is released or retained. Mention selection outside
`roadmap/current` explicitly. Do not pick another issue.
