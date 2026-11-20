const { defineConfig } = require('@vscode/test-cli');

const files = 'test/editor/**/*.test.js';
const mocha = { ui: 'tdd', timeout: 30000 };

module.exports = defineConfig([
  {
    label: 'minimum-api',
    version: '1.90.0',
    files,
    workspaceFolder: './test/fixtures/workspaces/single',
    mocha,
  },
  {
    label: 'stable',
    version: 'stable',
    files,
    workspaceFolder: './test/fixtures/workspaces/single',
    mocha,
  },
]);
