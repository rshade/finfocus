'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { AnnotationProvider } = require('../../src/editor/annotations');

const HOSTILE = '[run](command:workbench.action.reloadWindow) <script>alert(1)</script> $(rm -rf /)';

class FakeMarkdownString {
  constructor() {
    this.markdown = [];
    this.text = [];
  }

  appendMarkdown(value) {
    this.markdown.push(value);
    return this;
  }

  appendText(value) {
    this.text.push(value);
    return this;
  }
}

function fakeApi() {
  const noop = { dispose() {} };
  return {
    window: {
      createTextEditorDecorationType: () => ({}),
      onDidChangeVisibleTextEditors: () => noop,
      visibleTextEditors: [],
    },
    workspace: { onDidChangeTextDocument: () => noop },
    ThemeColor: class {},
    Range: class {},
    MarkdownString: FakeMarkdownString,
    Uri: { joinPath: (base, ...parts) => ({ toString: () => `${base}/${parts.join('/')}` }) },
  };
}

function paintedHover(hover) {
  const api = fakeApi();
  const provider = new AnnotationProvider(api);
  const uri = { toString: () => 'file:///p/index.js' };
  const options = [];
  provider.byDocument.set(uri.toString(), [
    { line: 1, text: HOSTILE, hover, urns: ['urn:a'] },
  ]);
  provider.freshness = new Map([['urn:a', [HOSTILE]]]);
  provider.paint({
    document: { uri, lineCount: 1, lineAt: () => ({ range: { end: {} } }) },
    setDecorations: (_, opts) => options.push(...opts),
  });
  return options[0];
}

test('hover content from finfocus reaches the editor only as escaped text', () => {
  const option = paintedHover([HOSTILE, `second ${HOSTILE}`]);
  const hover = option.hoverMessage;
  assert.deepEqual(hover.text.slice(0, 2), [HOSTILE, `second ${HOSTILE}`]);
  assert.ok(hover.text.includes(`Stale: ${HOSTILE}`));
  for (const chunk of hover.markdown) assert.match(chunk, /^\n\n$/, 'only fixed separators are markdown');
});

test('the hover is not trusted, so a command link cannot run', () => {
  const option = paintedHover([HOSTILE]);
  assert.notEqual(option.hoverMessage.isTrusted, true);
  assert.notEqual(option.hoverMessage.supportHtml, true);
});

test('the inline text is plain contentText, never markup', () => {
  const option = paintedHover([HOSTILE]);
  assert.equal(typeof option.renderOptions.after.contentText, 'string');
  assert.ok(option.renderOptions.after.contentText.startsWith(HOSTILE));
});
