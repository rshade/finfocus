# Contract: web API change

One existing route gains one field. No new route, no new setting.

## `GET /api/overview/resource?urn=<urn>`

Existing response (`webui.ResourceDetail`) gains `blastRadius`, filled by
`engine.ComputeBlastRadius` from the session's rows and the graph published
by `Snapshot.Graph` (the example omits the unchanged fields):

```json
{
  "row": { "urn": "urn:pulumi:dev::demo::aws:rds/instance:Instance::db" },
  "blastRadius": {
    "urn": "urn:pulumi:dev::demo::aws:rds/instance:Instance::db",
    "available": true,
    "count": 2,
    "monthlyTotal": 128.5,
    "currency": "USD",
    "mixedCurrency": false,
    "leftOut": { "deleting": 0, "unpriced": 0, "unresolved": 0 },
    "summaryDisplay": "2 dependents, $128.50/month",
    "noteDisplay": "",
    "dependents": [
      {
        "urn": "urn:pulumi:dev::demo::aws:ecs/service:Service::api",
        "type": "aws:ecs/service:Service",
        "displayName": "api",
        "status": "active",
        "depth": 1,
        "via": "urn:pulumi:dev::demo::aws:rds/instance:Instance::db",
        "monthlyCost": 72.0,
        "priced": true,
        "component": false,
        "monthlyDisplay": "$72.00",
        "statusLabel": ""
      }
    ]
  }
}
```

- `blastRadius` is always present on a 200. When the source has no dependency
  data, `available` is `false` and every other field is its zero value
  (`dependents` is `null`, `count` is `0`, `leftOut` is all zeros); clients
  must check `available` first.
- `status` is the lowercase `ResourceStatus` string (`active`, `creating`,
  `updating`, `deleting`, `replacing`). `monthlyCost` is `null` when the
  dependent is unpriced, deleting or a component.
- `monthlyTotal` is omitted when withheld (`mixedCurrency` true).
- `summaryDisplay`, `noteDisplay`, `monthlyDisplay` and `statusLabel` are the
  server-formatted copy from the table below. The browser renders them as
  given and formats no number (FR-011b); the numeric fields stay for
  clients that need them.
- `dependents` is ordered so that every `via` appears earlier in the list or
  is the selected URN.
- Errors are unchanged: unknown URN is 404 `not_found`.
- The payload passes through the existing JSON redaction path. It contains no
  properties, tags or inputs, and no data-keyed maps.

## Browser rendering (UX only)

`internal/webui/static/views/overview.js` renders `blastRadius` in the detail
panel as a captioned `.detail-table`: indented by `depth`, status and cost
columns aligned, total and left-out counts in the caption, and the two empty
messages from FR-006. Layout, rendering, interaction, copy and accessibility
are the UX refinement in the spec's Web UI Fit section (constitution
Principle VII). A component dependent renders as plain text, not a button,
because it has no overview row to open.

## Copy

The terminal and the web use these strings, produced once by
`ComputeBlastRadius` (the empty and unavailable messages are fixed per
surface). The web shows a sentence with its first letter capitalized; the terminal indents it by two spaces and shows it
as written. `TestBlastRadiusCopy` in `internal/engine/blast_radius_display_test.go` pins
this table; the TUI goldens and the copy test in
`internal/webui/frontendtest/overview.test.mjs` check that each surface
renders it unchanged.

| Element | String |
| --- | --- |
| Empty | `nothing depends on this resource` |
| Unavailable | `dependency information is not available` |
| Summary, one | `1 dependent, $10.00/month` |
| Summary, many | `2 dependents, $10.00/month` |
| Summary, total withheld | `2 dependents` |
| Left out, one | `1 deleted dependent is not in the total` |
| Left out, both | `1 deleted and 2 unpriced dependents are not in the total` |
| Unresolved | `3 unresolved references` |
| Mixed currency | `total withheld: dependents use different currencies` |
| Status labels | `new`, `deleting`, `replacing`, `updating`, `component` |
| No price | `no price` |
| Back button (web) | `Back to NAME`, where NAME is the earlier resource |

## Interaction

The dependents in the detail-panel block are buttons. Activating one opens that
resource's detail by calling the existing `GET /api/overview/resource` with the
dependent's `urn`, exactly as selecting a table row does. The earlier resource
is kept in browser memory only, so the detail header can offer `Back to NAME`.
This adds no endpoint, parameter, or payload field, and an unknown URN keeps
the existing 404 `not_found` response.
