'use strict';

const { declarationTarget, clampPosition } = require('../annotations');

/**
 * Opens the declaration a row recorded. The location must be a safe
 * project-relative path; the resulting URI must stay under the project, and the
 * position is clamped to the document so a stale line cannot point past it.
 *
 * @param {typeof import('vscode')} api
 * @param {import('vscode').Uri} projectUri
 * @param {import('../result').OverviewRow} row
 * @returns {Promise<boolean>} false when the row has no safe declaration or it cannot be opened
 */
async function openDeclaration(api, projectUri, row) {
  const location = declarationTarget(row);
  if (location === null) return false;
  const target = api.Uri.joinPath(projectUri, ...location.file.split('/'));
  const root = projectUri.toString().replace(/\/$/, '');
  if (!target.toString().startsWith(`${root}/`)) return false;
  let document;
  try {
    document = await api.workspace.openTextDocument(target);
  } catch {
    return false;
  }
  const { line, character } = clampPosition(location, {
    lineCount: document.lineCount,
    lineLength: (n) => document.lineAt(n).text.length,
  });
  const position = new api.Position(line, character);
  const editor = await api.window.showTextDocument(document, { selection: new api.Range(position, position) });
  editor.revealRange(new api.Range(position, position), api.TextEditorRevealType.InCenterIfOutsideViewport);
  return true;
}

module.exports = { openDeclaration };
