'use strict';

const fs = require('node:fs');
const path = require('node:path');

fs.writeFileSync(path.join(process.env.FIXTURE_PIDS, 'grandchild.pid'), String(process.pid));
if (process.env.FIXTURE_MODE === 'ignore') process.on('SIGTERM', () => {});
setInterval(() => {}, 1 << 30);
