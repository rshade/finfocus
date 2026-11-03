import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createNDJSONParser, parseNDJSON, RawLine } from '../src/ndjson.js';

test('a line split across chunks is joined', () => {
  const parser = createNDJSONParser();
  assert.equal(parser.push('{"message":"he').length, 0);
  const items = parser.push('llo"}\n{"message":"two"}\n');
  assert.deepEqual(items.map((i) => i.message), ['hello', 'two']);
});

test('the unterminated tail is kept until flush', () => {
  const parser = createNDJSONParser();
  assert.equal(parser.push('{"message":"tail"}').length, 0);
  assert.deepEqual(parser.flush().map((i) => i.message), ['tail']);
  assert.equal(parser.flush().length, 0);
});

test('CRLF and blank lines are handled', () => {
  const items = parseNDJSON('{"message":"a"}\r\n\r\n   \n{"message":"b"}\r\n');
  assert.deepEqual(items.map((i) => i.message), ['a', 'b']);
});

test('bad JSON and non-object JSON are kept as raw lines', () => {
  const items = parseNDJSON('plain text\n{broken\n123\n[1]\n{"message":"ok"}\n');
  assert.equal(items.length, 5);
  for (const item of items.slice(0, 4)) assert.ok(item instanceof RawLine);
  assert.deepEqual(items.slice(0, 4).map((i) => i.text), ['plain text', '{broken', '123', '[1]']);
  assert.equal(items[4].message, 'ok');
});

test('a very long unterminated tail is flushed as a raw line', () => {
  const parser = createNDJSONParser();
  const items = parser.push('x'.repeat(70 * 1024));
  assert.equal(items.length, 1);
  assert.ok(items[0] instanceof RawLine);
});

test('the shared sample fixture parses to one item per non-blank line', async () => {
  const text = await readFile(new URL('../../../internal/weblog/testdata/sample.ndjson', import.meta.url), 'utf8');
  const lines = text.split('\n').filter((l) => l.trim() !== '');
  const items = parseNDJSON(text);
  assert.equal(items.length, lines.length);
  assert.ok(items.some((i) => i instanceof RawLine));
});
