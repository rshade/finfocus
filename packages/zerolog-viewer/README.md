# zerolog-viewer

A framework-free browser viewer for [zerolog](https://github.com/rs/zerolog)
JSON logs. It colours levels, separates log sources (for example the host
process and its plugins), and filters by source, minimum level, and text.

The package has no dependencies and no build step. It is plain ES modules plus
one stylesheet.

> This package is not published yet (`private` is `true` in `package.json`).
> FinFocus ships it embedded in `finfocus --web`, where the Logs page is off by
> default.

## Usage without installing

Serve the `src` directory from any static host and import it directly:

```html
<link rel="stylesheet" href="/zerolog-viewer/viewer.css">
<div id="logs"></div>
<script type="module">
  import { createLogViewer } from '/zerolog-viewer/index.js';

  const viewer = createLogViewer(document.getElementById('logs'));
  viewer.pushChunk('{"level":"info","message":"hello"}\n');
</script>
```

Or use the optional custom element, which wraps the same viewer:

```html
<script type="module" src="/zerolog-viewer/element.js"></script>
<zerolog-viewer min-level="warn" sources="aws-public"></zerolog-viewer>
```

The element observes the `sources` (comma separated), `min-level`, `text`, and
`regex` (boolean) attributes and re-filters when they change. It exposes
`push`, `pushLine`, `pushChunk`, and `clear`, and tears the viewer down when it
is removed from the page.

## Pushing logs

`createLogViewer(container, options)` returns an object with these methods:

| Method | Purpose |
| --- | --- |
| `pushChunk(text)` | Feed raw text from a stream. Partial lines are buffered until their newline arrives. |
| `pushLine(text)` | Feed one complete line. |
| `push(entry)` | Add an already normalized entry. |
| `setFilters(filters)` | Replace the active filters. |
| `clear()` | Remove every row. |
| `destroy()` | Remove the viewer and its listeners. |

A line that is not valid JSON is shown as a raw `info` line from source
`unknown`, so stray stderr text is never dropped.

```javascript
import { createLogViewer } from '@rshade/zerolog-viewer';

const viewer = createLogViewer(container, { document, maxRows: 500 });

viewer.pushChunk('{"level":"info","message":"started"}\n{"level":"er');
viewer.pushChunk('ror","message":"failed","plugin_name":"aws-public"}\n');
viewer.pushLine('plain text from stderr');
viewer.push({
  time: '2026-11-02T10:00:00Z',
  level: 'warn',
  source: 'core',
  sourceKind: 'core',
  message: 'already normalized',
});

viewer.setFilters({ minLevel: 'error' });
viewer.clear();
viewer.destroy();
```

Options: `document` (defaults to the container's owner document), `maxRows`
(default 2000; older rows are dropped and counted), `filters`, `follow`
(default `true`; keeps the newest row in view until the user scrolls up),
`fieldMap`, `sourceFields`, and `pluginFields`.

## Entry schema

Every row is an entry with these fields:

| Field | Type | Meaning |
| --- | --- | --- |
| `time` | ISO 8601 string | Taken from the `time` field. Numbers are read as epoch seconds or milliseconds. A missing or invalid value becomes the arrival time. |
| `level` | string | One of `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`. `warning`, `err`, and `critical` are aliases. Anything else becomes `info`. |
| `source` | string | The source name, or `core` when none is found. |
| `sourceKind` | `core` or `plugin` | `plugin` when the source came from a plugin field. |
| `message` | string | The `message` field as text. |
| `fields` | object | Every other key of the original object. |

## Field names and sources

`fieldMap` renames the three core fields when your logger does not use zerolog's
defaults, and `sourceFields` lists the keys that name a source, in priority
order. The first non-empty string wins. Keys named in `pluginFields` mark the
entry as a plugin entry.

```javascript
import { createLogViewer } from '@rshade/zerolog-viewer';

const viewer = createLogViewer(container, {
  document,
  fieldMap: { time: 'ts', level: 'severity', message: 'msg' },
  sourceFields: ['service', 'component'],
  pluginFields: ['service'],
});

viewer.pushLine('{"ts":1762077600,"severity":"warning","msg":"slow","service":"billing"}');
```

The defaults are `time`, `level`, and `message`; sources come from
`plugin_name` then `component`; only `plugin_name` marks a plugin.

## Filters

The filter bar accepts free text, with an optional regular-expression switch.
Programmatic filters take the same shape:

```javascript
import { createLogViewer } from '@rshade/zerolog-viewer';

const viewer = createLogViewer(container, {
  document,
  filters: { sources: ['aws-public'], minLevel: 'warn', text: 'timeout', regex: false },
});

viewer.setFilters({ text: 'time(out|d)', regex: true });
```

Text matches the message and every field value, case-insensitively. An invalid
regular expression is reported in the toolbar and filters nothing.

## Theming

The stylesheet reads CSS custom properties, with light and dark defaults
chosen by `prefers-color-scheme`. Override any of them on the viewer or one of
its ancestors:

| Variable | Used for |
| --- | --- |
| `--zlv-bg` | Background |
| `--zlv-fg` | Text |
| `--zlv-muted` | Secondary text |
| `--zlv-border` | Borders and separators |
| `--zlv-badge-bg` | Source badge background |
| `--zlv-plugin-bg` | Plugin source badge background |
| `--zlv-trace` | Trace level colour |
| `--zlv-debug` | Debug level colour |
| `--zlv-info` | Info level colour |
| `--zlv-warn` | Warn level colour |
| `--zlv-error` | Error level colour, invalid-filter outline |
| `--zlv-fatal` | Fatal level colour |
| `--zlv-panic` | Panic level colour |
| `--zlv-font` | Font stack |

```css
zerolog-viewer,
#logs {
  --zlv-bg: #101418;
  --zlv-fg: #e6edf3;
  --zlv-warn: #f0b429;
}
```

## Security

The viewer writes every log value with `textContent`. It never assigns
`innerHTML`, never builds markup from log data, and never uses `eval` or
`new Function`. A line such as `<img src=x onerror=alert(1)>` is displayed as
text. Regular expressions come from the user's own filter box and run in the
browser against data already on the page.

## Publish checklist

1. Pick the npm scope and update `name` in `package.json` if it is not
   `@rshade`.
2. Set `private` to `false`.
3. Add the package to `release-please-config.json` with an `initial-version`,
   and add its path to the root package's `exclude-paths`.
4. Run the tests: `node --test packages/zerolog-viewer/test`.
5. Publish with provenance from CI: `npm publish --provenance --access public`.
