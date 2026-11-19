'use strict';

const { buildDashboard } = require('../dashboard');

const REVEAL_COMMAND = 'finfocus.revealDashboard';

/**
 * The monthly total and budget status in the status bar. Clicking it reveals
 * the card. The text names the budget state in words.
 */
class DashboardStatusBar {
  /**
   * @param {{api: typeof import('vscode'), tracking: InstanceType<typeof import('./staleTracking').StaleTracking>}} deps
   */
  constructor(deps) {
    this.api = deps.api;
    this.tracking = deps.tracking;
    this.item = this.api.window.createStatusBarItem(this.api.StatusBarAlignment.Left, 50);
    this.item.command = REVEAL_COMMAND;
    this.subscription = this.tracking.onDidChange(() => this.update());
    this.update();
  }

  update() {
    const view = this.tracking.view();
    if (view === null) {
      this.item.hide();
      return;
    }
    const dashboard = buildDashboard({ result: view.result, stale: view.stale, completeness: view.completeness });
    const budget = dashboard.budget.configured ? `Budget ${dashboard.budget.health}` : 'no budget';
    const incomplete = dashboard.completeness.kind === 'incomplete';
    this.item.text = `$(graph) ${dashboard.summary.projected} · ${budget}${incomplete ? ' · $(warning) incomplete' : ''}`;
    const caveat = incomplete ? ` ${dashboard.completeness.text}.` : '';
    this.item.accessibilityInformation = {
      label: `FinFocus ${dashboard.stack}: projected ${dashboard.summary.projected} a month, ${dashboard.budget.text}.${caveat} Press to open the cost summary.`,
      role: 'button',
    };
    const notes = [dashboard.budget.text, dashboard.freshness.text, dashboard.completeness.text].filter((part) => part !== '');
    this.item.tooltip = `${dashboard.stack}: ${notes.join('\n')}`;
    this.item.show();
  }

  dispose() {
    this.subscription.dispose();
    this.item.dispose();
  }
}

module.exports = { DashboardStatusBar, REVEAL_COMMAND };
