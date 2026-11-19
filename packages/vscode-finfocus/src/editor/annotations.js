'use strict';

const { projectAnnotations, shiftLines } = require('../annotations');
const { REASONS } = require('../stale');

/**
 * @typedef {import('../annotations').Annotation} Annotation
 * @typedef {import('../result').OverviewRow} OverviewRow
 */

/**
 * @param {string[]} reasons
 * @returns {string}
 */
function staleLabel(reasons) {
  return reasons.every((r) => r === REASONS.unknown) ? REASONS.unknown : 'stale';
}

/**
 * Paints each priced declaration with the projected string finfocus produced,
 * at the end of its line. The model is kept in line numbers and moved through
 * every edit, so a repaint after a change never lands on the old line.
 */
class AnnotationProvider {
  /**
   * @param {typeof import('vscode')} api
   */
  constructor(api) {
    this.api = api;
    this.decoration = api.window.createTextEditorDecorationType({
      after: {
        margin: '0 0 0 2em',
        color: new api.ThemeColor('editorCodeLens.foreground'),
        fontStyle: 'italic',
      },
    });
    /** @type {Map<string, Annotation[]>} */
    this.byDocument = new Map();
    /** @type {Map<string, string[]>} */
    this.freshness = new Map();
    this.disposables = [
      api.window.onDidChangeVisibleTextEditors(() => this.paintAll()),
      api.workspace.onDidChangeTextDocument((event) => this.onEdit(event)),
    ];
  }

  /**
   * Replaces what is shown. Rows without a safe, project-relative location are
   * skipped by the projection.
   *
   * @param {import('vscode').Uri} projectUri
   * @param {OverviewRow[]} rows
   */
  show(projectUri, rows) {
    this.byDocument.clear();
    for (const [file, annotations] of projectAnnotations(rows)) {
      const target = this.api.Uri.joinPath(projectUri, ...file.split('/'));
      this.byDocument.set(target.toString(), annotations);
    }
    this.paintAll();
  }

  clear() {
    this.byDocument.clear();
    this.freshness.clear();
    this.paintAll();
  }

  /**
   * Replaces the stale reasons by URN. A line shows a reason when any resource
   * on it has one, in the text and in the hover, so the state is not carried
   * by color alone.
   *
   * @param {Map<string, string[]>} freshness
   */
  setFreshness(freshness) {
    this.freshness = new Map(freshness);
    this.paintAll();
  }

  /**
   * @param {Annotation} annotation
   * @returns {string[]}
   */
  reasonsFor(annotation) {
    const reasons = new Set();
    for (const urn of annotation.urns) {
      for (const reason of this.freshness.get(urn) ?? []) reasons.add(reason);
    }
    return [...reasons];
  }

  paintAll() {
    for (const editor of this.api.window.visibleTextEditors) this.paint(editor);
  }

  /**
   * @param {import('vscode').TextEditor} editor
   */
  paint(editor) {
    const annotations = this.byDocument.get(editor.document.uri.toString()) ?? [];
    const options = [];
    for (const annotation of annotations) {
      if (annotation.line > editor.document.lineCount) continue;
      const end = editor.document.lineAt(annotation.line - 1).range.end;
      const hover = new this.api.MarkdownString();
      annotation.hover.forEach((line, i) => {
        if (i > 0) hover.appendMarkdown('\n\n');
        hover.appendText(line);
      });
      const reasons = this.reasonsFor(annotation);
      if (reasons.length > 0) hover.appendMarkdown('\n\n').appendText(`Stale: ${reasons.join(', ')}`);
      options.push({
        range: new this.api.Range(end, end),
        hoverMessage: hover,
        renderOptions: {
          after: {
            contentText: reasons.length === 0 ? annotation.text : `${annotation.text} · ${staleLabel(reasons)}`,
            ...(reasons.length === 0 ? {} : { color: new this.api.ThemeColor('editorWarning.foreground') }),
          },
        },
      });
    }
    editor.setDecorations(this.decoration, options);
  }

  /**
   * @param {import('vscode').TextDocumentChangeEvent} event
   */
  onEdit(event) {
    const key = event.document.uri.toString();
    const annotations = this.byDocument.get(key);
    if (annotations === undefined || event.contentChanges.length === 0) return;
    this.byDocument.set(
      key,
      shiftLines(
        annotations,
        event.contentChanges.map((change) => ({
          startLine: change.range.start.line,
          endLine: change.range.end.line,
          text: change.text,
        })),
      ),
    );
    this.paintAll();
  }

  /**
   * The annotations currently placed in a document, for tests and the
   * navigation command.
   *
   * @param {import('vscode').Uri} uri
   * @returns {Annotation[]}
   */
  annotationsFor(uri) {
    return this.byDocument.get(uri.toString()) ?? [];
  }

  dispose() {
    for (const d of this.disposables) d.dispose();
    this.decoration.dispose();
    this.byDocument.clear();
    this.freshness.clear();
  }
}

module.exports = { AnnotationProvider };
