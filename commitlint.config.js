module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // The conventional types plus deps, which release-please renders under a
    // Dependencies heading (release-please-config.json).
    'type-enum': [
      2,
      'always',
      ['build', 'chore', 'ci', 'deps', 'docs', 'feat', 'fix', 'perf', 'refactor', 'revert', 'style', 'test'],
    ],
  },
};
