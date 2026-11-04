import test from 'node:test';
import assert from 'node:assert/strict';
import { createLogViewer } from '../src/index.js';
import { mount } from './dom.mjs';

const entry = (over = {}) => ({
  time: '2026-11-02T10:00:00Z', level: 'info', source: 'core', sourceKind: 'core', message: 'hello', ...over,
});

function setup() {
  const { document, container } = mount();
  const viewer = createLogViewer(container, { document });
  return {
    container,
    viewer,
    rows: () => container.byClass('zlv-row'),
    search: container.byClass('zlv-search')[0],
    regex: container.byClass('zlv-regex')[0],
    count: () => /(\d+ shown of \d+)/.exec(container.byClass('zlv-status')[0].textContent)[1],
    error: () => container.byClass('zlv-filter-error')[0].textContent,
  };
}

function type(search, value) {
  search.value = value;
  search.dispatch('input');
}

test('typing in the search box filters rows and updates the match count', () => {
  const { viewer, rows, search, count } = setup();
  viewer.push(entry({ source: 'engine', message: 'enrichment done' }));
  viewer.push(entry({ source: 'aws-public', sourceKind: 'plugin', message: 'router starting' }));
  assert.equal(count(), '2 shown of 2');
  type(search, 'router');
  assert.equal(rows().length, 1);
  assert.equal(rows()[0].byClass('zlv-source')[0].textContent, 'aws-public');
  assert.equal(count(), '1 shown of 2');
  viewer.push(entry({ source: 'scorer', sourceKind: 'plugin', message: 'router ready' }));
  assert.equal(rows().length, 2);
  assert.equal(count(), '2 shown of 3');
  type(search, '');
  assert.equal(rows().length, 3);
});

test('the regex toggle switches the search to a pattern and flags a bad one', () => {
  const { viewer, rows, search, regex, error } = setup();
  viewer.push(entry({ source: 'engine', message: 'enrichment done' }));
  viewer.push(entry({ source: 'aws-public', sourceKind: 'plugin', message: 'router starting' }));
  type(search, 'enrich|router');
  assert.equal(rows().length, 0);
  regex.checked = true;
  regex.dispatch('change');
  assert.equal(rows().length, 2);
  assert.equal(error(), '');
  type(search, '(oops');
  assert.notEqual(error(), '');
  assert.equal(search.attrs['aria-invalid'], 'true');
  assert.equal(rows().length, 2);
  type(search, 'router');
  assert.equal(error(), '');
  assert.equal(search.attrs['aria-invalid'], 'false');
  assert.equal(rows().length, 1);
});

test('setFilters keeps the search box in step', () => {
  const { viewer, search, regex } = setup();
  viewer.setFilters({ text: 'abc', regex: true });
  assert.equal(search.value, 'abc');
  assert.equal(regex.checked, true);
});
