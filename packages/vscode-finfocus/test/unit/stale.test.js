'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { FreshnessModel, StaleStore, REASONS } = require('../../src/stale');

const T0 = Date.parse('2026-11-17T10:00:00Z');

function row(name, file, modifiedAt = '2026-11-17T10:00:00Z') {
  return {
    urn: `urn:pulumi:dev::shop::t:m:T::${name}`,
    location: file === null ? null : { file, line: 1 },
    modifiedAt,
  };
}

const urnOf = (name) => `urn:pulumi:dev::shop::t:m:T::${name}`;

test('a deployed row is fresh when the file is not newer than modifiedAt', () => {
  const model = new FreshnessModel();
  const rows = [row('older', 'a.js'), row('equal', 'b.js')];
  const mtimes = new Map([
    ['a.js', T0 - 1000],
    ['b.js', T0],
  ]);
  assert.equal(model.staleSet(rows, 'state', mtimes).size, 0);
});

test('a file newer than modifiedAt marks its rows edited', () => {
  const model = new FreshnessModel();
  const stale = model.staleSet([row('web', 'a.js')], 'state', new Map([['a.js', T0 + 1]]));
  assert.deepEqual(stale.get(urnOf('web')), [REASONS.edited]);
});

test('missing or invalid modifiedAt, an unreadable file, or no location are freshness unknown', () => {
  const model = new FreshnessModel();
  const rows = [
    row('none', 'a.js', null),
    row('garbage', 'a.js', 'not a date'),
    row('unreadable', 'gone.js'),
    row('nolocation', null),
  ];
  const mtimes = new Map([
    ['a.js', T0],
    ['gone.js', null],
  ]);
  const stale = model.staleSet(rows, 'state', mtimes);
  for (const name of ['none', 'garbage', 'unreadable', 'nolocation']) {
    assert.deepEqual(stale.get(urnOf(name)), [REASONS.unknown], name);
  }
});

test('a file with no stat result is unknown, never fresh', () => {
  const model = new FreshnessModel();
  const stale = model.staleSet([row('web', 'a.js')], 'state', new Map());
  assert.deepEqual(stale.get(urnOf('web')), [REASONS.unknown]);
});

test('an edit marks every row declared in that file, saved or not', () => {
  const model = new FreshnessModel();
  const rows = [row('a1', 'a.js'), row('a2', 'a.js'), row('b1', 'b.js')];
  const mtimes = new Map([['a.js', T0], ['b.js', T0]]);
  model.recordEdit('a.js', true);
  let stale = model.staleSet(rows, 'state', mtimes);
  assert.deepEqual(stale.get(urnOf('a1')), [REASONS.unsaved]);
  assert.deepEqual(stale.get(urnOf('a2')), [REASONS.unsaved]);
  assert.equal(stale.has(urnOf('b1')), false);
  model.recordSave('a.js');
  stale = model.staleSet(rows, 'state', mtimes);
  assert.deepEqual(stale.get(urnOf('a1')), [REASONS.edited]);
});

test('a pre-existing dirty document is stale from the first evaluation', () => {
  const model = new FreshnessModel();
  model.recordEdit('a.js', true);
  const stale = model.staleSet([row('a1', 'a.js')], 'state', new Map([['a.js', T0 - 5]]));
  assert.deepEqual(stale.get(urnOf('a1')), [REASONS.unsaved]);
});

test('an external change, rename or deletion marks the file edited', () => {
  const model = new FreshnessModel();
  model.recordExternalChange('a.js');
  const stale = model.staleSet([row('a1', 'a.js')], 'state', new Map([['a.js', null]]));
  assert.deepEqual(stale.get(urnOf('a1')), [REASONS.edited]);
});

test('a stack settings change marks every row, even without a location', () => {
  const model = new FreshnessModel();
  model.recordConfigChange();
  const stale = model.staleSet([row('a1', 'a.js'), row('x', null)], 'state', new Map([['a.js', T0]]));
  assert.deepEqual(stale.get(urnOf('a1')), [REASONS.config]);
  assert.deepEqual(stale.get(urnOf('x')), [REASONS.config]);
});

test('a state-only refresh preserves edit and config marks', () => {
  const model = new FreshnessModel();
  const rows = [row('a1', 'a.js')];
  const mtimes = new Map([['a.js', T0]]);
  model.recordEdit('a.js', false);
  model.recordConfigChange();
  const before = model.staleSet(rows, 'state', mtimes);
  model.applyState();
  assert.deepEqual(model.staleSet(rows, 'state', mtimes), before);
  assert.deepEqual(before.get(urnOf('a1')), [REASONS.edited, REASONS.config]);
});

test('a covering preview with no later edits clears edit and config marks', () => {
  const model = new FreshnessModel();
  const rows = [row('a1', 'a.js'), row('b1', 'b.js')];
  model.recordEdit('a.js', false);
  model.recordConfigChange();
  const snapshot = model.snapshot();
  model.applyPreview(rows, snapshot);
  assert.equal(model.staleSet(rows, 'preview', new Map()).size, 0);
});

test('an edit after the request started stays stale even if saved before completion', () => {
  const model = new FreshnessModel();
  const rows = [row('a1', 'a.js'), row('b1', 'b.js')];
  model.recordEdit('a.js', false);
  model.recordEdit('b.js', false);
  const snapshot = model.snapshot();
  model.recordEdit('a.js', true);
  model.recordSave('a.js');
  model.applyPreview(rows, snapshot);
  const stale = model.staleSet(rows, 'preview', new Map());
  assert.deepEqual(stale.get(urnOf('a1')), [REASONS.edited]);
  assert.equal(stale.has(urnOf('b1')), false, 'generations are per file');
});

test('a file dirty at request start is not cleared even when saved afterwards', () => {
  const model = new FreshnessModel();
  const rows = [row('a1', 'a.js')];
  model.recordEdit('a.js', true);
  const snapshot = model.snapshot();
  model.recordSave('a.js');
  model.applyPreview(rows, snapshot);
  assert.deepEqual(model.staleSet(rows, 'preview', new Map()).get(urnOf('a1')), [REASONS.edited]);
});

test('a file still dirty when the preview finishes is not cleared', () => {
  const model = new FreshnessModel();
  const rows = [row('a1', 'a.js')];
  model.recordEdit('a.js', false);
  const snapshot = model.snapshot();
  model.recordEdit('a.js', true);
  model.applyPreview(rows, snapshot);
  assert.deepEqual(model.staleSet(rows, 'preview', new Map()).get(urnOf('a1')), [REASONS.unsaved]);
});

test('a config change during the run keeps the config mark', () => {
  const model = new FreshnessModel();
  const rows = [row('a1', 'a.js')];
  model.recordConfigChange();
  const snapshot = model.snapshot();
  model.recordConfigChange();
  model.applyPreview(rows, snapshot);
  assert.deepEqual(model.staleSet(rows, 'preview', new Map()).get(urnOf('a1')), [REASONS.config]);
});

test('a preview clears only files its rows cover', () => {
  const model = new FreshnessModel();
  model.recordEdit('a.js', false);
  model.recordEdit('elsewhere.js', false);
  model.applyPreview([row('a1', 'a.js')], model.snapshot());
  const rows = [row('a1', 'a.js'), row('e1', 'elsewhere.js')];
  const stale = model.staleSet(rows, 'preview', new Map());
  assert.equal(stale.has(urnOf('a1')), false);
  assert.deepEqual(stale.get(urnOf('e1')), [REASONS.edited]);
});

test('after a preview, a row without a location is still freshness unknown', () => {
  const model = new FreshnessModel();
  const rows = [row('x', null)];
  model.applyPreview(rows, model.snapshot());
  assert.deepEqual(model.staleSet(rows, 'preview', new Map()).get(urnOf('x')), [REASONS.unknown]);
});

test('preview basis does not consult timestamps', () => {
  const model = new FreshnessModel();
  const stale = model.staleSet([row('a1', 'a.js')], 'preview', new Map([['a.js', T0 + 99999]]));
  assert.equal(stale.size, 0);
});

test('a snapshot is a copy: later edits do not change it', () => {
  const model = new FreshnessModel();
  const snapshot = model.snapshot();
  model.recordEdit('a.js', true);
  assert.equal(snapshot.dirty.size, 0);
  assert.equal(snapshot.generations.size, 0);
});

test('the store keeps one model per context and forgets a dropped one', () => {
  const store = new StaleStore();
  const a = store.model('["p","dev"]');
  assert.equal(store.model('["p","dev"]'), a);
  assert.notEqual(store.model('["p","test"]'), a);
  store.drop('["p","dev"]');
  assert.notEqual(store.model('["p","dev"]'), a);
  assert.deepEqual(store.keysForProject('p').sort(), ['["p","dev"]', '["p","test"]']);
});
