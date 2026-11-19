'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {
  ProjectError,
  parseStackList,
  discoverProjects,
  nearestProject,
  resolveProject,
  chooseStack,
  contextKey,
  stackSettingsFiles,
  isStackSettingsFile,
  Selection,
} = require('../../src/projects');

const fixture = (name) => fs.readFileSync(path.join(__dirname, '..', 'fixtures', 'projects', name), 'utf8');

test('parses the recorded local stack list with its current flag', () => {
  assert.deepEqual(parseStackList(fixture('stack-ls-local.json')), [
    { name: 'dev', current: false, lastUpdate: '2026-11-17T13:09:31.357Z', resourceCount: 0 },
    { name: 'prod', current: true, lastUpdate: '2026-11-17T13:09:31.593Z', resourceCount: 0 },
  ]);
});

test('parses qualified cloud stack references', () => {
  const stacks = parseStackList(fixture('stack-ls-cloud.json'));
  assert.deepEqual(
    stacks.map((s) => s.name),
    ['acme/shop/dev', 'acme/shop/prod'],
  );
  assert.equal(stacks[0].current, true);
});

test('keeps only the cache change signals from a stack list, and only with the right types', () => {
  const [dev, prod] = parseStackList(fixture('stack-ls-cloud.json'));
  assert.deepEqual(dev, {
    name: 'acme/shop/dev',
    current: true,
    lastUpdate: '2026-11-16T10:00:00.000Z',
    resourceCount: 61,
    updateInProgress: false,
  });
  assert.equal(prod.resourceCount, 80);
  const odd = parseStackList(
    JSON.stringify([{ name: 'a', lastUpdate: 5, resourceCount: '3', updateInProgress: 'yes', url: 'u', extra: 1 }]),
  );
  assert.deepEqual(odd, [{ name: 'a', current: false }]);
});

test('a stack never updated has no change signals', () => {
  assert.deepEqual(parseStackList('[{"name":"prod","current":false,"url":"u"}]'), [{ name: 'prod', current: false }]);
});

test('an empty stack list is valid and empty', () => {
  assert.deepEqual(parseStackList('[]'), []);
});

test('malformed stack list output is a typed error', () => {
  for (const text of ['', 'not json', '{}', '[{"current":true}]', '[{"name":""}]', '[1]']) {
    assert.throws(
      () => parseStackList(text),
      (err) => err instanceof ProjectError && err.code === 'malformed_stack_list',
      JSON.stringify(text),
    );
  }
});

const folder = 'file:///work/repo';

test('discovers Pulumi.yaml and Pulumi.yml project directories', () => {
  const found = discoverProjects(
    [
      `${folder}/infra/Pulumi.yaml`,
      `${folder}/apps/api/Pulumi.yml`,
      `${folder}/README.md`,
      `${folder}/infra/Pulumi.dev.yaml`,
    ],
    [folder],
  );
  assert.deepEqual(found, [`${folder}/apps/api`, `${folder}/infra`]);
});

test('discovery excludes dependency, VCS, and build directories below the workspace folder', () => {
  const found = discoverProjects(
    [
      `${folder}/node_modules/pkg/Pulumi.yaml`,
      `${folder}/.git/x/Pulumi.yaml`,
      `${folder}/vendor/mod/Pulumi.yaml`,
      `${folder}/dist/Pulumi.yaml`,
      `${folder}/Pulumi.yaml`,
    ],
    [folder],
  );
  assert.deepEqual(found, [folder]);
});

test('a workspace folder that itself sits under a build directory is not excluded', () => {
  const inBuild = 'file:///build/repo';
  assert.deepEqual(discoverProjects([`${inBuild}/Pulumi.yaml`], [inBuild]), [inBuild]);
});

test('discovery ignores project files outside every workspace folder', () => {
  assert.deepEqual(discoverProjects(['file:///elsewhere/Pulumi.yaml'], [folder]), []);
});

test('discovery de-duplicates a directory holding both Pulumi.yaml and Pulumi.yml', () => {
  assert.deepEqual(discoverProjects([`${folder}/a/Pulumi.yaml`, `${folder}/a/Pulumi.yml`], [folder]), [`${folder}/a`]);
});

test('the nearest ancestor project of a file wins, so nested projects win', () => {
  const projects = [folder, `${folder}/infra`, `${folder}/infra/edge`];
  assert.equal(nearestProject(projects, `${folder}/infra/edge/index.ts`), `${folder}/infra/edge`);
  assert.equal(nearestProject(projects, `${folder}/infra/main.ts`), `${folder}/infra`);
  assert.equal(nearestProject(projects, `${folder}/docs/a.md`), folder);
});

test('a sibling directory with a shared name prefix is not an ancestor', () => {
  assert.equal(nearestProject([`${folder}/infra`], `${folder}/infra-old/main.ts`), null);
});

test('a project is its own ancestor for files directly inside it', () => {
  assert.equal(nearestProject([`${folder}/infra`], `${folder}/infra/Pulumi.yaml`), `${folder}/infra`);
});

test('remote authorities do not match each other', () => {
  const wsl = 'vscode-remote://wsl%2Bubuntu/work/repo';
  const ssh = 'vscode-remote://ssh-remote%2Bbox/work/repo';
  assert.equal(nearestProject([wsl], `${ssh}/a.ts`), null);
  assert.equal(nearestProject([wsl], `${wsl}/a.ts`), wsl);
});

test('resolveProject picks the active file project, ahead of an explicit choice', () => {
  const projects = [`${folder}/a`, `${folder}/b`];
  assert.deepEqual(resolveProject({ projects, activeFile: `${folder}/a/x.ts`, explicit: `${folder}/b` }), {
    kind: 'selected',
    project: `${folder}/a`,
  });
});

test('with no containing active file, the explicit choice is retained', () => {
  const projects = [`${folder}/a`, `${folder}/b`];
  assert.deepEqual(resolveProject({ projects, activeFile: `${folder}/docs/x.md`, explicit: `${folder}/b` }), {
    kind: 'selected',
    project: `${folder}/b`,
  });
  assert.deepEqual(resolveProject({ projects, activeFile: null, explicit: `${folder}/b` }), {
    kind: 'selected',
    project: `${folder}/b`,
  });
});

test('an explicit choice that is no longer discovered is ignored', () => {
  assert.deepEqual(resolveProject({ projects: [`${folder}/a`], activeFile: null, explicit: `${folder}/gone` }), {
    kind: 'selected',
    project: `${folder}/a`,
  });
});

test('a sole project is selected, several ask for a picker, none stays silent', () => {
  assert.deepEqual(resolveProject({ projects: [`${folder}/a`], activeFile: null, explicit: null }), {
    kind: 'selected',
    project: `${folder}/a`,
  });
  assert.deepEqual(resolveProject({ projects: [`${folder}/a`, `${folder}/b`], activeFile: null, explicit: null }), {
    kind: 'pick',
    candidates: [`${folder}/a`, `${folder}/b`],
  });
  assert.deepEqual(resolveProject({ projects: [], activeFile: null, explicit: null }), { kind: 'none' });
});

test('chooseStack defaults to the current stack', () => {
  const stacks = parseStackList(fixture('stack-ls-local.json'));
  assert.deepEqual(chooseStack(stacks, null), { kind: 'selected', stack: 'prod' });
});

test('an explicit stack beats the current flag while it still exists', () => {
  const stacks = parseStackList(fixture('stack-ls-local.json'));
  assert.deepEqual(chooseStack(stacks, 'dev'), { kind: 'selected', stack: 'dev' });
  assert.deepEqual(chooseStack(stacks, 'gone'), { kind: 'selected', stack: 'prod' });
});

test('with no current stack the user picks, and with none there is nothing to choose', () => {
  const noCurrent = [
    { name: 'a', current: false },
    { name: 'b', current: false },
  ];
  assert.deepEqual(chooseStack(noCurrent, null), { kind: 'pick', stacks: ['a', 'b'] });
  assert.deepEqual(chooseStack([], null), { kind: 'none' });
});

test('same-name dev stacks in different projects have different context keys', () => {
  const a = contextKey(`${folder}/a`, 'dev');
  const b = contextKey(`${folder}/b`, 'dev');
  assert.notEqual(a, b);
  assert.notEqual(contextKey(`${folder}/a`, 'dev'), contextKey(`${folder}/a`, 'acme/a/dev'));
});

test('context keys include the remote authority', () => {
  assert.notEqual(
    contextKey('vscode-remote://wsl%2Bubuntu/work/a', 'dev'),
    contextKey('vscode-remote://ssh-remote%2Bbox/work/a', 'dev'),
  );
});

test('context keys cannot collide through separator characters in names', () => {
  assert.notEqual(contextKey('a', 'b|c'), contextKey('a|b', 'c'));
  assert.notEqual(contextKey('a', 'b::c'), contextKey('a::b', 'c'));
});

test('stack settings candidates use the short name and never interpolate qualified names', () => {
  assert.deepEqual(stackSettingsFiles('dev'), ['Pulumi.dev.yaml', 'Pulumi.dev.yml']);
  assert.deepEqual(stackSettingsFiles('acme/shop/dev'), ['Pulumi.dev.yaml', 'Pulumi.dev.yml']);
});

test('unsafe stack names produce no candidate file', () => {
  for (const name of ['', '..', 'acme/shop/..', 'a\\..\\b', 'dev/', 'a b', '../x']) {
    assert.deepEqual(stackSettingsFiles(name), [], JSON.stringify(name));
  }
});

test('recognizes stack settings files that invalidate rows', () => {
  assert.equal(isStackSettingsFile('Pulumi.dev.yaml', 'dev'), true);
  assert.equal(isStackSettingsFile('Pulumi.dev.yml', 'acme/shop/dev'), true);
  assert.equal(isStackSettingsFile('Pulumi.prod.yaml', 'dev'), false);
  assert.equal(isStackSettingsFile('Pulumi.yaml', 'dev'), false);
});

test('Selection starts empty at epoch zero', () => {
  const selection = new Selection();
  assert.equal(selection.epoch, 0);
  assert.equal(selection.project, null);
  assert.equal(selection.stackFor(`${folder}/a`), null);
});

test('changing the project or the stack increments the epoch and invalidates the old one', () => {
  const selection = new Selection();
  const first = selection.selectProject(`${folder}/a`);
  assert.equal(selection.isCurrent(first), true);
  const second = selection.selectStack(`${folder}/a`, 'dev');
  assert.ok(second > first);
  assert.equal(selection.isCurrent(first), false);
  assert.equal(selection.isCurrent(second), true);
  assert.equal(selection.stackFor(`${folder}/a`), 'dev');
});

test('re-selecting the same context keeps the epoch', () => {
  const selection = new Selection();
  selection.selectProject(`${folder}/a`);
  const epoch = selection.selectStack(`${folder}/a`, 'dev');
  assert.equal(selection.selectStack(`${folder}/a`, 'dev'), epoch);
  assert.equal(selection.selectProject(`${folder}/a`), epoch);
});

test('stack choices are remembered per project for the session', () => {
  const selection = new Selection();
  selection.selectProject(`${folder}/a`);
  selection.selectStack(`${folder}/a`, 'dev');
  selection.selectProject(`${folder}/b`);
  selection.selectStack(`${folder}/b`, 'prod');
  assert.equal(selection.stackFor(`${folder}/a`), 'dev');
  assert.equal(selection.stackFor(`${folder}/b`), 'prod');
});

test('invalidating a project drops its stack choice and bumps the epoch', () => {
  const selection = new Selection();
  selection.selectProject(`${folder}/a`);
  const before = selection.selectStack(`${folder}/a`, 'dev');
  const after = selection.invalidateProject(`${folder}/a`);
  assert.ok(after > before);
  assert.equal(selection.stackFor(`${folder}/a`), null);
  assert.equal(selection.project, null);
});

test('invalidating an unrelated project leaves the active selection alone', () => {
  const selection = new Selection();
  selection.selectProject(`${folder}/a`);
  const epoch = selection.selectStack(`${folder}/a`, 'dev');
  assert.equal(selection.invalidateProject(`${folder}/b`), epoch);
  assert.equal(selection.stackFor(`${folder}/a`), 'dev');
});

test('forgetting a stack that disappeared keeps the project but clears the choice', () => {
  const selection = new Selection();
  selection.selectProject(`${folder}/a`);
  selection.selectStack(`${folder}/a`, 'dev');
  const epoch = selection.forgetStack(`${folder}/a`);
  assert.equal(selection.stackFor(`${folder}/a`), null);
  assert.equal(selection.project, `${folder}/a`);
  assert.equal(selection.isCurrent(epoch), true);
});
