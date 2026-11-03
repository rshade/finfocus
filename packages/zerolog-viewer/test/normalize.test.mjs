import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeEntry } from '../src/normalize.js';
import { RawLine } from '../src/ndjson.js';

const now = () => '2030-01-01T00:00:00.000Z';

test('default zerolog mapping', () => {
  const entry = normalizeEntry({
    level: 'warn', time: '2026-11-02T10:00:00Z', message: 'm', plugin_name: 'aws-public', region: 'us-east-1',
  }, { now });
  assert.equal(entry.level, 'warn');
  assert.equal(entry.time, '2026-11-02T10:00:00.000Z');
  assert.equal(entry.message, 'm');
  assert.equal(entry.source, 'aws-public');
  assert.equal(entry.sourceKind, 'plugin');
  assert.deepEqual(entry.fields, { plugin_name: 'aws-public', region: 'us-east-1' });
  assert.equal('time' in entry.fields, false);
});

test('component is a core source and the fallback is core', () => {
  const a = normalizeEntry({ component: 'engine', message: 'x' }, { now });
  assert.equal(a.source, 'engine');
  assert.equal(a.sourceKind, 'core');
  const b = normalizeEntry({ message: 'x' }, { now });
  assert.equal(b.source, 'core');
  assert.equal(b.sourceKind, 'core');
  assert.equal(b.fields, undefined);
});

test('sourceFields order decides the source', () => {
  const raw = { plugin_name: 'p', component: 'c', service: 's' };
  assert.equal(normalizeEntry(raw, { now }).source, 'p');
  assert.equal(normalizeEntry(raw, { now, sourceFields: ['component', 'plugin_name'] }).source, 'c');
  assert.equal(normalizeEntry(raw, { now, sourceFields: ['service'] }).source, 's');
});

test('unknown or missing level becomes info; numeric unix time is accepted', () => {
  assert.equal(normalizeEntry({ level: 'loud' }, { now }).level, 'info');
  assert.equal(normalizeEntry({}, { now }).level, 'info');
  assert.equal(normalizeEntry({ level: 'WARNING' }, { now }).level, 'warn');
  assert.equal(normalizeEntry({ time: 1762077600 }, { now }).time, '2025-11-02T10:00:00.000Z');
  assert.equal(normalizeEntry({ time: 'garbage' }, { now }).time, now());
});

test('custom fieldMap for another logger', () => {
  const entry = normalizeEntry({ lvl: 'error', ts: '2026-11-02T10:00:00Z', msg: 'boom', svc: 'api' }, {
    now, fieldMap: { level: 'lvl', time: 'ts', message: 'msg' }, sourceFields: ['svc'],
  });
  assert.equal(entry.level, 'error');
  assert.equal(entry.message, 'boom');
  assert.equal(entry.source, 'api');
  assert.deepEqual(entry.fields, { svc: 'api' });
});

test('a raw line becomes a raw info entry from unknown', () => {
  const entry = normalizeEntry(new RawLine('plain text'), { now });
  assert.equal(entry.raw, true);
  assert.equal(entry.level, 'info');
  assert.equal(entry.message, 'plain text');
  assert.equal(entry.source, 'unknown');
  assert.equal(entry.sourceKind, 'core');
  assert.equal(entry.fields, undefined);
});
