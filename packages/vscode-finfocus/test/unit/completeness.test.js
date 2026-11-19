'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { parseOverview } = require('../../src/result');
const { gapsOf, needsRetry, settle } = require('../../src/completeness');

const fixture = (name) =>
  parseOverview(fs.readFileSync(path.join(__dirname, '..', 'fixtures', 'completeness', name), 'utf8'));
const urn = (name) => `urn:pulumi:dev::shop::aws:ec2/instance:Instance::${name}`;

test('a result with every row priced has no gaps, and a deleting row is not one', () => {
  const result = fixture('complete.json');
  assert.equal(result.rows.at(-1).display.projected, '-');
  assert.deepEqual(gapsOf(result), []);
});

test('rows printing the placeholder, other than deleting rows, are gaps', () => {
  assert.deepEqual(gapsOf(fixture('partial.json')), ['b', 'c', 'd', 'e', 'f'].map(urn));
  assert.deepEqual(gapsOf(fixture('stable-gaps.json')), [urn('dns')]);
});

test('no gaps never needs a retry', () => {
  assert.equal(needsRetry([], null), false);
  assert.equal(needsRetry([], [urn('dns')]), false);
});

test('gaps with no baseline need a retry', () => {
  assert.equal(needsRetry([urn('dns')], null), true);
});

test('gaps the baseline already confirmed need no retry; a new gap does', () => {
  assert.equal(needsRetry([urn('dns')], [urn('dns'), urn('x')]), false);
  assert.equal(needsRetry([urn('dns'), urn('b')], [urn('dns')]), true);
});

test('equal gap sets across two runs are confirmed', () => {
  const outcome = settle({ first: [urn('dns')], second: [urn('dns')], baseline: null });
  assert.equal(outcome.kind, 'confirmed');
  assert.deepEqual(outcome.gaps, [urn('dns')]);
  assert.equal(outcome.keep, 'second');
});

test('differing gap sets are incomplete and keep the run with fewer gaps', () => {
  const partial = ['b', 'c', 'd', 'e', 'f'].map(urn);
  const better = settle({ first: partial, second: [urn('f')], baseline: null });
  assert.equal(better.kind, 'incomplete');
  assert.equal(better.keep, 'second');
  assert.deepEqual(better.gaps, [urn('f')]);
  const worse = settle({ first: [urn('f')], second: partial, baseline: null });
  assert.equal(worse.kind, 'incomplete');
  assert.equal(worse.keep, 'first');
});

test('a tie between differing sets keeps the later run', () => {
  const outcome = settle({ first: [urn('b')], second: [urn('c')], baseline: null });
  assert.equal(outcome.kind, 'incomplete');
  assert.equal(outcome.keep, 'second');
});

test('a retry whose gaps all sit inside the baseline is confirmed', () => {
  const outcome = settle({ first: [urn('b'), urn('dns')], second: [urn('dns')], baseline: [urn('dns')] });
  assert.equal(outcome.kind, 'confirmed');
});

test('a retry that clears every gap is complete', () => {
  const outcome = settle({ first: ['b', 'c'].map(urn), second: [], baseline: null });
  assert.equal(outcome.kind, 'complete');
  assert.equal(outcome.keep, 'second');
});

test('known limit: the same rows failing on both runs look confirmed, not incomplete', () => {
  const failing = ['b', 'c'].map(urn);
  assert.equal(settle({ first: failing, second: [...failing], baseline: null }).kind, 'confirmed');
});

test('a result with no gaps on the first run is complete without a second run', () => {
  assert.deepEqual(settle({ first: [], second: null, baseline: null }), { kind: 'complete', gaps: [], keep: 'first' });
});

test('a first run inside the baseline settles confirmed without a second run', () => {
  const outcome = settle({ first: [urn('dns')], second: null, baseline: [urn('dns')] });
  assert.equal(outcome.kind, 'confirmed');
  assert.equal(outcome.keep, 'first');
});

test('the placeholder and exempt statuses come from the contract inventory', () => {
  const result = fixture('complete.json');
  const inventory = {
    ...require('../../contract/overview-fields.json'),
    placeholders: { unpriced: { path: 'resources[].display.projected', value: '$10.00', exemptStatuses: [] } },
  };
  assert.deepEqual(gapsOf(result, inventory), [urn('a')]);
});
