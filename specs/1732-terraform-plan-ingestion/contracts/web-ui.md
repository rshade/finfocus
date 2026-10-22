# Contract: Web UI

The owner's follow-up requires Terraform-compatible project detection in
`finfocus --web`; it supersedes the original exemption.

## Layout fit

Use the existing overview view: heading, controls row, live loading status,
summary strip, and captioned resource table. Planned Terraform addresses
remain the resource selectors; resource types are resolved by plugins.
Cost, recommendations, and estimate retain their existing navigation and
query shapes. No new view is introduced.

## API hook

The existing handlers in `internal/webui` call the CLI's shared overview
and engine methods. See [http-api.md](http-api.md) for endpoint and
package-qualified method mappings. The web layer adds no parsing or
pricing logic and uses the same redacted descriptors as the CLI.

## Browser evidence and follow-up

Validate a fresh OpenTofu plan with the installed pricing plugin. Check
resource identities, status, totals, estimate queries, and browser console.
Capture the populated overview and record results in `../validation.md`.
File one issue labeled `web-ui` for layout, control copy, interaction, and
accessibility only. Any engine, API, redaction, or data-shape failure must
be fixed in this feature before completion.

UX-only follow-up: [#1888](https://github.com/rshade/finfocus/issues/1888).
