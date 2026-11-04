import test from 'node:test';
import assert from 'node:assert/strict';
import { createLogViewer } from '../src/index.js';
import { mount } from './dom.mjs';

const payload = '<img src=x onerror=alert(1)></script><script>alert(2)</script>';

function tagsIn(container) {
  return new Set(container.find(() => true).map((n) => n.tagName));
}

test('markup in a message and field values renders as literal text', () => {
  const { document, container } = mount();
  const viewer = createLogViewer(container, { document });
  viewer.push({
    time: '2026-11-02T10:00:00Z', level: 'error', source: payload, sourceKind: 'plugin',
    message: payload, fields: { note: payload, nested: { inner: payload } },
  });
  const row = container.byClass('zlv-row')[0];
  row.byClass('zlv-line')[0].dispatch('click');

  assert.equal(row.byClass('zlv-message')[0].textContent, payload);
  assert.equal(row.byClass('zlv-source')[0].textContent, payload);
  const values = row.byClass('zlv-fields')[0].childNodes.filter((n) => n.tagName === 'dd');
  assert.equal(values[0].textContent, payload);
  assert.ok(values[1].textContent.includes('onerror=alert(1)'));

  const tags = tagsIn(container);
  for (const banned of ['img', 'script', 'style', 'iframe']) assert.equal(tags.has(banned), false, banned);
});

test('a hostile source name becomes chip text, never an element', () => {
  const { document, container } = mount();
  const viewer = createLogViewer(container, { document, sources: [payload] });
  viewer.push({ time: '2026-11-02T10:00:00Z', level: 'info', source: payload, sourceKind: 'core', message: payload });
  const tags = tagsIn(container);
  assert.equal(tags.has('img'), false);
  assert.equal(tags.has('script'), false);
  const chips = container.byClass('zlv-chip');
  assert.equal(chips.length, 1);
  assert.equal(chips[0].textContent, payload);
});
