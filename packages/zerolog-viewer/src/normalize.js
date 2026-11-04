import { RawLine } from './ndjson.js';

export const LEVELS = Object.freeze(['trace', 'debug', 'info', 'warn', 'error', 'fatal', 'panic']);

export const defaultFieldMap = Object.freeze({ time: 'time', level: 'level', message: 'message' });
export const defaultSourceFields = Object.freeze(['plugin_name', 'component']);
export const defaultPluginFields = Object.freeze(['plugin_name']);

const LEVEL_ALIASES = { warning: 'warn', err: 'error', critical: 'fatal' };

function normalizeLevel(value) {
  const name = typeof value === 'string' ? value.trim().toLowerCase() : '';
  const resolved = LEVEL_ALIASES[name] ?? name;
  return LEVELS.includes(resolved) ? resolved : 'info';
}

function normalizeTime(value, now) {
  let ms = NaN;
  if (typeof value === 'number') ms = value < 1e12 ? value * 1000 : value;
  else if (typeof value === 'string') ms = Date.parse(value);
  const date = new Date(ms);
  return Number.isNaN(date.getTime()) ? now() : date.toISOString();
}

/**
 * Converts a decoded zerolog object (or a RawLine) into an Entry.
 * options: fieldMap, sourceFields (priority order), pluginFields, now.
 */
export function normalizeEntry(input, options = {}) {
  const now = options.now ?? (() => new Date().toISOString());
  if (input instanceof RawLine) {
    return { time: now(), level: 'info', source: 'unknown', sourceKind: 'core', message: input.text, raw: true };
  }
  const map = { ...defaultFieldMap, ...options.fieldMap };
  const sourceFields = options.sourceFields ?? defaultSourceFields;
  const pluginFields = options.pluginFields ?? defaultPluginFields;
  const obj = input ?? {};

  let source = 'core';
  let sourceKind = 'core';
  for (const name of sourceFields) {
    const value = obj[name];
    if (typeof value === 'string' && value !== '') {
      source = value;
      sourceKind = pluginFields.includes(name) ? 'plugin' : 'core';
      break;
    }
  }

  const fields = {};
  const mapped = new Set([map.time, map.level, map.message]);
  for (const [key, value] of Object.entries(obj)) {
    if (!mapped.has(key)) fields[key] = value;
  }
  const message = obj[map.message];
  const entry = {
    time: normalizeTime(obj[map.time], now),
    level: normalizeLevel(obj[map.level]),
    source,
    sourceKind,
    message: typeof message === 'string' ? message : message == null ? '' : String(message),
  };
  if (Object.keys(fields).length > 0) entry.fields = fields;
  return entry;
}
