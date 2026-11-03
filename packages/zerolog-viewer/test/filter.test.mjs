import test from 'node:test';
import assert from 'node:assert/strict';
import { matchesFilter, compileFilter } from '../src/index.js';

const core = { seq: 1, level: 'info', source: 'engine', sourceKind: 'core', message: 'row enrichment complete', fields: {} };
const plugin = { seq: 2, level: 'warn', source: 'aws-public', sourceKind: 'plugin', message: 'router starting', fields: { region: 'us-west-2' } };
const hostLine = { seq: 3, level: 'info', source: 'pluginhost', sourceKind: 'core', message: 'plugin connected successfully', fields: {} };

test('plain text matches the message and field values only', () => {
  const filter = compileFilter({ text: 'plugin' });
  assert.equal(filter.error, null);
  assert.equal(matchesFilter(core, filter), false);
  assert.equal(matchesFilter(plugin, filter), false);
  assert.equal(matchesFilter(hostLine, filter), true);
  assert.equal(matchesFilter(plugin, compileFilter({ text: 'US-WEST' })), true);
  assert.equal(matchesFilter(plugin, compileFilter({ text: 'warn' })), false);
  assert.equal(matchesFilter(plugin, compileFilter({ text: 'aws-public' })), false);
  assert.equal(matchesFilter(plugin, compileFilter({ text: 'region' })), false);
});

test('the kind filter selects core or plugin entries', () => {
  assert.equal(matchesFilter(core, compileFilter({ kind: 'plugin' })), false);
  assert.equal(matchesFilter(plugin, compileFilter({ kind: 'plugin' })), true);
  assert.equal(matchesFilter(plugin, compileFilter({ kind: 'core' })), false);
  assert.equal(matchesFilter(hostLine, compileFilter({ kind: 'core' })), true);
  assert.equal(matchesFilter(plugin, compileFilter({ kind: '' })), true);
});

test('regex mode matches a pattern case-insensitively', () => {
  const filter = compileFilter({ text: '^aws|enrichment', regex: true });
  assert.equal(filter.error, null);
  assert.equal(matchesFilter(core, filter), true);
  assert.equal(matchesFilter(plugin, filter), false);
  assert.equal(matchesFilter(hostLine, filter), false);
  assert.equal(matchesFilter(plugin, compileFilter({ text: 'us-west-\\d$', regex: true })), true);
});

test('plain text treats regex characters literally', () => {
  const filter = compileFilter({ text: 'a.c' });
  assert.equal(matchesFilter({ ...core, message: 'abc' }, filter), false);
  assert.equal(matchesFilter({ ...core, message: 'a.c' }, filter), true);
});

test('an invalid regex reports an error and matches everything', () => {
  const filter = compileFilter({ text: '(unclosed', regex: true });
  assert.equal(typeof filter.error, 'string');
  assert.ok(filter.error.length > 0);
  assert.equal(matchesFilter(core, filter), true);
});

test('text combines with source and level filters', () => {
  const filter = compileFilter({ text: 'router', sources: ['aws-public'], minLevel: 'warn' });
  assert.equal(matchesFilter(plugin, filter), true);
  assert.equal(matchesFilter({ ...plugin, level: 'info' }, filter), false);
  assert.equal(matchesFilter({ ...plugin, source: 'scorer' }, filter), false);
});

test('an empty filter matches everything', () => {
  const filter = compileFilter({});
  assert.equal(matchesFilter(core, filter), true);
  assert.equal(matchesFilter(plugin, filter), true);
});
