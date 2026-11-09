# Feature Specification: Stack-Aware Routing and a Prefix-Safe Web Frontend

**Feature Branch**: `2012-web-stack-routing-prefix-safe`
**Created**: 2026-11-08
**Status**: Draft
**Input**: GitHub issue rshade/finfocus#2037 — "web: stack-aware routing and
prefix-safe frontend (`/s/<stack>/`)". Step 2 of 4 (#2036, #2037, #2038, then
\#2039). This step adds the transport and the frontend capability only. The
`--stack` list flag, tab bar, and per-stack loading belong to #2039.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reach a stack's dashboard under `/s/<stack>/` (Priority: P1)

A developer embedding the web UI (the multi-stack dashboard of #2039) mounts
one session per stack. A browser that opens `/s/dev/` gets the full app, and
every API call, static file, and event stream it makes lands on the `dev`
stack's session, never on another's.

**Why this priority**: Without routing there is nothing for the multi-stack
UI to build on.

**Independent Test**: Register two stacks with distinct sessions, request the
same API path under each prefix, and confirm each answer comes from its own
session.

**Acceptance Scenarios**:

1. **Given** stacks `dev` and `prod` are registered, **When** an
   authenticated client requests `/s/dev/api/overview/budget`, **Then** the
   `dev` session answers, and `/s/prod/...` reaches the `prod` session.
2. **Given** the same server, **When** the client requests `/s/dev/`,
   **Then** it receives the SPA shell carrying the prefix `/s/dev`.
3. **Given** the same server, **When** the client requests an unregistered
   stack, `/s/nope/api/...`, **Then** it receives 404 with the contract error
   shape, after authentication.
4. **Given** an unauthenticated client, **When** it requests any `/s/...`
   path, registered or not, **Then** it receives 401, so stack names cannot be
   probed.
5. **Given** `/s/dev` without the trailing slash, **When** requested,
   **Then** the server redirects to `/s/dev/` so relative asset URLs resolve.

---

### User Story 2 - The frontend works under any prefix (Priority: P1)

The single-page app loads its assets, calls its API, opens its event streams,
and loads the log viewer module relative to the prefix it was served under,
so the same files serve `/` and `/s/<stack>/`.

**Why this priority**: The app uses absolute `/api` and `/static` URLs, which
break under a prefix. Routing alone is useless until this is fixed.

**Independent Test**: Run the frontend harness with the prefix meta set and
assert that fetch, both `EventSource` sites, the session check, and the viewer
import all carry the prefix; with no prefix, every URL is unchanged.

**Acceptance Scenarios**:

1. **Given** the page declares prefix `/s/dev`, **When** the app calls
   `api.get('/api/config')`, **Then** the request goes to
   `/s/dev/api/config`.
2. **Given** the same page, **When** the overview stream, the plugin
   operation stream, or the logs stream opens, **Then** each opens under the
   prefix, as does the session-expiry check and the log viewer import.
3. **Given** the page declares no prefix, **When** any request is made,
   **Then** the URL is byte-identical to today.
4. **Given** the shell is served under a prefix, **When** the browser loads
   CSS, favicon, and scripts, **Then** they resolve under the prefix.

---

### User Story 3 - Existing single-stack use is unchanged (Priority: P1)

A user running `finfocus --web` today sees no difference: same URLs, same
cookie, same behavior, with no stacks registered.

**Independent Test**: The existing web test suite passes unmodified, and
`GET /` serves a shell whose prefix is empty.

**Acceptance Scenarios**:

1. **Given** no stack is registered, **When** any existing request is made,
   **Then** the response is identical to the previous release, apart from the
   shell's added empty prefix meta tag and relative asset URLs.
2. **Given** a registered stack, **When** a root path such as `/api/config`
   is requested, **Then** the root session still answers it.

### Edge Cases

- Stack names: one path segment, `[A-Za-z0-9_.-]`, not `.` or `..`, at most
  100 characters (Pulumi's simple stack name). Anything else is refused at
  registration, so a name can never alter the routing path or inject into the
  shell.
- A duplicate registration, or one after shutdown, is an error.
- A request under `/s/` with no stack segment, or `/s/%2e%2e/`, is a 404
  after authentication; the path is never normalized into another stack.
- The token bootstrap (`GET /?token=`) also works at `/s/<stack>/?token=` and
  redirects back to that stack's URL. One cookie, scoped to `/`, signs the
  browser into every stack.
- Logs routes are excluded from the request log under a prefix exactly as at
  root, so the Logs view cannot feed itself.
- Shutdown closes every stack's session and plugin operations.
- Host, Origin, and fetch-metadata hardening apply to prefixed requests
  unchanged, because they run in the outer layer.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The server MUST support registering named stacks, each with its
  own session, and MUST dispatch `/s/<stack>/<path>` to that stack's
  handler tree with the prefix stripped.
- **FR-002**: Authentication, Host check, Origin and content-type hardening,
  security headers, and request logging MUST run once, in the outer layer,
  before dispatch. A stack handler tree MUST NOT repeat them.
- **FR-003**: An unregistered stack MUST return 404 `{error, code}` and only
  to an authenticated client; an unauthenticated client MUST get 401.
- **FR-004**: A stack name MUST be validated at registration per the edge
  cases; invalid, duplicate, or late registrations MUST return an error.
- **FR-005**: `/s/<stack>` MUST redirect to `/s/<stack>/`, preserving the
  query, after authentication.
- **FR-006**: The SPA shell MUST carry the serving prefix in a
  `<meta name="finfocus-base">` element (CSP forbids inline script), empty at
  root. The shell MUST reference its CSS, icon, and script by relative URL.
- **FR-007**: The frontend MUST derive every network URL through one
  `withBase(path)` helper over `base()`, covering fetch, both `EventSource`
  sites, the session-expiry check, the log viewer import, and the HTML
  references. No absolute `/api` or `/static` URL may remain outside it.
- **FR-008**: The session cookie MUST stay one global cookie named from the
  listen port with `Path=/`; this MUST be documented.
- **FR-009**: The token bootstrap MUST be accepted at the root and at a
  registered stack's root, and MUST answer 303 to the URL it was received at.
- **FR-010**: With no stack registered, behavior MUST be unchanged (spec 627).
- **FR-011**: Each stack MUST get its own plugin operations; the log source
  MUST be shared and default to the server's.
- **FR-012**: `Shutdown` MUST release every registered stack's resources.

### Key Entities

- **Stack route**: a validated name bound to a handler tree and a session.
- **Prefix**: `""` at root, `/s/<stack>` under a stack; delivered to the
  frontend through the shell's meta tag.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable — this adds routing and prefix-safety to
  the existing dashboard. No view, control, or data is added, and no CLI
  feature gains a web hook here. The tab bar and stack label are #2039's.
- **Web data**: None new.
- **Layout fit**: Unchanged. The shell differs only by a meta tag and
  relative asset URLs.
- **Additions**: none
- **Render evidence**: A Playwright check serves the app under `/s/<stack>/`
  and the existing Overview renders and streams under the prefix; recorded in
  the PR.
- **Follow-up issue scope**: None. #2039 carries the UX.

## Success Criteria *(mandatory)*

- **SC-001**: 100% of requests under `/s/<stack>/` reach exactly that stack's
  session (automated test with two sessions).
- **SC-002**: No frontend source line outside `base()`/`withBase()` builds an
  absolute `/api` or `/static` URL (enforced by a test).
- **SC-003**: The existing web Go and frontend tests pass with only the
  assertions that pin the shell changed.
- **SC-004**: An unauthenticated client cannot distinguish a registered stack
  from an unregistered one.

## Assumptions

- A caller (#2039) registers stacks before `Serve`; registration while serving
  is safe but not required.
- Plugin sets and shared stores stay per #2038; this feature only routes.
- Stack names are Pulumi simple names. Fully qualified `org/project/stack`
  names are mapped to a simple name by the caller.
