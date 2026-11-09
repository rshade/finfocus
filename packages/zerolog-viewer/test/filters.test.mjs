import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { resolveObjectURL } from 'node:buffer';
import { fileURLToPath } from 'node:url';
import { createLogViewer, matchesFilter, compileFilter } from '../src/index.js';
import { createDocument } from './dom.mjs';

const casesPath = fileURLToPath(new URL('./fixtures/filter_cases.json', import.meta.url));

test('the viewer filter agrees with the Go filter on the shared cases', async () => {
  const cases = JSON.parse(await readFile(casesPath, 'utf8'));
  assert.ok(cases.length > 0);
  for (const c of cases) {
    const filter = compileFilter(c.filter);
    const got = c.entries.filter((e) => matchesFilter(e, filter)).map((e) => e.seq);
    assert.deepEqual(got, c.expectSeq, c.name);
  }
});

const entry = (over = {}) => ({
  time: '2026-11-02T10:00:00Z', level: 'info', source: 'engine', sourceKind: 'core', message: 'hello', ...over,
});
const plugin = (over = {}) => entry({ source: 'aws-public', sourceKind: 'plugin', ...over });

function setup(options = {}) {
  const document = createDocument();
  const anchors = [];
  const create = document.createElement;
  document.createElement = (tag) => {
    const node = create(tag);
    if (tag === 'a') {
      node.clickCount = 0;
      node.click = () => {
      node.clickCount += 1;
      node.attachedAtClick = node.parentNode != null;
    };
      anchors.push(node);
    }
    return node;
  };
  const container = document.createElement('div');
  const viewer = createLogViewer(container, { document, ...options });
  const one = (name) => container.byClass(name)[0];
  return {
    container,
    viewer,
    anchors,
    one,
    rows: () => container.byClass('zlv-row'),
    chips: () => container.byClass('zlv-chip'),
    status: () => one('zlv-status').textContent,
    list: () => one('zlv-list'),
  };
}

test('source chips come from the sources option before any line arrives', () => {
  const { chips } = setup({ sources: ['core', 'aws-public'] });
  assert.deepEqual(chips().map((c) => c.textContent), ['core', 'aws-public']);
});

test('setSources adds chips and new sources seen in entries append without duplicates', () => {
  const { viewer, chips } = setup();
  viewer.setSources(['kubernetes']);
  viewer.push(plugin());
  viewer.push(plugin());
  viewer.push(entry());
  assert.deepEqual(chips().map((c) => c.textContent), ['kubernetes', 'aws-public', 'engine']);
});

test('selecting chips filters to those sources and deselecting restores all', () => {
  const { viewer, chips, rows } = setup({ sources: ['engine', 'aws-public'] });
  viewer.push(entry());
  viewer.push(plugin());
  chips()[1].dispatch('click');
  assert.equal(rows().length, 1);
  assert.equal(chips()[1].attrs['aria-pressed'], 'true');
  assert.equal(chips()[0].attrs['aria-pressed'], 'false');
  chips()[0].dispatch('click');
  assert.equal(rows().length, 2);
  chips()[0].dispatch('click');
  chips()[1].dispatch('click');
  assert.equal(rows().length, 2);
});

test('the level select hides entries below the minimum', () => {
  const { viewer, one, rows } = setup();
  viewer.push(entry({ level: 'debug' }));
  viewer.push(entry({ level: 'warn' }));
  const select = one('zlv-level-select');
  select.value = 'warn';
  select.dispatch('change');
  assert.equal(rows().length, 1);
  select.value = '';
  select.dispatch('change');
  assert.equal(rows().length, 2);
});

test('the kind select shows only plugin or only core output', () => {
  const { viewer, one, rows } = setup();
  viewer.push(entry());
  viewer.push(plugin());
  viewer.push(plugin());
  const select = one('zlv-kind-select');
  select.value = 'plugin';
  select.dispatch('change');
  assert.equal(rows().length, 2);
  select.value = 'core';
  select.dispatch('change');
  assert.equal(rows().length, 1);
  select.value = '';
  select.dispatch('change');
  assert.equal(rows().length, 3);
});

test('setFilters keeps every control in step', () => {
  const { viewer, one, chips, rows } = setup({ sources: ['engine', 'aws-public'] });
  viewer.push(entry({ level: 'error' }));
  viewer.push(plugin({ level: 'error' }));
  viewer.push(plugin());
  viewer.setFilters({ sources: ['aws-public'], kind: 'plugin', minLevel: 'error', text: 'hello', regex: false });
  assert.equal(rows().length, 1);
  assert.equal(one('zlv-level-select').value, 'error');
  assert.equal(one('zlv-kind-select').value, 'plugin');
  assert.equal(chips()[1].attrs['aria-pressed'], 'true');
  assert.equal(one('zlv-search').value, 'hello');
});

test('the status line reports shown of total', () => {
  const { viewer, status, one } = setup();
  viewer.push(entry({ message: 'alpha' }));
  viewer.push(entry({ message: 'beta' }));
  assert.match(status(), /Showing 2 of 2 lines/);
  one('zlv-search').value = 'alpha';
  one('zlv-search').dispatch('input');
  assert.match(status(), /Showing 1 of 2 lines/);
});

test('pause keeps the scroll position and counts new lines; resume jumps to the newest', () => {
  const { viewer, one, list, status } = setup();
  viewer.push(entry());
  const follow = one('zlv-follow');
  assert.equal(list().scrollTop, list().scrollHeight);
  follow.dispatch('click');
  list().scrollTop = 7;
  viewer.push(entry());
  viewer.push(entry());
  assert.equal(list().scrollTop, 7);
  assert.match(status(), /2 new lines while paused/);
  assert.equal(follow.attrs['aria-pressed'], 'false');
  follow.dispatch('click');
  assert.equal(list().scrollTop, list().scrollHeight);
  assert.doesNotMatch(status(), /while paused/);
  assert.equal(follow.attrs['aria-pressed'], 'true');
});

test('lines hidden by the filter do not count as new while paused', () => {
  const { viewer, one, status } = setup();
  viewer.setFilters({ text: 'keep' });
  one('zlv-follow').dispatch('click');
  viewer.push(entry({ message: 'drop' }));
  viewer.push(entry({ message: 'keep' }));
  assert.match(status(), /1 new line while paused/);
});

test('clear empties the list and keeps only later lines', () => {
  const { viewer, one, rows, status } = setup();
  viewer.push(entry({ message: 'old' }));
  one('zlv-clear').dispatch('click');
  assert.equal(rows().length, 0);
  assert.match(status(), /Showing 0 of 0 lines/);
  viewer.push(entry({ message: 'new' }));
  assert.equal(rows().length, 1);
  assert.equal(rows()[0].byClass('zlv-message')[0].textContent, 'new');
});

test('download builds NDJSON of the shown entries through a Blob link', () => {
  const { viewer, one, anchors } = setup();
  viewer.push(entry({ message: 'plain' }));
  viewer.push(plugin({ message: 'from plugin', fields: { region: 'us-west-2' } }));
  viewer.setFilters({ kind: 'plugin' });
  one('zlv-download').dispatch('click');
  assert.equal(anchors.length, 1);
  const anchor = anchors[0];
  assert.equal(anchor.clickCount, 1);
  assert.equal(anchor.attachedAtClick, true);
  assert.equal(anchor.className, 'zlv-save-link');
  assert.equal(anchor.attrs.tabindex, '-1');
  assert.equal(anchor.attrs['aria-hidden'], 'true');
  assert.match(anchor.attrs.download, /\.ndjson$/);
  const blob = resolveObjectURL(anchor.href);
  assert.ok(blob, 'href is a blob URL');
  return blob.text().then((text) => {
    const lines = text.trimEnd().split('\n').map((l) => JSON.parse(l));
    assert.equal(lines.length, 1);
    assert.equal(lines[0].message, 'from plugin');
    assert.equal(lines[0].source, 'aws-public');
    assert.equal(lines[0].region, undefined);
    assert.equal(lines[0].fields.region, 'us-west-2');
    assert.ok(text.endsWith('\n'));
  });
});

test('exportNDJSON returns the shown entries and an empty string when none show', () => {
  const { viewer } = setup();
  assert.equal(viewer.exportNDJSON(), '');
  viewer.push(entry({ message: 'a' }));
  viewer.push(entry({ message: 'b' }));
  viewer.setFilters({ text: 'b' });
  assert.equal(viewer.exportNDJSON().trimEnd().split('\n').length, 1);
});

test('download tolerates a document without anchor click support', () => {
  const document = createDocument();
  const container = document.createElement('div');
  const viewer = createLogViewer(container, { document });
  viewer.push(entry());
  container.byClass('zlv-download')[0].dispatch('click');
  assert.match(container.byClass('zlv-status')[0].textContent, /Showing 1 of 1 line/);
});
