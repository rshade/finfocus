import { el } from './app.js';

// The server formats the spot label (for example "SPOT · risk 0.12 · -62% vs on-demand"). The text
// carries the meaning; the cyan accent only emphasizes it. Returns child nodes, empty for no label.
// The leading space keeps the label a separate word in the accessible name ("Modify SPOT", not "ModifySPOT").
export function spotBadge(text) {
  return text ? [' ', el('span', { class: 'spot-badge', text })] : [];
}
