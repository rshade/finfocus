'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { buildDashboard } = require('../../src/dashboard');
const { REASONS } = require('../../src/stale');

function row(name, overrides = {}) {
  return {
    urn: `urn:pulumi:dev::shop::aws:ec2/instance:Instance::${name}`,
    type: 'aws:ec2/instance:Instance',
    status: 'active',
    monthlyCost: 10,
    location: { file: 'lib/web.ts', line: 12, column: 5 },
    modifiedAt: null,
    display: { projected: '$10.00', actual: '$3.00', delta: '+$1.00', status: 'active' },
    ...overrides,
  };
}

function result(rows, overrides = {}) {
  return {
    stack: 'dev',
    generatedAt: '2026-11-17T00:00:00Z',
    stateOnly: true,
    currency: 'USD',
    rows,
    summaryDisplay: { projected: '$99.00', actual: '$40.00', delta: '+$5.00' },
    budgetDisplay: { healthDisplay: 'WARNING', amountsDisplay: '$80.00 of $100.00 (80%)', footerDisplay: 'Monthly budget', details: [] },
    errors: [],
    ...overrides,
  };
}

test('the card carries the stack and the CLI summary and budget strings untouched', () => {
  const view = buildDashboard({ result: result([row('web')]), stale: new Map() });
  assert.equal(view.stack, 'dev');
  assert.deepEqual(view.summary, { projected: '$99.00', actual: '$40.00', delta: '+$5.00' });
  assert.deepEqual(view.budget, {
    configured: true,
    health: 'WARNING',
    amounts: '$80.00 of $100.00 (80%)',
    footer: 'Monthly budget',
    text: 'Budget WARNING: $80.00 of $100.00 (80%) · Monthly budget',
  });
});

test('no configured budget is said in words', () => {
  const none = result([row('web')], { budgetDisplay: { healthDisplay: '', amountsDisplay: '', footerDisplay: '', details: [] } });
  const view = buildDashboard({ result: none, stale: new Map() });
  assert.equal(view.budget.configured, false);
  assert.equal(view.budget.text, 'No budget configured');
});

test('the top five are ordered by numeric cost, not by display text, and are never summed', () => {
  const costs = [9, 100, 20, 1000, 3, 50, 7];
  const rows = costs.map((c, i) => row(`r${i}`, { monthlyCost: c, display: { projected: `$${c}.00`, actual: '-', delta: '-', status: 'active' } }));
  const view = buildDashboard({ result: result(rows), stale: new Map() });
  assert.deepEqual(
    view.topFive.map((r) => r.projected),
    ['$1000.00', '$100.00', '$50.00', '$20.00', '$9.00'],
  );
  assert.equal(view.topFive.length, 5);
});

test('equal costs keep a stable order by URN', () => {
  const rows = [row('b', { monthlyCost: 5 }), row('a', { monthlyCost: 5 })];
  const view = buildDashboard({ result: result(rows), stale: new Map() });
  assert.deepEqual(
    view.topFive.map((r) => r.name),
    ['a', 'b'],
  );
});

test('unpriced rows never reach the top five', () => {
  const rows = [row('free', { monthlyCost: null, display: { projected: '-', actual: '-', delta: '-', status: 'active' } }), row('web')];
  const view = buildDashboard({ result: result(rows), stale: new Map() });
  assert.deepEqual(
    view.topFive.map((r) => r.name),
    ['web'],
  );
});

test('every resource is listed once, grouped by type with an unpriced group last', () => {
  const rows = [
    row('db', { type: 'aws:rds/instance:Instance' }),
    row('web'),
    row('api'),
    row('queue', { type: 'aws:sqs/queue:Queue', monthlyCost: null, display: { projected: '-', actual: '-', delta: '-', status: 'active' } }),
    row('broken', { monthlyCost: null, display: { projected: 'ERR', actual: '-', delta: '-', status: 'active' } }),
  ];
  const view = buildDashboard({ result: result(rows), stale: new Map() });
  assert.deepEqual(
    view.groups.map((g) => [g.label, g.unpriced, g.items.map((i) => i.name)]),
    [
      ['aws:ec2/instance:Instance', false, ['api', 'web']],
      ['aws:rds/instance:Instance', false, ['db']],
      ['Unpriced', true, ['broken', 'queue']],
    ],
  );
  assert.equal(view.total, 5);
});

test('a row error is attached to its resource and kept in the full error list', () => {
  const failing = row('web', { monthlyCost: null, display: { projected: 'ERR', actual: '-', delta: '-', status: 'active' } });
  const errors = [
    { urn: failing.urn, message: 'plugin said no' },
    { urn: 'urn:pulumi:dev::shop::aws:s3/bucket:Bucket::gone', message: 'not in the table' },
  ];
  const view = buildDashboard({ result: result([failing], { errors }), stale: new Map() });
  assert.equal(view.groups.find((g) => g.unpriced).items[0].error, 'plugin said no');
  assert.deepEqual(view.errors, errors);
});

test('stale and unknown resources are counted separately with the reasons given', () => {
  const rows = [row('a'), row('b'), row('c', { location: null })];
  const stale = new Map([
    [rows[0].urn, [REASONS.edited]],
    [rows[1].urn, [REASONS.unsaved, REASONS.config]],
    [rows[2].urn, [REASONS.unknown]],
  ]);
  const view = buildDashboard({ result: result(rows), stale });
  assert.deepEqual(view.freshness, {
    stale: 2,
    unknown: 1,
    text: '2 stale · 1 freshness unknown',
  });
  const items = view.groups.flatMap((g) => g.items);
  assert.deepEqual(items.find((i) => i.name === 'b').reasons, [REASONS.unsaved, REASONS.config]);
});

test('nothing stale reads as such', () => {
  const view = buildDashboard({ result: result([row('a')]), stale: new Map() });
  assert.deepEqual(view.freshness, { stale: 0, unknown: 0, text: 'No stale resources' });
});

test('resource details expose every hover field, the location, the stale reason and the error', () => {
  const failing = row('web', { location: { file: 'lib/web.ts', line: 12 } });
  const view = buildDashboard({
    result: result([failing], { errors: [{ urn: failing.urn, message: 'boom' }] }),
    stale: new Map([[failing.urn, [REASONS.edited]]]),
  });
  const [item] = view.groups[0].items.length ? view.groups[0].items : view.groups[1].items;
  assert.deepEqual(item.detail, [
    'web',
    'aws:ec2/instance:Instance',
    'Projected $10.00 · Actual $3.00 · Delta +$1.00 · active',
    'Declared at lib/web.ts:12',
    `Stale: ${REASONS.edited}`,
    'Error: boom',
  ]);
});

test('a resource with no safe location says so and cannot be opened', () => {
  const unsafe = row('web', { location: { file: '../outside.ts', line: 1 } });
  const bare = row('db', { location: null });
  const view = buildDashboard({ result: result([unsafe, bare]), stale: new Map() });
  for (const item of view.groups.flatMap((g) => g.items)) {
    assert.equal(item.openable, false);
    assert.ok(item.detail.includes('No recorded location'));
  }
});

test('an empty stack still produces a card with no groups', () => {
  const view = buildDashboard({ result: result([]), stale: new Map() });
  assert.deepEqual(view.groups, []);
  assert.deepEqual(view.topFive, []);
  assert.equal(view.total, 0);
});

// ---- completeness marker (FR-012b) ----

const incomplete = { kind: 'incomplete', gaps: ['a', 'b', 'c', 'd', 'e'], total: 6 };

test('no completeness, or a complete one, adds no marker text', () => {
  for (const completeness of [undefined, null, { kind: 'complete', gaps: [], total: 1 }]) {
    const view = buildDashboard({ result: result([row('web')]), stale: new Map(), completeness });
    assert.deepEqual(view.completeness, { kind: 'complete', text: '', help: '' });
  }
});

test('an incomplete result says how many of how many resources are unpriced, in words, and explains', () => {
  const view = buildDashboard({ result: result([row('web')]), stale: new Map(), completeness: incomplete });
  assert.equal(view.completeness.kind, 'incomplete');
  assert.equal(view.completeness.text, 'Incomplete: 5 of 6 resources unpriced');
  assert.match(view.completeness.help, /total may be too low/i);
  assert.match(view.completeness.help, /Refresh costs/);
});

test('one unpriced resource reads in the singular', () => {
  const view = buildDashboard({
    result: result([row('web')]),
    stale: new Map(),
    completeness: { kind: 'incomplete', gaps: ['a'], total: 1 },
  });
  assert.equal(view.completeness.text, 'Incomplete: 1 of 1 resource unpriced');
});

test('confirmed gaps are a note, not an incomplete marker', () => {
  const one = buildDashboard({ result: result([row('web')]), stale: new Map(), completeness: { kind: 'confirmed', gaps: ['a'], total: 4 } });
  assert.deepEqual(one.completeness, { kind: 'confirmed', text: '1 resource unpriced', help: '' });
  const two = buildDashboard({ result: result([row('web')]), stale: new Map(), completeness: { kind: 'confirmed', gaps: ['a', 'b'], total: 4 } });
  assert.equal(two.completeness.text, '2 resources unpriced');
});

test('the marker never alters the CLI summary strings', () => {
  const view = buildDashboard({ result: result([row('web')]), stale: new Map(), completeness: incomplete });
  assert.deepEqual(view.summary, { projected: '$99.00', actual: '$40.00', delta: '+$5.00' });
});

test('a real finfocus result fills the top five, ordered by the cost it reports (ISS-002)', () => {
  const real = require('../../src/result').parseOverview(
    require('node:fs').readFileSync(require('node:path').join(__dirname, '..', 'fixtures', 'real', 'overview-real-shape.json'), 'utf8'),
  );
  const view = buildDashboard({ result: real, stale: new Map() });
  assert.deepEqual(view.topFive.map((i) => [i.name, i.projected]), [
    ['windowsClientInstance', '$33.37'],
    ['clientNatGateway', '$32.85'],
    ['webNatGateway', '$32.85'],
  ]);
});
