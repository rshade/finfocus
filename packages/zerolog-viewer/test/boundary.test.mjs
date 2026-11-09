import test from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import { isBuiltin } from 'node:module';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const packageRoot = path.resolve(fileURLToPath(new URL('..', import.meta.url)));
const testDir = path.dirname(fileURLToPath(import.meta.url));
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

function outsidePackage(resolved) {
  const rel = path.relative(packageRoot, resolved);
  return rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel);
}

function quotedSpecs(source, patterns) {
  const specs = [];
  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) specs.push(match[1]);
  }
  return specs;
}

function resolvedURL(spec, fromDir) {
  const base = pathToFileURL(path.join(fromDir, 'file.mjs'));
  return fileURLToPath(new URL(spec, base));
}

function resolvedImport(spec, fromDir) {
  if (spec.startsWith('.') || spec.startsWith('file:')) return resolvedURL(spec, fromDir);
  if (path.isAbsolute(spec)) return spec;
  return '';
}

export function leavingPaths(file, source, fromDir) {
  const found = [];
  const imports = quotedSpecs(source, [
    /\bfrom\s+['"]([^'"]+)['"]/g,
    /\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
    /^\s*import\s+['"]([^'"]+)['"]/gm,
  ]);
  for (const spec of imports) {
    try {
      const resolved = resolvedImport(spec, fromDir);
      if (!resolved && !isBuiltin(spec)) found.push(`${file}: bare import ${spec}`);
      else if (resolved && outsidePackage(resolved)) found.push(`${file}: leaves package: ${spec}`);
    } catch {
      found.push(`${file}: unreadable path: ${spec}`);
    }
  }
  for (const spec of quotedSpecs(source, [/new\s+URL\(\s*['"]([^'"]+)['"]/g])) {
    try {
      if (outsidePackage(resolvedURL(spec, fromDir))) found.push(`${file}: leaves package: ${spec}`);
    } catch {
      found.push(`${file}: unreadable path: ${spec}`);
    }
  }
  return found;
}

async function javascriptFiles(dir) {
  const out = [];
  for (const ent of await readdir(dir, { withFileTypes: true })) {
    const full = path.join(dir, ent.name);
    if (ent.isDirectory()) out.push(...await javascriptFiles(full));
    else if (ent.isFile() && /\.(?:mjs|js)$/.test(ent.name)) out.push(full);
  }
  return out;
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
  const outsideSrc = '../'.repeat(2) + 'x.js';
  assert.equal(violations('x.js', `import a from ${JSON.stringify(outsideSrc)};`).length > 0, true);
  assert.equal(violations('x.js', 'el.innerHTML = s;').length > 0, true);
  assert.deepEqual(violations('x.js', "import a from './viewer.js';"), []);
});

test('test and src paths stay inside the package', async () => {
  const files = [...await javascriptFiles(testDir), ...await javascriptFiles(srcDir)];
  assert.ok(files.length > 0);
  for (const full of files) {
    const source = await readFile(full, 'utf8');
    assert.deepEqual(leavingPaths(path.relative(packageRoot, full), source, path.dirname(full)), []);
  }
});

test('the check catches a path that leaves the package', () => {
  const up = '../'.repeat(3);
  const encoded = '%2e%2e/%2e%2e/%2e%2e/';
  const urlCall = (spec) => `readFile(new URL(${JSON.stringify(spec)}, import.meta.url))`;
  const cases = [
    urlCall(`${up}internal/weblog/testdata/sample.ndjson`),
    urlCall(`fixtures/${up}internal/weblog/testdata/sample.ndjson`),
    urlCall(`./${encoded}internal/weblog/testdata/sample.ndjson`),
    urlCall('file:' + '//etc/passwd'),
    `import x from ${JSON.stringify('file:' + '//etc/passwd.js')}`,
    `import x from ${JSON.stringify(`./${encoded}internal/weblog/x.mjs`)}`,
    `import x from ${JSON.stringify('left-pad')}`,
  ];
  for (const source of cases) {
    assert.equal(leavingPaths('ndjson.test.mjs', source, testDir).length > 0, true, source);
  }
  const inside = "readFile(new URL('./fixtures/sample.ndjson', import.meta.url))";
  assert.deepEqual(leavingPaths('ndjson.test.mjs', inside, testDir), []);
  assert.deepEqual(leavingPaths('ndjson.test.mjs', "import test from 'node:test';", testDir), []);
});
