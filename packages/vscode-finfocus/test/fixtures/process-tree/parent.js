'use strict';

// Stands in for finfocus: it starts a child, and a grandchild that moves to its
// own process group the way a plugin does while staying in the session.
// Modes (FIXTURE_MODE): graceful (default) stops its children on SIGTERM;
// leave exits on SIGTERM and leaves the grandchild; ignore ignores SIGTERM;
// orphan-clean-exit exits 0 on its own and leaves the grandchild behind.
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const dir = process.env.FIXTURE_PIDS;
const mode = process.env.FIXTURE_MODE || 'graceful';
const here = __dirname;

function record(role, pid) {
  fs.writeFileSync(path.join(dir, `${role}.pid`), String(pid));
}

record('parent', process.pid);

const child = spawn(process.execPath, [path.join(here, 'child.js')], { stdio: 'ignore', env: process.env });

if (mode === 'ignore') process.on('SIGTERM', () => {});
if (mode === 'graceful') {
  process.on('SIGTERM', () => {
    child.kill('SIGTERM');
    for (const role of ['grandchild']) {
      try {
        process.kill(Number(fs.readFileSync(path.join(dir, `${role}.pid`), 'utf8')), 'SIGTERM');
      } catch {
        continue;
      }
    }
    setTimeout(() => process.exit(0), 50);
  });
}
if (mode === 'leave') process.on('SIGTERM', () => process.exit(0));
if (mode === 'orphan-clean-exit') {
  const wait = setInterval(() => {
    if (fs.existsSync(path.join(dir, 'grandchild.pid'))) {
      clearInterval(wait);
      process.exit(0);
    }
  }, 20);
}

setInterval(() => {}, 1 << 30);
