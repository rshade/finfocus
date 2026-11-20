'use strict';

(function () {
  const vscode = acquireVsCodeApi();
  const byId = (id) => document.getElementById(id);

  function text(id, value) {
    byId(id).textContent = value;
  }

  function topItem(item) {
    const li = document.createElement('li');
    const button = document.createElement('button');
    button.type = 'button';
    const name = document.createElement('span');
    name.className = 'name';
    name.textContent = item.name;
    const cost = document.createElement('span');
    cost.textContent = item.projected;
    button.append(name, cost);
    button.setAttribute('aria-label', item.detail.join(', '));
    button.addEventListener('click', () => vscode.postMessage({ type: 'open', urn: item.urn }));
    button.addEventListener('keydown', (event) => {
      if (event.key === 'F2' || (event.key === 'Enter' && event.shiftKey)) {
        event.preventDefault();
        vscode.postMessage({ type: 'details', urn: item.urn });
      }
    });
    li.append(button);
    return li;
  }

  function renderCompleteness(completeness) {
    const note = byId('completeness');
    note.hidden = completeness.text === '';
    note.classList.toggle('incomplete', completeness.kind === 'incomplete');
    note.textContent = completeness.kind === 'incomplete' ? `\u26A0 ${completeness.text}` : completeness.text;
    note.title = completeness.help;
    if (completeness.help === '') note.removeAttribute('aria-description');
    else note.setAttribute('aria-description', completeness.help);
  }

  function render(state) {
    byId('empty').hidden = state !== null;
    byId('card').hidden = state === null;
    if (state === null) {
      text('stack', 'FinFocus');
      return;
    }
    text('stack', `${state.stack}${state.stateOnly ? ' · deployed state' : ' · preview'}`);
    text('projected', state.summary.projected);
    text('actual', state.summary.actual);
    text('delta', state.summary.delta);
    renderCompleteness(state.completeness);
    text('budget', state.budget.text);
    text('freshness', state.freshness.text);
    byId('top').replaceChildren(...state.topFive.map(topItem));
  }

  window.addEventListener('message', (event) => {
    if (event.data && event.data.type === 'state') render(event.data.state);
  });
  byId('preview').addEventListener('click', () => vscode.postMessage({ type: 'preview' }));
  vscode.postMessage({ type: 'ready' });
})();
