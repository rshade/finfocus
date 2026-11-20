'use strict';

const { buildDashboard } = require('../dashboard');
const { REASONS } = require('../stale');

/**
 * @typedef {import('../dashboard').ResourceGroup} ResourceGroup
 * @typedef {import('../dashboard').ResourceItem} ResourceItem
 * @typedef {import('vscode').TreeItem & {group?: ResourceGroup}} Node
 */

/**
 * @param {string[]} reasons
 * @returns {string}
 */
function marker(reasons) {
  if (reasons.length === 0) return '';
  return reasons.every((r) => r === REASONS.unknown) ? ' · freshness unknown' : ' · stale';
}

/**
 * The complete resource list: every resource under its type with an unpriced
 * group last. It reads only the tracked result, so it keeps working when the
 * summary card cannot render.
 */
class DashboardTree {
  /**
   * @param {{api: typeof import('vscode'), tracking: InstanceType<typeof import('./staleTracking').StaleTracking>}} deps
   */
  constructor(deps) {
    this.api = deps.api;
    this.tracking = deps.tracking;
    this.changed = new this.api.EventEmitter();
    this.onDidChangeTreeData = this.changed.event;
    this.subscription = this.tracking.onDidChange(() => this.changed.fire(undefined));
  }

  /**
   * @param {Node} [node]
   * @returns {Promise<Node[]>}
   */
  async getChildren(node) {
    if (node?.group !== undefined) return node.group.items.map((item) => this.resourceNode(item));
    if (node !== undefined) return [];
    const view = this.tracking.view();
    if (view === null) return [];
    const dashboard = buildDashboard({ result: view.result, stale: view.stale, completeness: view.completeness });
    const groups = dashboard.groups.map((group) => this.groupNode(group));
    return dashboard.completeness.kind === 'incomplete' ? [this.noticeNode(dashboard.completeness.text), ...groups] : groups;
  }

  /**
   * @param {Node} node
   * @returns {Node}
   */
  getTreeItem(node) {
    return node;
  }

  /**
   * A line above the groups for a result that may understate the total.
   *
   * @param {string} text
   * @returns {Node}
   */
  noticeNode(text) {
    /** @type {Node} */
    const node = new this.api.TreeItem(text, this.api.TreeItemCollapsibleState.None);
    node.iconPath = new this.api.ThemeIcon('warning');
    node.contextValue = 'notice';
    node.tooltip = text;
    node.accessibilityInformation = { label: `Warning: ${text}`, role: 'treeitem' };
    return node;
  }

  /**
   * @param {ResourceGroup} group
   * @returns {Node}
   */
  groupNode(group) {
    const count = group.items.length;
    /** @type {Node} */
    const node = new this.api.TreeItem(`${group.label} (${count})`, this.api.TreeItemCollapsibleState.Expanded);
    node.group = group;
    node.contextValue = 'group';
    node.accessibilityInformation = {
      label: `${group.label}, ${count} ${count === 1 ? 'resource' : 'resources'}${group.unpriced ? ', not priced' : ''}`,
      role: 'treeitem',
    };
    return node;
  }

  /**
   * @param {ResourceItem} resource
   * @returns {Node}
   */
  resourceNode(resource) {
    /** @type {Node} */
    const node = new this.api.TreeItem(resource.name, this.api.TreeItemCollapsibleState.None);
    node.id = resource.urn;
    node.description = `${resource.projected}${marker(resource.reasons)}`;
    node.contextValue = resource.openable ? 'resource.openable' : 'resource';
    node.tooltip = resource.detail.join('\n');
    node.accessibilityInformation = { label: resource.detail.join(', '), role: 'treeitem' };
    node.command = { command: 'finfocus.openResource', title: 'Open declaration', arguments: [resource.urn] };
    return node;
  }

  dispose() {
    this.subscription.dispose();
    this.changed.dispose();
  }
}

module.exports = { DashboardTree };
