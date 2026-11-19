'use strict';

/**
 * Pure projection from overview rows to per-line annotations. It formats no
 * money: every figure is a display string finfocus produced.
 *
 * @typedef {import('./result').OverviewRow} OverviewRow
 * @typedef {import('./result').Location} Location
 * @typedef {{
 *   file: string, line: number, text: string, hover: string[],
 *   count: number, urns: string[], unpriced: boolean
 * }} Annotation
 */

const UNPRICED_MARKERS = new Set(['-', 'ERR']);

/**
 * A project-relative, forward-slash path with no escape from the project.
 *
 * @param {string} file
 * @returns {boolean}
 */
function isProjectRelative(file) {
  if (file === '' || file.startsWith('/') || file.includes('\\') || /^[A-Za-z]:/.test(file)) return false;
  return !file.split('/').includes('..');
}

/**
 * @param {string} urn
 * @returns {string}
 */
function resourceName(urn) {
  const cut = urn.lastIndexOf('::');
  return cut < 0 ? urn : urn.slice(cut + 2);
}

/**
 * @param {OverviewRow} row
 * @returns {string[]}
 */
function hoverLines(row) {
  const { projected, actual, delta, status } = row.display;
  const parts = [`Projected ${projected}`];
  if (actual !== '-') parts.push(`Actual ${actual}`);
  if (delta !== '-') parts.push(`Delta ${delta}`);
  parts.push(status);
  return [resourceName(row.urn), parts.join(' · ')];
}

/**
 * The declaration to open for a row, or null when it has none or it is unsafe.
 *
 * @param {OverviewRow} row
 * @returns {Location | null}
 */
function declarationTarget(row) {
  const location = row.location;
  if (location === null || !isProjectRelative(location.file)) return null;
  return location;
}

/**
 * Groups rows by declaring file and line. Several resources on one line show a
 * count and list each figure in the hover; they are never summed.
 *
 * @param {OverviewRow[]} rows
 * @returns {Map<string, Annotation[]>}
 */
function projectAnnotations(rows) {
  /** @type {Map<string, Map<number, OverviewRow[]>>} */
  const grouped = new Map();
  for (const row of rows) {
    const target = declarationTarget(row);
    if (target === null) continue;
    const lines = grouped.get(target.file) ?? new Map();
    lines.set(target.line, [...(lines.get(target.line) ?? []), row]);
    grouped.set(target.file, lines);
  }

  /** @type {Map<string, Annotation[]>} */
  const result = new Map();
  for (const [file, lines] of grouped) {
    const annotations = [...lines.entries()]
      .sort(([a], [b]) => a - b)
      .map(([line, onLine]) => {
        const sorted = [...onLine].sort((a, b) => (a.urn < b.urn ? -1 : a.urn > b.urn ? 1 : 0));
        return /** @type {Annotation} */ ({
          file,
          line,
          text: sorted.length === 1 ? sorted[0].display.projected : `${sorted.length} resources`,
          hover: sorted.flatMap(hoverLines),
          count: sorted.length,
          urns: sorted.map((r) => r.urn),
          unpriced: sorted.every((r) => UNPRICED_MARKERS.has(r.display.projected)),
        });
      });
    result.set(file, annotations);
  }
  return result;
}

/**
 * Converts a 1-based location to a 0-based editor position inside the
 * document, so a stale line number can never point past the end.
 *
 * @param {Location} location
 * @param {{lineCount: number, lineLength: (line: number) => number}} doc
 * @returns {{line: number, character: number}}
 */
function clampPosition(location, doc) {
  if (doc.lineCount <= 0) return { line: 0, character: 0 };
  const line = Math.min(location.line - 1, doc.lineCount - 1);
  const character = Math.min((location.column ?? 1) - 1, doc.lineLength(line));
  return { line, character: Math.max(character, 0) };
}

/**
 * Moves 1-based line numbers through one document edit. Changes use the
 * pre-edit coordinates VS Code reports. A line after the changed span shifts
 * by the number of lines the edit added or removed, a line inside a removed
 * span is dropped, and the start line of a change keeps its annotation.
 *
 * @template {{line: number}} T
 * @param {T[]} items
 * @param {{startLine: number, endLine: number, text: string}[]} changes 0-based pre-edit lines
 * @returns {T[]}
 */
function shiftLines(items, changes) {
  /** @type {T[]} */
  const kept = [];
  for (const item of items) {
    const line = item.line - 1;
    let shift = 0;
    let removed = false;
    for (const change of changes) {
      if (line > change.endLine) {
        shift += change.text.split('\n').length - 1 - (change.endLine - change.startLine);
      } else if (line > change.startLine) {
        removed = true;
      }
    }
    if (!removed) kept.push({ ...item, line: item.line + shift });
  }
  return kept;
}

module.exports = { projectAnnotations, declarationTarget, clampPosition, resourceName, shiftLines, UNPRICED_MARKERS };
