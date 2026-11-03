const MAX_TAIL = 64 * 1024;

/** A line that was not a JSON object; kept so nothing is silently lost. */
export class RawLine {
  constructor(text) {
    this.text = text;
  }
}

/**
 * Parses one NDJSON line. Returns the decoded object, a RawLine when the line
 * is not a JSON object, or null for a blank line.
 */
export function parseLine(line) {
  const text = line.endsWith('\r') ? line.slice(0, -1) : line;
  if (text.trim() === '') return null;
  try {
    const value = JSON.parse(text);
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) return value;
  } catch {
    // fall through to a raw line
  }
  return new RawLine(text);
}

/**
 * Creates a chunk-safe NDJSON parser. push() returns the items completed by
 * the chunk and keeps the unterminated tail; flush() returns the tail.
 */
export function createNDJSONParser() {
  let tail = '';
  const collect = (lines) => {
    const items = [];
    for (const line of lines) {
      const item = parseLine(line);
      if (item !== null) items.push(item);
    }
    return items;
  };
  return {
    push(chunk) {
      const parts = (tail + chunk).split('\n');
      tail = parts.pop();
      const items = collect(parts);
      if (tail.length > MAX_TAIL) {
        items.push(...collect([tail]));
        tail = '';
      }
      return items;
    },
    flush() {
      const rest = tail;
      tail = '';
      return collect([rest]);
    },
  };
}

/** Parses a complete NDJSON text, including an unterminated last line. */
export function parseNDJSON(text) {
  const parser = createNDJSONParser();
  return [...parser.push(text), ...parser.flush()];
}
