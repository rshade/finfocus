'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { projectAnnotations, declarationTarget, clampPosition, resourceName } = require('../../src/annotations');

function row(name, overrides = {}) {
  return {
    urn: `urn:pulumi:dev::shop::aws:ec2/instance:Instance::${name}`,
    type: 'aws:ec2/instance:Instance',
    status: 'active',
    monthlyCost: 10,
    location: { file: 'lib/web.ts', line: 12, column: 5 },
    modifiedAt: null,
    display: { projected: '$61.32', actual: '$18.40', delta: '+$30.66', status: 'active' },
    ...overrides,
  };
}

test('one resource annotates its declaration line with the CLI projected string', () => {
  const byFile = projectAnnotations([row('web')]);
  assert.deepEqual([...byFile.keys()], ['lib/web.ts']);
  const [annotation] = byFile.get('lib/web.ts');
  assert.equal(annotation.line, 12);
  assert.equal(annotation.text, '$61.32');
  assert.equal(annotation.count, 1);
  assert.deepEqual(annotation.urns, [row('web').urn]);
});

test('the hover shows actual, delta and status when finfocus reports them', () => {
  const [annotation] = projectAnnotations([row('web')]).get('lib/web.ts');
  assert.deepEqual(annotation.hover, ['web', 'Projected $61.32 · Actual $18.40 · Delta +$30.66 · active']);
});

test('the hover leaves out values finfocus prints as "-"', () => {
  const quiet = row('web', { display: { projected: '$5.00', actual: '-', delta: '-', status: '+ creating' } });
  const [annotation] = projectAnnotations([quiet]).get('lib/web.ts');
  assert.deepEqual(annotation.hover, ['web', 'Projected $5.00 · + creating']);
});

test('resources sharing a line show a count and list each figure without summing', () => {
  const rows = [
    row('b', { display: { projected: '$2.00', actual: '-', delta: '-', status: 'active' } }),
    row('a', { display: { projected: '$1.00', actual: '-', delta: '-', status: 'active' } }),
    row('c', { display: { projected: '$3.00', actual: '-', delta: '-', status: 'active' } }),
  ];
  const [annotation] = projectAnnotations(rows).get('lib/web.ts');
  assert.equal(annotation.text, '3 resources');
  assert.equal(annotation.count, 3);
  assert.deepEqual(annotation.hover, [
    'a', 'Projected $1.00 · active',
    'b', 'Projected $2.00 · active',
    'c', 'Projected $3.00 · active',
  ]);
  assert.doesNotMatch(annotation.text + annotation.hover.join(' '), /\$6/);
});

test('rows on different lines or files get separate annotations, ordered by line', () => {
  const rows = [
    row('late', { location: { file: 'lib/web.ts', line: 40 } }),
    row('early', { location: { file: 'lib/web.ts', line: 3 } }),
    row('other', { location: { file: 'lib/db.ts', line: 1 } }),
  ];
  const byFile = projectAnnotations(rows);
  assert.deepEqual(byFile.get('lib/web.ts').map((a) => a.line), [3, 40]);
  assert.deepEqual(byFile.get('lib/db.ts').map((a) => a.line), [1]);
});

test('a missing column still annotates the line', () => {
  const noColumn = row('web', { location: { file: 'main.go', line: 30 } });
  const [annotation] = projectAnnotations([noColumn]).get('main.go');
  assert.equal(annotation.line, 30);
});

test('a row with no location places no annotation', () => {
  assert.equal(projectAnnotations([row('web', { location: null })]).size, 0);
});

test('locations outside the project are skipped, whatever shape they take', () => {
  for (const file of ['/etc/passwd', '../outside.ts', 'a/../../b.ts', 'C:\\x\\y.ts', 'C:/x/y.ts', '\\\\host\\share\\a.ts', 'a\\b.ts']) {
    const unsafe = row('web', { location: { file, line: 1 } });
    assert.equal(projectAnnotations([unsafe]).size, 0, file);
  }
});

test('an unpriced row shows the CLI marker and never a figure', () => {
  for (const marker of ['-', 'ERR']) {
    const unpriced = row('web', { monthlyCost: 0, display: { projected: marker, actual: '-', delta: '-', status: 'active' } });
    const [annotation] = projectAnnotations([unpriced]).get('lib/web.ts');
    assert.equal(annotation.text, marker);
    assert.equal(annotation.unpriced, true);
  }
});

test('a real zero price passes through as the CLI printed it', () => {
  const free = row('web', { monthlyCost: 0, display: { projected: '$0.00', actual: '-', delta: '-', status: 'active' } });
  const [annotation] = projectAnnotations([free]).get('lib/web.ts');
  assert.equal(annotation.text, '$0.00');
  assert.equal(annotation.unpriced, false);
});

test('a shared line is unpriced only when every resource on it is', () => {
  const rows = [
    row('a', { display: { projected: '-', actual: '-', delta: '-', status: 'active' } }),
    row('b'),
  ];
  const [annotation] = projectAnnotations(rows).get('lib/web.ts');
  assert.equal(annotation.unpriced, false);
});

test('the resource name is the last URN segment', () => {
  assert.equal(resourceName('urn:pulumi:dev::shop::aws:ec2/instance:Instance::web'), 'web');
  assert.equal(resourceName('urn:pulumi:dev::shop::comp$aws:s3/bucket:Bucket::logs'), 'logs');
  assert.equal(resourceName('plain'), 'plain');
});

test('declarationTarget returns the safe location or null', () => {
  assert.deepEqual(declarationTarget(row('web')), { file: 'lib/web.ts', line: 12, column: 5 });
  assert.equal(declarationTarget(row('web', { location: null })), null);
  assert.equal(declarationTarget(row('web', { location: { file: '../x.ts', line: 1 } })), null);
});

const doc = { lineCount: 3, lineLength: (/** @type {number} */ line) => [10, 4, 0][line] };

test('navigation converts 1-based positions to 0-based editor positions', () => {
  assert.deepEqual(clampPosition({ file: 'a', line: 1, column: 1 }, doc), { line: 0, character: 0 });
  assert.deepEqual(clampPosition({ file: 'a', line: 2, column: 3 }, doc), { line: 1, character: 2 });
});

test('a line past the end of the document clamps to the last line', () => {
  assert.deepEqual(clampPosition({ file: 'a', line: 99, column: 7 }, doc), { line: 2, character: 0 });
});

test('a column past the end of the line clamps to the line length', () => {
  assert.deepEqual(clampPosition({ file: 'a', line: 2, column: 50 }, doc), { line: 1, character: 4 });
});

test('a missing column lands at the start of the line', () => {
  assert.deepEqual(clampPosition({ file: 'a', line: 2 }, doc), { line: 1, character: 0 });
});

test('an empty document clamps to its start', () => {
  assert.deepEqual(clampPosition({ file: 'a', line: 5 }, { lineCount: 0, lineLength: () => 0 }), { line: 0, character: 0 });
});

test('shiftLines moves lines after an insertion and leaves earlier ones alone', () => {
  const { shiftLines } = require('../../src/annotations');
  const items = [{ line: 3 }, { line: 10 }];
  const moved = shiftLines(items, [{ startLine: 4, endLine: 4, text: 'a\nb\n' }]);
  assert.deepEqual(moved, [{ line: 3 }, { line: 12 }]);
});

test('shiftLines drops a line inside a removed span and keeps the start line', () => {
  const { shiftLines } = require('../../src/annotations');
  const items = [{ line: 5 }, { line: 6 }, { line: 9 }];
  const moved = shiftLines(items, [{ startLine: 4, endLine: 5, text: '' }]);
  assert.deepEqual(moved, [{ line: 5 }, { line: 8 }]);
});

test('shiftLines applies several changes from the pre-edit coordinates', () => {
  const { shiftLines } = require('../../src/annotations');
  const moved = shiftLines(
    [{ line: 20 }],
    [
      { startLine: 0, endLine: 0, text: 'x\n' },
      { startLine: 9, endLine: 11, text: '' },
    ],
  );
  assert.deepEqual(moved, [{ line: 19 }]);
});
