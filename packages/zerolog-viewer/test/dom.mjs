export class Node {
  constructor(tag, doc) {
    this.tagName = tag;
    this.ownerDocument = doc;
    this.childNodes = [];
    this.attrs = {};
    this.listeners = new Map();
    this.className = '';
    this.hidden = false;
    this.scrollTop = 0;
    this.scrollHeight = 1000;
    this.clientHeight = 100;
    this._text = '';
  }

  get firstChild() { return this.childNodes[0] ?? null; }

  appendChild(child) {
    child.parentNode = this;
    this.childNodes.push(child);
    return child;
  }

  removeChild(child) {
    const index = this.childNodes.indexOf(child);
    if (index >= 0) this.childNodes.splice(index, 1);
    child.parentNode = null;
    return child;
  }

  set textContent(value) {
    this.childNodes = [];
    this._text = String(value);
  }

  get textContent() {
    return this._text + this.childNodes.map((c) => c.textContent).join('');
  }

  setAttribute(key, value) { this.attrs[key] = String(value); }

  addEventListener(name, fn) {
    if (!this.listeners.has(name)) this.listeners.set(name, []);
    this.listeners.get(name).push(fn);
  }

  removeEventListener(name, fn) {
    const list = this.listeners.get(name) ?? [];
    this.listeners.set(name, list.filter((f) => f !== fn));
  }

  dispatch(name) { for (const fn of this.listeners.get(name) ?? []) fn({ type: name, target: this }); }

  find(predicate, out = []) {
    for (const child of this.childNodes) {
      if (predicate(child)) out.push(child);
      child.find(predicate, out);
    }
    return out;
  }

  byClass(name) { return this.find((n) => n.className.split(' ').includes(name)); }
}

export function createDocument() {
  const doc = { createElement: (tag) => new Node(tag, doc) };
  return doc;
}

export function mount() {
  const document = createDocument();
  const container = document.createElement('div');
  return { document, container };
}
