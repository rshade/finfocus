'use strict';

const { openDeclaration } = require('./navigation');
const { buildDashboard } = require('../dashboard');

/**
 * What the card and the tree do with a resource: open its declaration, or list
 * its details for a keyboard or screen-reader user. Both read the shown
 * context's current result, so a resource that left it cannot be opened.
 */
class ResourceActions {
  /**
   * @param {{
   *   api: typeof import('vscode'),
   *   tracking: InstanceType<typeof import('./staleTracking').StaleTracking>,
   *   present?: (lines: string[]) => unknown,
   *   notify?: (message: string) => unknown,
   * }} deps
   */
  constructor(deps) {
    this.api = deps.api;
    this.tracking = deps.tracking;
    this.present = deps.present ?? ((lines) => this.api.window.showQuickPick(lines, { title: lines[0], placeHolder: 'Resource details' }));
    this.notify = deps.notify ?? ((message) => this.api.window.showInformationMessage(message));
  }

  /**
   * @param {string} urn
   * @returns {import('../result').OverviewRow | null}
   */
  rowFor(urn) {
    return this.tracking.view()?.result.rows.find((r) => r.urn === urn) ?? null;
  }

  /**
   * @param {string} urn
   * @returns {Promise<boolean>} whether a declaration was opened
   */
  async open(urn) {
    const view = this.tracking.view();
    const row = this.rowFor(urn);
    if (view === null || row === null) return false;
    if (await openDeclaration(this.api, view.project, row)) return true;
    const name = row.urn.slice(row.urn.lastIndexOf('::') + 2);
    await this.notify(`${name} has no recorded location in this project.`);
    return false;
  }

  /**
   * @param {string} urn
   * @returns {Promise<void>}
   */
  async details(urn) {
    const view = this.tracking.view();
    if (view === null) return;
    const dashboard = buildDashboard({ result: view.result, stale: view.stale });
    const item = dashboard.groups.flatMap((g) => g.items).find((i) => i.urn === urn);
    if (item !== undefined) await this.present(item.detail);
  }
}

module.exports = { ResourceActions };
