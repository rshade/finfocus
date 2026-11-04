import test from 'node:test';
import assert from 'node:assert/strict';
import { defineViewerElement } from '../src/element.js';
import { Node, createDocument } from './dom.mjs';

class FakeElement extends Node {
  constructor() {
    super('zerolog-viewer', createDocument());
    this.attrMap = {};
  }

  getAttribute(name) { return name in this.attrMap ? this.attrMap[name] : null; }
  hasAttribute(name) { return name in this.attrMap; }

  set(name, value) {
    this.attrMap[name] = value;
    this.attributeChangedCallback(name);
  }
}

const Element = defineViewerElement(FakeElement);
const entry = (over = {}) => ({
  time: '2026-11-02T10:00:00Z', level: 'info', source: 'core', sourceKind: 'core', message: 'hello', ...over,
});
const rows = (el) => el.byClass('zlv-row');
const visible = (el) => rows(el).filter((r) => !r.hidden).length;

test('importing the module without HTMLElement does not throw', async () => {
  assert.equal(typeof HTMLElement, 'undefined');
  const mod = await import('../src/element.js');
  assert.equal(mod.ZerologViewerElement, null);
});

test('connect renders a viewer and push reaches it', () => {
  const el = new Element();
  el.connectedCallback();
  el.push(entry({ message: 'one' }));
  el.pushLine(JSON.stringify({ level: 'warn', message: 'two' }));
  el.pushChunk('{"level":"error","message":"three"}\n');
  assert.equal(rows(el).length, 3);
  assert.equal(rows(el)[2].byClass('zlv-message')[0].textContent, 'three');
});

test('initial attributes become filters and changes re-filter', () => {
  const el = new Element();
  el.attrMap['min-level'] = 'warn';
  el.connectedCallback();
  el.push(entry({ level: 'info', message: 'quiet' }));
  el.push(entry({ level: 'error', message: 'loud' }));
  assert.equal(visible(el), 1);
  el.set('min-level', '');
  assert.equal(visible(el), 2);
  el.set('text', 'quiet');
  assert.equal(visible(el), 1);
});

test('sources attribute is a comma list', () => {
  const el = new Element();
  el.connectedCallback();
  el.push(entry({ source: 'aws-public', sourceKind: 'plugin', message: 'p' }));
  el.push(entry({ message: 'c' }));
  el.set('sources', 'aws-public, other');
  assert.equal(visible(el), 1);
});

test('disconnect removes the viewer and later pushes are ignored', () => {
  const el = new Element();
  el.connectedCallback();
  el.push(entry());
  el.disconnectedCallback();
  assert.equal(el.childNodes.length, 0);
  el.push(entry());
  el.set('text', 'x');
  assert.equal(el.childNodes.length, 0);
});
