import { LEVELS } from './normalize.js';

function rank(level) {
  const index = LEVELS.indexOf(level);
  return index < 0 ? LEVELS.indexOf('info') : index;
}

function valueStrings(value, out) {
  if (value === null || value === undefined) return out;
  if (typeof value === 'object') {
    for (const item of Object.values(value)) valueStrings(item, out);
    return out;
  }
  out.push(String(value));
  return out;
}

function haystack(entry) {
  return [entry.message ?? '', ...valueStrings(entry.fields, [])];
}

function textMatcher(text, regex) {
  if (text === '') return { test: null, error: null };
  if (regex) {
    try {
      const pattern = new RegExp(text, 'i');
      return { test: (value) => pattern.test(value), error: null };
    } catch (err) {
      return { test: null, error: err.message };
    }
  }
  const needle = text.toLowerCase();
  return { test: (value) => value.toLowerCase().includes(needle), error: null };
}

/**
 * Turns a raw filter ({ sources, kind, minLevel, text, regex }) into one that can be
 * matched repeatedly. With regex set, text is a case-insensitive pattern; an
 * invalid pattern sets error and filters nothing. Without it, text is a
 * case-insensitive substring.
 */
export function compileFilter(filter = {}) {
  const { test, error } = textMatcher(filter.text ?? '', Boolean(filter.regex));
  return {
    sources: filter.sources ?? [],
    kind: filter.kind ?? '',
    minLevel: filter.minLevel ?? '',
    text: filter.text ?? '',
    regex: Boolean(filter.regex),
    test,
    error,
    compiled: true,
  };
}

/**
 * True when the entry satisfies the filter: source in the selected set (empty
 * means all), source kind equal to kind ('' means any), level at least
 * minLevel, and the text matching the message or any field value (never the
 * source, kind, level, or field names).
 */
export function matchesFilter(entry, filter = {}) {
  const f = filter.compiled ? filter : compileFilter(filter);
  if (f.sources.length > 0 && !f.sources.includes(entry.source)) return false;
  if (f.kind && entry.sourceKind !== f.kind) return false;
  if (f.minLevel && rank(entry.level) < rank(f.minLevel)) return false;
  if (f.test === null) return true;
  return haystack(entry).some(f.test);
}
