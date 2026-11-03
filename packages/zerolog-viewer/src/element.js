import { createLogViewer } from './viewer.js';

const OBSERVED = ['sources', 'min-level', 'text', 'regex'];

function filtersFrom(element) {
  const sources = (element.getAttribute('sources') ?? '')
    .split(',')
    .map((name) => name.trim())
    .filter(Boolean);
  return {
    sources,
    minLevel: element.getAttribute('min-level') ?? '',
    text: element.getAttribute('text') ?? '',
    regex: element.hasAttribute('regex'),
  };
}

/**
 * Builds the <zerolog-viewer> class over a base HTMLElement class. Exposed so
 * tests can supply a DOM double; the default export below uses the real one.
 */
export function defineViewerElement(Base) {
  return class ZerologViewerElement extends Base {
    static get observedAttributes() {
      return OBSERVED;
    }

    connectedCallback() {
      if (this.viewer) return;
      this.viewer = createLogViewer(this, {
        document: this.ownerDocument,
        filters: filtersFrom(this),
      });
    }

    disconnectedCallback() {
      this.viewer?.destroy();
      this.viewer = null;
    }

    attributeChangedCallback() {
      this.viewer?.setFilters(filtersFrom(this));
    }

    push(entry) {
      this.viewer?.push(entry);
    }

    pushLine(text) {
      this.viewer?.pushLine(text);
    }

    pushChunk(text) {
      this.viewer?.pushChunk(text);
    }

    clear() {
      this.viewer?.clear();
    }
  };
}

export const ZerologViewerElement =
  typeof HTMLElement === 'undefined' ? null : defineViewerElement(HTMLElement);

if (ZerologViewerElement && typeof customElements !== 'undefined' && !customElements.get('zerolog-viewer')) {
  customElements.define('zerolog-viewer', ZerologViewerElement);
}
