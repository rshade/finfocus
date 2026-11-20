'use strict';

/**
 * @typedef {{kind: 'ok'} | {kind: 'unknown', output: string} | {kind: 'too_old', found: string, required: string}} VersionCheck
 */

/**
 * @param {string} text
 * @returns {[number, number, number] | null}
 */
function parseSemver(text) {
  const match = /v?(\d+)\.(\d+)\.(\d+)/.exec(text);
  return match === null ? null : [Number(match[1]), Number(match[2]), Number(match[3])];
}

/**
 * Compares the output of `finfocus --version` with the manifest's minimum. A
 * development or unparsable version is allowed, and reported so the caller can
 * log a warning; only a version known to be older blocks runs. A build suffix
 * such as `-10-gabc123` is ignored.
 *
 * @param {string} output
 * @param {string} required
 * @returns {VersionCheck}
 */
function checkVersion(output, required) {
  const found = parseSemver(output);
  const minimum = parseSemver(required);
  if (found === null || minimum === null) return { kind: 'unknown', output: output.trim() };
  for (let i = 0; i < 3; i += 1) {
    if (found[i] > minimum[i]) return { kind: 'ok' };
    if (found[i] < minimum[i]) return { kind: 'too_old', found: found.join('.'), required: minimum.join('.') };
  }
  return { kind: 'ok' };
}

module.exports = { checkVersion, parseSemver };
