'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const inventory = require('../../contract/overview-fields.json');
const { ResultError, parseOverview, parseFailure } = require('../../src/result');

function fixture() {
  return {
    metadata: { stackName: 'dev', generatedAt: '2026-11-16T10:00:00Z', isStateOnly: true },
    resources: [
      {
        urn: 'urn:pulumi:dev::shop::aws:ec2/instance:Instance::web',
        type: 'aws:ec2/instance:Instance',
        status: 'active',
        projectedCost: { monthlyCost: 61.32, currency: 'USD' },
        modifiedAt: '2026-11-16T09:00:00Z',
        location: { file: 'lib/web.ts', line: 12, column: 5 },
        display: { projected: '$61.32', actual: '$18.40', delta: '+$30.66', status: 'active' },
      },
      {
        urn: 'urn:pulumi:dev::shop::aws:s3/bucket:Bucket::logs',
        type: 'aws:s3/bucket:Bucket',
        status: 'creating',
        projectedCost: { monthlyCost: 0, currency: 'USD' },
        display: { projected: '$0.00', actual: '-', delta: '-', status: '+ creating' },
      },
    ],
    summary: { currency: 'USD', display: { projected: '$61.32', actual: '$18.40', delta: '+$30.66' } },
    budgetDisplay: { healthDisplay: '', amountsDisplay: '', footerDisplay: '', details: [] },
    errors: [{ urn: 'urn:pulumi:dev::shop::aws:rds/instance:Instance::db', message: 'no price' }],
  };
}

function parse(doc) {
  return parseOverview(JSON.stringify(doc));
}

function walk(doc, path) {
  const segments = path.split('.');
  const last = segments.pop().replace('[]', '');
  let nodes = [doc];
  for (const segment of segments) {
    const key = segment.replace('[]', '');
    nodes = nodes
      .filter((node) => node && typeof node === 'object' && key in node)
      .flatMap((node) => (segment.endsWith('[]') ? node[key] : [node[key]]));
  }
  return { nodes: nodes.filter((node) => node && typeof node === 'object'), last };
}

function remove(doc, path) {
  const { nodes, last } = walk(doc, path);
  for (const node of nodes) delete node[last];
}

function setKind(doc, path, value) {
  const { nodes, last } = walk(doc, path);
  for (const node of nodes) if (last in node) node[last] = value;
}

test('maps the contract fields to an OverviewResult', () => {
  const result = parse(fixture());
  assert.equal(result.stack, 'dev');
  assert.equal(result.generatedAt, '2026-11-16T10:00:00Z');
  assert.equal(result.stateOnly, true);
  assert.equal(result.currency, 'USD');
  assert.equal(result.rows.length, 2);
  assert.deepEqual(result.rows[0].location, { file: 'lib/web.ts', line: 12, column: 5 });
  assert.equal(result.rows[0].modifiedAt, '2026-11-16T09:00:00Z');
  assert.equal(result.rows[0].display.projected, '$61.32');
  assert.equal(result.summaryDisplay.delta, '+$30.66');
  assert.deepEqual(result.errors, [
    { urn: 'urn:pulumi:dev::shop::aws:rds/instance:Instance::db', message: 'no price' },
  ]);
});

test('a preview result omits isStateOnly, which decodes as false', () => {
  const doc = fixture();
  delete doc.metadata.isStateOnly;
  assert.equal(parse(doc).stateOnly, false);
});

test('optional row fields may be absent', () => {
  const row = parse(fixture()).rows[1];
  assert.equal(row.location, null);
  assert.equal(row.modifiedAt, null);
});

test('a numeric zero stays distinct from missing data', () => {
  const doc = fixture();
  delete doc.resources[0].projectedCost;
  const rows = parse(doc).rows;
  assert.equal(rows[0].monthlyCost, null);
  assert.equal(rows[1].monthlyCost, 0);
});

test('a location without a column keeps no column', () => {
  const doc = fixture();
  doc.resources[0].location = { file: 'main.go', line: 30 };
  assert.deepEqual(parse(doc).rows[0].location, { file: 'main.go', line: 30 });
});

test('an unusable location is dropped rather than shown', () => {
  for (const location of [{ file: '', line: 3 }, { file: 'a.ts', line: 0 }, { file: 'a.ts', line: 1.5 }]) {
    const doc = fixture();
    doc.resources[0].location = location;
    assert.equal(parse(doc).rows[0].location, null, JSON.stringify(location));
  }
});

test('display strings pass through unchanged', () => {
  const doc = fixture();
  doc.resources[0].display = { projected: '  $1,0.5 ', actual: 'ERR', delta: '-', status: '~ updating' };
  assert.deepEqual(parse(doc).rows[0].display, doc.resources[0].display);
});

test('malformed JSON is a typed error', () => {
  assert.throws(
    () => parseOverview('{not json'),
    (err) => err instanceof ResultError && err.code === 'malformed_json',
  );
  assert.throws(() => parseOverview(''), ResultError);
});

for (const path of Object.keys(inventory.required)) {
  test(`required path ${path} missing is a schema error`, () => {
    const doc = fixture();
    remove(doc, path);
    assert.throws(
      () => parse(doc),
      (err) => err instanceof ResultError && err.code === 'schema',
    );
  });

  test(`required path ${path} with the wrong type is a schema error`, () => {
    const doc = fixture();
    setKind(doc, path, inventory.required[path] === 'string' ? 7 : 'wrong');
    assert.throws(() => parse(doc), ResultError);
  });
}

for (const path of Object.keys(inventory.optional)) {
  test(`optional path ${path} with the wrong type is a schema error`, () => {
    const doc = fixture();
    setKind(doc, path, inventory.optional[path] === 'string' ? 7 : 'wrong');
    assert.throws(() => parse(doc), ResultError);
  });

  test(`optional path ${path} may be absent`, () => {
    const doc = fixture();
    remove(doc, path);
    assert.doesNotThrow(() => parse(doc));
  });
}

test('the inventory is read in full: an extra required path breaks parsing', () => {
  const custom = { required: { ...inventory.required, 'metadata.newField': 'string' }, optional: {} };
  assert.throws(() => parseOverview(JSON.stringify(fixture()), custom), /metadata\.newField/);
});

test('a passphrase failure envelope exposes its category', () => {
  const failure = parseFailure(
    JSON.stringify({ error_code: 'INTERNAL', message: 'loading stack: passphrase required (passphrase_required)', version: '0.5.8' }),
  );
  assert.equal(failure.category, 'passphrase_required');
  assert.equal(failure.version, '0.5.8');
  assert.match(failure.message, /passphrase required/);
});

test('the mixed-currency error is a plain envelope with a category', () => {
  const failure = parseFailure(
    JSON.stringify({
      error_code: 'INTERNAL',
      message: 'mixed currencies not supported in cross-provider aggregation (internal)',
      version: '0.5.8',
    }),
  );
  assert.equal(failure.category, 'internal');
  assert.match(failure.message, /mixed currencies/);
});

test('an envelope with no trailing category has an empty one', () => {
  assert.equal(parseFailure(JSON.stringify({ message: 'boom' })).category, '');
});

test('stderr without an envelope parses to null', () => {
  assert.equal(parseFailure('panic: something\n'), null);
  assert.equal(parseFailure(''), null);
  assert.equal(parseFailure('{"foo":1}'), null);
});

test('the unpriced placeholder is pinned: its path is a required string field of the inventory', () => {
  const { path, value, exemptStatuses } = inventory.placeholders.unpriced;
  assert.equal(inventory.required[path], 'string');
  assert.equal(value, '-');
  assert.deepEqual(exemptStatuses, ['deleting']);
});

// ---- the real row shape (ISS-002) ----

const realShape = () => JSON.parse(require('node:fs').readFileSync(require('node:path').join(__dirname, '..', 'fixtures', 'real', 'overview-real-shape.json'), 'utf8'));

test('cost comes from projectedCost.monthlyCost, the shape finfocus emits (ISS-002)', () => {
  const rows = parse(realShape()).rows;
  assert.deepEqual(rows.map((r) => r.monthlyCost), [33.367999999999995, 32.85, 32.85, null]);
  assert.equal(realShape().resources.some((r) => 'monthlyCost' in r), false, 'no real row has a top-level monthlyCost');
});

test('a top-level monthlyCost is not read, so a stale fixture shape cannot pass', () => {
  const doc = fixture();
  doc.resources[0].monthlyCost = 61.32;
  delete doc.resources[0].projectedCost;
  assert.equal(parse(doc).rows[0].monthlyCost, null);
});

test('a retyped projectedCost.monthlyCost is a schema error, not a silent null', () => {
  const doc = realShape();
  doc.resources[0].projectedCost = { monthlyCost: '33.37', currency: 'USD' };
  assert.throws(() => parse(doc), (err) => err.name === 'ResultError' && err.code === 'schema' && /projectedCost\.monthlyCost/.test(err.message));
});

test('the inventory pins the nested cost as optional', () => {
  assert.equal(inventory.optional['resources[].projectedCost'], 'object');
  assert.equal(inventory.optional['resources[].projectedCost.monthlyCost'], 'number');
  assert.equal(inventory.required['resources[].projectedCost.monthlyCost'], undefined);
});
