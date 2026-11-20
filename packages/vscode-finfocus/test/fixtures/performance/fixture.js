'use strict';

const SOURCE_FILE = 'perf-program.js';
const RESOURCE_COUNT = 300;

/** The program text: one declaration per line, so row N is declared on line N + 2. */
function buildSource() {
  const lines = ['const { Box } = require("./box");'];
  for (let i = 0; i < RESOURCE_COUNT; i += 1) lines.push(`new Box("r${String(i).padStart(3, '0')}");`);
  return `${lines.join('\n')}\n`;
}

/** Deterministic rows: the same names, positions, and display strings on every run. */
function buildRows() {
  const rows = [];
  for (let i = 0; i < RESOURCE_COUNT; i += 1) {
    rows.push({
      name: `r${String(i).padStart(3, '0')}`,
      location: { file: SOURCE_FILE, line: i + 2, column: 1 },
      modifiedAt: '2099-01-01T00:00:00Z',
      projected: `$${(10 + (i % 37) * 3.25).toFixed(2)}`,
    });
  }
  return rows;
}

module.exports = { SOURCE_FILE, RESOURCE_COUNT, buildSource, buildRows };
