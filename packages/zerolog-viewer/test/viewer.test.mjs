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
  const { viewer, rows } = setup();
  viewer.push(entry({ fields: { region: 'us-east-1', n: 3, nested: { a: 1 } } }));
  const row = rows()[0];
  assert.equal(row.byClass('zlv-fields').length, 0);
  row.byClass('zlv-line')[0].dispatch('click');
  const detail = row.byClass('zlv-fields')[0];
  assert.match(detail.textContent, /regionus-east-1/);
  assert.match(detail.textContent, /\{"a":1\}/);
  row.byClass('zlv-line')[0].dispatch('click');
  assert.equal(detail.hidden, true);
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
  assert.match(status(), /2 older entries dropped/);
});

test('10,000 pushes leave at most 2000 row nodes', () => {
  const { viewer, rows, status } = setup();
  for (let i = 0; i < 10000; i += 1) viewer.push(entry({ message: `m${i}` }));
  assert.equal(rows().length, 2000);
  assert.match(status(), /8000 older entries dropped/);
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
