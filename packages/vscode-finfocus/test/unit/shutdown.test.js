'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { createShutdown } = require('../../src/shutdown');

function windowsShutdown(taskkillResult) {
  const calls = [];
  const tree = { taskkill: async (pid) => (calls.push(pid), taskkillResult), members: async () => [], killAll() {} };
  return { shutdown: createShutdown({ tree, platform: 'win32', confirmMs: 50 }), calls };
}

test('on Windows a stop ends the tree with taskkill and confirms once the process exits', async () => {
  const { shutdown, calls } = windowsShutdown(true);
  const result = await shutdown.stop({ pid: 7, exited: new Promise((r) => setTimeout(r, 5)), terminate() {} });
  assert.deepEqual(calls, [7]);
  assert.deepEqual(result, { confirmed: true, forced: true, survivors: [] });
});

test('on Windows a process that died on its own is confirmed although taskkill finds nothing', async () => {
  const { shutdown } = windowsShutdown(false);
  const result = await shutdown.stop({ pid: 7, exited: Promise.resolve(), terminate() {} });
  assert.equal(result.confirmed, true, 'the kill-on-close job already ended the descendants');
});

test('on Windows a live process that taskkill cannot end is not confirmed', async () => {
  const { shutdown } = windowsShutdown(false);
  const result = await shutdown.stop({ pid: 7, exited: new Promise(() => {}), terminate() {} });
  assert.deepEqual(result, { confirmed: false, forced: true, survivors: [7] });
});
