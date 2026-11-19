'use strict';

const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

fs.writeFileSync(path.join(process.env.FIXTURE_PIDS, 'child.pid'), String(process.pid));
if (process.env.FIXTURE_MODE === 'ignore') process.on('SIGTERM', () => {});

const grandchild = path.join(__dirname, 'grandchild.js');
if (process.platform === 'win32') {
  spawn(process.execPath, [grandchild], { stdio: 'ignore', env: process.env });
} else {
  // Job control puts the background job in its own process group inside this
  // session, the way a plugin's setpgid does.
  spawn('bash', ['-c', `set -m; "${process.execPath}" "${grandchild}" & wait`], { stdio: 'ignore', env: process.env });
}

setInterval(() => {}, 1 << 30);
