import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import * as lib from '../src/index.js';
import { mount } from './dom.mjs';

const readme = readFileSync(fileURLToPath(new URL('../README.md', import.meta.url)), 'utf8');
const AsyncFunction = Object.getPrototypeOf(async () => {}).constructor;

const samples = [...readme.matchAll(/```javascript\n([\s\S]*?)```/g)].map((m) => m[1]);

function prepare(source) {
  return source.replace(
    /import\s*\{([^}]*)\}\s*from\s*'@rshade\/zerolog-viewer';/g,
    (_, names) => `const {${names}} = lib;`,
  );
}

test('the README has JavaScript samples', () => {
  assert.ok(samples.length >= 3);
});

for (const [index, source] of samples.entries()) {
  test(`README sample ${index + 1} runs against the DOM double`, async () => {
    const { document, container } = mount();
    const run = new AsyncFunction('lib', 'container', 'document', prepare(source));
    await run(lib, container, document);
  });
}

test('the fieldMap sample renders the mapped source and level', async () => {
  const source = samples.find((s) => s.includes('fieldMap'));
  assert.ok(source);
  const { document, container } = mount();
  await new AsyncFunction('lib', 'container', 'document', prepare(source))(lib, container, document);
  const row = container.byClass('zlv-row')[0];
  assert.equal(row.byClass('zlv-source')[0].textContent, 'billing');
  assert.equal(row.byClass('zlv-message')[0].textContent, 'slow');
  assert.equal(row.attrs['data-level'], 'warn');
});
