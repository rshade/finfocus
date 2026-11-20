'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..', '..');
const manifest = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));

test('engine, publisher and extension kind are the supported values', () => {
  assert.equal(manifest.engines.vscode, '^1.90.0');
  assert.equal(manifest.publisher, 'finfocus');
  assert.deepEqual(manifest.extensionKind, ['workspace']);
});

test('untrusted workspaces are limited and say why', () => {
  const support = manifest.capabilities.untrustedWorkspaces;
  assert.equal(support.supported, 'limited');
  assert.match(support.description, /trusted/);
});

test('package metadata is complete enough for a store listing', () => {
  assert.match(manifest.name, /^[a-z][a-z0-9-]*$/);
  assert.ok(manifest.displayName);
  assert.ok(manifest.description);
  assert.match(manifest.version, /^\d+\.\d+\.\d+$/);
  assert.equal(manifest.license, 'Apache-2.0');
  assert.equal(manifest.repository.directory, 'packages/vscode-finfocus');
  assert.ok(fs.statSync(path.join(root, 'LICENSE')).size > 0);
  assert.ok(fs.statSync(path.join(root, 'README.md')).size > 0);
});

test('the extension has no runtime dependencies and no proposed APIs', () => {
  assert.equal(manifest.dependencies, undefined);
  assert.equal(manifest.enabledApiProposals, undefined);
});

test('minFinfocusVersion is a plain semver', () => {
  assert.match(manifest.minFinfocusVersion, /^\d+\.\d+\.\d+$/);
});

test('the icon is a PNG file', () => {
  const icon = fs.readFileSync(path.join(root, manifest.icon));
  assert.deepEqual([...icon.subarray(0, 8)], [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
});

test('the package is an allow-list that leaves out tests, secrets and editor downloads', () => {
  const lines = fs
    .readFileSync(path.join(root, '.vscodeignore'), 'utf8')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
  assert.equal(lines[0], '**', 'everything is excluded before the allow-list');
  const included = lines.filter((line) => line.startsWith('!')).map((line) => line.slice(1));
  assert.ok(included.includes('package.json'));
  assert.ok(included.includes('LICENSE'));
  assert.ok(included.includes('src/**'));
  for (const entry of included) {
    assert.doesNotMatch(entry, /test|node_modules|\.vscode-test|\.env|\.npmrc|pnpm|jsconfig/, entry);
  }
});

test('every file the manifest names ships in the package', () => {
  const included = fs
    .readFileSync(path.join(root, '.vscodeignore'), 'utf8')
    .split('\n')
    .filter((line) => line.startsWith('!'))
    .map((line) => line.slice(1));
  const covered = (file) =>
    included.some((entry) => (entry.endsWith('/**') ? file.startsWith(entry.slice(0, -2)) : file === entry));
  assert.ok(covered(manifest.main.replace(/^\.\//, '')), manifest.main);
  assert.ok(covered(manifest.icon), manifest.icon);
});
