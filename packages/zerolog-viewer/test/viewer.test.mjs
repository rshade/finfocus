import test from 'node:test';
import assert from 'node:assert/strict';
import { createLogViewer } from '../src/index.js';
import { mount } from './dom.mjs';

const entry = (over = {}) => ({
  time: '2026-11-02T10:00:00Z', level: 'info', source: 'core', sourceKind: 'core', message: 'hello', ...over,
});

function setup(options = {}) {
  const { document, container } = mount();
  const viewer = createLogViewer(container, { document, ...options });
  const rows = () => container.byClass('zlv-row');
  return { container, viewer, rows, status: () => container.byClass('zlv-status')[0].textContent };
}

test('a row shows time, level badge text, source and message', () => {
  const { viewer, rows } = setup();
  viewer.push(entry({
    level: 'warn', source: 'aws-public', sourceKind: 'plugin', message: 'slow',
    display: { time: '10:00:00.000', level: 'WARN ', source: 'aws-public' },
  }));
  assert.equal(rows().length, 1);
  const row = rows()[0];
  assert.equal(row.byClass('zlv-time')[0].textContent, '10:00:00.000');
  assert.equal(row.byClass('zlv-level')[0].textContent, 'WARN');
  assert.equal(row.byClass('zlv-source')[0].textContent, 'aws-public');
  assert.equal(row.byClass('zlv-source')[0].attrs['data-kind'], 'plugin');
  assert.equal(row.byClass('zlv-message')[0].textContent, 'slow');
  assert.equal(row.attrs['data-level'], 'warn');
});

test('without display the viewer derives the badge text', () => {
  const { viewer, rows } = setup();
  viewer.push(entry({ level: 'error' }));
  assert.equal(rows()[0].byClass('zlv-level')[0].textContent, 'ERROR');
});

test('fields expand on click and toggle', () => {
  const { container, viewer, rows } = setup();
  viewer.push(entry({
    fields: { region: 'us-east-1', n: 3, nested: { a: 1 } },
    display: { time: '10:00:00.000', level: 'INFO', source: 'core' },
  }));
  const row = rows()[0];
  const line = row.byClass('zlv-line')[0];
  assert.equal(line.tagName, 'button');
  assert.equal(line.attrs.type, 'button');
  assert.equal(line.attrs['aria-label'], '10:00:00.000 INFO core hello');
  assert.equal(line.attrs['aria-expanded'], 'false');
  assert.equal(row.byClass('zlv-fields').length, 0);
  line.dispatch('click');
  const detail = row.byClass('zlv-fields')[0];
  assert.equal(line.attrs['aria-expanded'], 'true');
  assert.deepEqual(detail.scrollIntoViewArgs, { block: 'nearest' });
  assert.equal(container.byClass('zlv-follow')[0].textContent, 'Resume');
  assert.equal(line.attrs['aria-controls'], detail.attrs.id);
  assert.match(detail.attrs.id, /^zlv-detail-\d+$/);
  assert.match(detail.textContent, /regionus-east-1/);
  assert.match(detail.textContent, /\{"a":1\}/);
  line.dispatch('click');
  assert.equal(detail.hidden, true);
  assert.equal(line.attrs['aria-expanded'], 'false');
  line.dispatch('click');
  assert.equal(detail.hidden, false);
  assert.deepEqual(detail.scrollIntoViewArgs, { block: 'nearest' });
});

test('dropping the focused row moves focus to the next row', () => {
  const { document, container } = mount();
  const viewer = createLogViewer(container, { document, maxRows: 2 });
  viewer.push(entry({ message: 'first', fields: { a: '1' } }));
  viewer.push(entry({ message: 'second', fields: { b: '2' } }));
  const rows = () => container.byClass('zlv-row');
  rows()[0].byClass('zlv-line')[0].focus();
  viewer.push(entry({ message: 'third', fields: { c: '3' } }));
  assert.equal(rows().length, 2);
  assert.equal(document.activeElement, rows()[0].byClass('zlv-line')[0]);
  assert.equal(rows()[0].byClass('zlv-message')[0].textContent, 'second');
});

test('dropping the only focused row moves focus to pause', () => {
  const { document, container } = mount();
  const viewer = createLogViewer(container, { document, maxRows: 1 });
  viewer.push(entry({ message: 'only', fields: { a: '1' } }));
  container.byClass('zlv-line')[0].focus();
  viewer.push(entry({ message: 'next' }));
  assert.equal(document.activeElement.className, 'zlv-follow');
  assert.equal(container.byClass('zlv-message')[0].textContent, 'next');
});

test('field detail ids stay unique across viewers', () => {
  const { document, container } = mount();
  const other = document.createElement('div');
  const first = createLogViewer(container, { document });
  const second = createLogViewer(other, { document });
  first.push(entry({ fields: { a: '1' } }));
  second.push(entry({ fields: { b: '2' } }));
  const ids = [container, other].map((node) => node.byClass('zlv-line')[0].attrs['aria-controls']);
  assert.notEqual(ids[0], ids[1]);
});

test('a row without fields is not a control', () => {
  const { viewer, rows } = setup();
  viewer.push(entry());
  const line = rows()[0].byClass('zlv-line')[0];
  assert.equal(line.tagName, 'div');
  assert.equal(line.attrs['aria-expanded'], undefined);
});

test('new lines are announced only while following', () => {
  const { container, viewer } = setup();
  const list = container.byClass('zlv-list')[0];
  const status = container.byClass('zlv-status')[0];
  assert.equal(list.attrs.role, 'log');
  assert.equal(list.attrs['aria-live'], 'polite');
  assert.equal(status.attrs['aria-live'], 'off');
  container.byClass('zlv-follow')[0].dispatch('click');
  assert.equal(list.attrs['aria-live'], 'off');
  assert.equal(status.attrs['aria-live'], 'polite');
  viewer.push(entry());
  assert.equal(status.attrs['aria-live'], 'polite');
});

test('follow-tail scrolls to the newest row until the user scrolls up', () => {
  const { container, viewer } = setup();
  const list = container.byClass('zlv-list')[0];
  viewer.push(entry());
  assert.equal(list.scrollTop, list.scrollHeight);
  list.scrollTop = 0;
  list.dispatch('scroll');
  viewer.push(entry());
  assert.equal(list.scrollTop, 0);
});

test('the row cap drops the oldest and says so', () => {
  const { viewer, rows, status } = setup({ maxRows: 3 });
  for (let i = 1; i <= 5; i += 1) viewer.push(entry({ message: `m${i}` }));
  assert.deepEqual(rows().map((r) => r.byClass('zlv-message')[0].textContent), ['m3', 'm4', 'm5']);
  assert.match(status(), /2 older lines dropped \(showing the newest 3\)/);
});

test('one dropped line uses the singular', () => {
  const { viewer, status } = setup({ maxRows: 1 });
  viewer.push(entry({ message: 'a' }));
  viewer.push(entry({ message: 'b' }));
  assert.match(status(), /Showing 1 of 1 line, 1 older line dropped \(showing the newest 1\)/);
});

test('10,000 pushes leave at most 2000 row nodes', () => {
  const { viewer, rows, status } = setup();
  for (let i = 0; i < 10000; i += 1) viewer.push(entry({ message: `m${i}` }));
  assert.equal(rows().length, 2000);
  assert.match(status(), /8000 older lines dropped \(showing the newest 2000\)/);
});

test('markup in an entry stays inert text', () => {
  const { viewer, rows, container } = setup();
  const payload = '<img src=x onerror=alert(1)>';
  viewer.push(entry({ message: payload, source: payload, fields: { k: payload } }));
  assert.equal(rows()[0].byClass('zlv-message')[0].textContent, payload);
  assert.equal(container.find((n) => n.tagName === 'img').length, 0);
});

test('pushChunk and pushLine normalise raw zerolog text', () => {
  const { viewer, rows } = setup();
  viewer.pushChunk('{"level":"debug","plugin_name":"kube","message":"a"}\n{"level":"er');
  assert.equal(rows().length, 1);
  viewer.pushChunk('ror","message":"b"}\nnot json\n');
  assert.equal(rows().length, 3);
  assert.equal(rows()[2].byClass('zlv-message')[0].textContent, 'not json');
  viewer.pushLine('{"message":"c"}');
  assert.equal(rows().length, 4);
});

test('setFilters re-renders and clear and destroy empty the viewer', () => {
  const { container, viewer, rows } = setup();
  viewer.push(entry({ source: 'a', level: 'debug', message: 'one' }));
  viewer.push(entry({ source: 'b', level: 'error', message: 'two' }));
  viewer.setFilters({ sources: ['b'] });
  assert.equal(rows().length, 1);
  viewer.setFilters({ minLevel: 'error', text: 'TW' });
  assert.equal(rows().length, 1);
  viewer.setFilters({});
  assert.equal(rows().length, 2);
  viewer.clear();
  assert.equal(rows().length, 0);
  viewer.destroy();
  assert.equal(container.childNodes.length, 0);
});
