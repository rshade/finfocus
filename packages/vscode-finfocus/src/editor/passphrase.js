'use strict';

/**
 * The passphrase prompt for one context. The value is returned to the
 * scheduler, which keeps it in memory for that context only; it is never
 * logged or written. An empty value is valid and is returned as an empty
 * string; dismissing the box returns null.
 *
 * @param {{
 *   api: typeof import('vscode'),
 *   labelFor: (key: string) => string,
 * }} deps
 * @returns {(query: import('../scheduler').PassphraseQuery) => Promise<string | null>}
 */
function createPassphraseAsker({ api, labelFor }) {
  return async ({ key, retry, signal }) => {
    const source = new api.CancellationTokenSource();
    const abort = () => source.cancel();
    if (signal.aborted) source.cancel();
    signal.addEventListener('abort', abort);
    try {
      const value = await api.window.showInputBox(
        {
          title: `FinFocus: Pulumi passphrase for ${labelFor(key)}`,
          prompt: retry
            ? 'The passphrase was not accepted. Enter it again, or press Escape to cancel. An empty passphrase is allowed.'
            : 'Enter the Pulumi config passphrase. An empty passphrase is allowed. It is kept in memory for this stack only.',
          password: true,
          ignoreFocusOut: true,
        },
        source.token,
      );
      return value ?? null;
    } finally {
      signal.removeEventListener('abort', abort);
      source.dispose();
    }
  };
}

module.exports = { createPassphraseAsker };
