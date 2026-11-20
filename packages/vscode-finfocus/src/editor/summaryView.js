'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { buildDashboard } = require('../dashboard');

const VIEW_ID = 'finfocus.summary';
const FALLBACK = 'The summary card could not be shown. The resource tree and the status bar still show the stack.';

/**
 * @param {string} text
 * @returns {string}
 */
function escapeHtml(text) {
  return text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
}

/**
 * The side-bar summary card: stack, totals, budget, stale count, top five and
 * Run preview. The page is a fixed template under a strict content security
 * policy; resource text reaches it only as data, written with `textContent`.
 * A failure here is contained, so the tree and status bar never depend on it.
 */
class SummaryView {
  /**
   * @param {{
   *   api: typeof import('vscode'),
   *   tracking: InstanceType<typeof import('./staleTracking').StaleTracking>,
   *   actions: InstanceType<typeof import('./resourceActions').ResourceActions>,
   *   extensionUri: import('vscode').Uri,
   *   log?: (message: string) => void,
   * }} deps
   */
  constructor(deps) {
    this.api = deps.api;
    this.tracking = deps.tracking;
    this.actions = deps.actions;
    this.extensionUri = deps.extensionUri;
    this.log = deps.log ?? (() => {});
    /** @type {import('vscode').WebviewView | null} */
    this.view = null;
    this.failed = false;
    this.disposables = [this.tracking.onDidChange(() => this.post())];
  }

  /**
   * @param {import('vscode').WebviewView} view
   */
  resolveWebviewView(view) {
    this.view = view;
    this.failed = false;
    try {
      view.webview.options = { enableScripts: true, localResourceRoots: [this.api.Uri.joinPath(this.extensionUri, 'media')] };
      view.webview.html = this.html(view.webview);
      this.disposables.push(view.webview.onDidReceiveMessage((message) => void this.onMessage(message)));
      view.onDidDispose(() => {
        if (this.view === view) this.view = null;
      });
      this.post();
    } catch (error) {
      this.failed = true;
      this.log(`Summary card failed: ${error instanceof Error ? error.message : String(error)}`);
      this.fallback(view);
    }
  }

  /**
   * @param {import('vscode').WebviewView} view
   */
  fallback(view) {
    try {
      view.webview.html = `<!DOCTYPE html><html lang="en"><body><p>${escapeHtml(FALLBACK)}</p></body></html>`;
    } catch {
      this.log(FALLBACK);
    }
  }

  /**
   * @param {import('vscode').Webview} webview
   * @returns {string}
   */
  html(webview) {
    const media = path.join(this.extensionUri.fsPath, 'media');
    const nonce = crypto.randomBytes(16).toString('base64');
    const asUri = (/** @type {string} */ file) => webview.asWebviewUri(this.api.Uri.joinPath(this.extensionUri, 'media', file)).toString();
    const values = { cspSource: webview.cspSource, nonce, styleUri: asUri('summary.css'), scriptUri: asUri('summary.js') };
    return fs
      .readFileSync(path.join(media, 'summary.html'), 'utf8')
      .replace(/\{\{(\w+)\}\}/g, (_, key) => escapeHtml(values[/** @type {keyof typeof values} */ (key)] ?? ''));
  }

  /** Sends the current projection, or null when there is no result. */
  post() {
    if (this.view === null || this.failed) return;
    const tracked = this.tracking.view();
    const state = tracked === null ? null : buildDashboard({ result: tracked.result, stale: tracked.stale, completeness: tracked.completeness });
    try {
      void Promise.resolve(this.view.webview.postMessage({ type: 'state', state })).catch(() => {});
    } catch (error) {
      this.log(`Summary card update failed: ${error instanceof Error ? error.message : String(error)}`);
    }
  }

  /**
   * Only these message shapes act, and a resource must be in the current result.
   *
   * @param {unknown} message
   * @returns {Promise<void>}
   */
  async onMessage(message) {
    if (message === null || typeof message !== 'object') return;
    const { type, urn } = /** @type {{type?: unknown, urn?: unknown}} */ (message);
    if (type === 'ready') this.post();
    else if (type === 'preview') await this.api.commands.executeCommand('finfocus.runPreview');
    else if (type === 'open' && typeof urn === 'string') await this.actions.open(urn);
    else if (type === 'details' && typeof urn === 'string') await this.actions.details(urn);
  }

  /** Shows the card, for the status-bar item. */
  async reveal() {
    await this.api.commands.executeCommand(`${VIEW_ID}.focus`);
  }

  dispose() {
    for (const d of this.disposables) d.dispose();
  }
}

module.exports = { SummaryView, VIEW_ID };
