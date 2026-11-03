import test from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const srcDir = fileURLToPath(new URL('../src/', import.meta.url));

export function violations(file, source) {
  const found = [];
  const specifiers = [
    ...source.matchAll(/\bfrom\s+['"]([^'"]+)['"]/g),
    ...source.matchAll(/\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)/g),
    ...source.matchAll(/^\s*import\s+['"]([^'"]+)['"]/gm),
  ].map((m) => m[1]);
  for (const spec of specifiers) {
    if (!spec.startsWith('.')) found.push(`${file}: bare import ${spec}`);
    else if (!path.resolve(srcDir, path.dirname(file), spec).startsWith(srcDir)) found.push(`${file}: leaves src: ${spec}`);
    if (/app\.js/.test(spec)) found.push(`${file}: imports app.js`);
  }
  if (/\b(fetch|EventSource|XMLHttpRequest)\s*\(|new\s+EventSource/.test(source)) found.push(`${file}: network access`);
  if (/\.innerHTML|\beval\s*\(|new\s+Function/.test(source)) found.push(`${file}: unsafe sink`);
  return found;
}

test('src imports nothing outside src and has no network or unsafe sinks', async () => {
  const files = (await readdir(srcDir)).filter((f) => f.endsWith('.js'));
  assert.ok(files.length > 0);
  for (const file of files) {
    assert.deepEqual(violations(file, await readFile(path.join(srcDir, file), 'utf8')), []);
  }
});

test('the check catches a planted bad import', () => {
  assert.equal(violations('x.js', "import { el } from '../app.js';").length > 0, true);
  assert.equal(violations('x.js', "import a from '../../x.js';").length > 0, true);
  assert.equal(violations('x.js', 'el.innerHTML = s;').length > 0, true);
  assert.deepEqual(violations('x.js', "import a from './viewer.js';"), []);
});
