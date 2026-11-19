'use strict';

const { parseOverview, ResultError } = require('./result');
const { describeOutcome, RunnerError } = require('./runner');
const { gapsOf, needsRetry, settle } = require('./completeness');

/**
 * @typedef {import('./result').OverviewResult} OverviewResult
 * @typedef {import('./completeness').Completeness} Completeness
 * @typedef {import('./stale').RequestSnapshot} RequestSnapshot
 * @typedef {import('./shutdown').StopResult} StopResult
 * @typedef {'idle' | 'running' | 'cancelling' | 'awaitingPassphrase' | 'cleanupFailed'} RunState
 * @typedef {{
 *   kind: 'result', result: OverviewResult, startedAt: Date, requestId: number, snapshot: RequestSnapshot | null,
 *   completeness: Completeness | null, fromCache: boolean,
 * }
 *   | {kind: 'failure', message: string}
 *   | {kind: 'superseded'}
 *   | {kind: 'dismissed'}
 *   | {kind: 'blocked', message: string}} RunResult
 * @typedef {{key: string, retry: boolean, signal: AbortSignal}} PassphraseQuery
 */

/**
 * @param {OverviewResult} result
 * @param {import('./completeness').CompletenessKind} kind
 * @param {string[]} [gaps]
 * @returns {Completeness}
 */
function describe(result, kind, gaps = gapsOf(result)) {
  return { kind, gaps, total: result.rows.length };
}

/**
 * Per-context overview runs. A context has at most one process; a newer
 * request invalidates the older one first, stops its process to a confirmed
 * end, and only then starts. Only the latest request for a still-selected
 * context can publish a result or show a prompt, so late output is dropped.
 * A cleanup that cannot be confirmed blocks the context until it is retried.
 */
class Scheduler {
  /**
   * @param {{
   *   runner: Pick<InstanceType<typeof import('./runner').Runner>, 'overview'>,
   *   askPassphrase?: (query: PassphraseQuery) => Promise<string | null>,
   *   now?: () => Date,
   *   cache?: Pick<InstanceType<typeof import('./cache').ResultCache>, 'enabled' | 'lookup' | 'previous' | 'save'> | null,
   * }} options
   */
  constructor(options) {
    this.runner = options.runner;
    this.askPassphrase = options.askPassphrase ?? (async () => null);
    this.now = options.now ?? (() => new Date());
    this.cache = options.cache ?? null;
    /** @type {Map<string, string[]>} gaps the last complete or confirmed load of each context agreed on */
    this.baselines = new Map();
    this.nextId = 0;
    /** @type {Map<string, {requestId: number, state: RunState, handle: import('./runner').RunHandle | null, prompt: AbortController | null}>} */
    this.contexts = new Map();
    /** @type {Map<string, string>} */
    this.credentials = new Map();
    /** @type {((key: string, state: RunState) => void)[]} */
    this.listeners = [];
  }

  /**
   * @param {(key: string, state: RunState) => void} listener
   * @returns {{dispose: () => void}}
   */
  onState(listener) {
    this.listeners.push(listener);
    return { dispose: () => void (this.listeners = this.listeners.filter((l) => l !== listener)) };
  }

  /**
   * @param {string} key
   * @returns {RunState}
   */
  stateOf(key) {
    return this.contexts.get(key)?.state ?? 'idle';
  }

  /**
   * @param {string} key
   * @returns {NonNullable<ReturnType<Scheduler['contexts']['get']>>}
   */
  context(key) {
    let ctx = this.contexts.get(key);
    if (ctx === undefined) {
      ctx = { requestId: 0, state: 'idle', handle: null, prompt: null };
      this.contexts.set(key, ctx);
    }
    return ctx;
  }

  /**
   * @param {string} key
   * @param {RunState} state
   */
  setState(key, state) {
    const ctx = this.context(key);
    if (ctx.state === state) return;
    ctx.state = state;
    for (const listener of this.listeners) listener(key, state);
  }

  /**
   * Runs one request. A deployed-state request first asks the cache (unless the
   * caller bypasses it) and, when it must run finfocus, judges the unpriced rows
   * by stability: a result whose gaps no baseline covers is run once more, and
   * gap sets that differ between the two runs make the result incomplete and
   * keep it out of the cache (FR-012a, FR-012b).
   *
   * @param {{
   *   key: string, projectDir: string, stack: string, stateOnly: boolean,
   *   snapshot?: RequestSnapshot | null, isCurrent?: () => boolean,
   *   cacheSignals?: import('./cache').CacheSignals | null, bypassCache?: boolean,
   * }} request
   * @returns {Promise<RunResult>}
   */
  async run({ key, projectDir, stack, stateOnly, snapshot = null, isCurrent = () => true, cacheSignals = null, bypassCache = false }) {
    const ctx = this.context(key);
    this.nextId += 1;
    const id = this.nextId;
    ctx.requestId = id;
    ctx.prompt?.abort();
    ctx.prompt = null;

    if (ctx.state === 'cleanupFailed') return this.blocked(key);
    if (ctx.handle !== null) {
      this.setState(key, 'cancelling');
      const stopped = await ctx.handle.stop();
      if (!stopped.confirmed) return this.failCleanup(key, stopped);
      ctx.handle = null;
    }
    if (ctx.requestId !== id) return { kind: 'superseded' };

    const cache = this.cache;
    const useCache = stateOnly && cache !== null && cacheSignals !== null;
    if (useCache && !bypassCache) {
      const hit = await cache.lookup(key, cacheSignals);
      if (ctx.requestId !== id || !isCurrent()) {
        this.release(key, id, null);
        return { kind: 'superseded' };
      }
      if (hit !== null) {
        this.baselines.set(key, hit.confirmedGaps);
        this.setState(key, 'idle');
        return { kind: 'result', result: hit.result, startedAt: hit.producedAt, requestId: id, snapshot, completeness: describe(hit.result, gapsOf(hit.result).length > 0 ? 'confirmed' : 'complete'), fromCache: true };
      }
    }

    const startedAt = this.now();
    const attempt = { key, ctx, id, projectDir, stack, stateOnly, isCurrent };
    const first = await this.attempt(attempt);
    if (first.terminal) return first.terminal;
    if (!stateOnly) {
      return { kind: 'result', result: first.result, startedAt, requestId: id, snapshot, completeness: null, fromCache: false };
    }

    const firstGaps = gapsOf(first.result);
    const baseline = await this.baselineFor(key);
    /** @type {{result: import('./result').OverviewResult, stdout: string}} */
    let kept = first;
    /** @type {import('./completeness').Settled} */
    let settled;
    if (needsRetry(firstGaps, baseline)) {
      const second = await this.attempt(attempt);
      if (second.terminal && second.terminal.kind !== 'failure') return second.terminal;
      if (second.terminal) {
        settled = { kind: 'incomplete', gaps: firstGaps, keep: 'first' };
      } else {
        settled = settle({ first: firstGaps, second: gapsOf(second.result), baseline });
        if (settled.keep === 'second') kept = second;
      }
    } else {
      settled = settle({ first: firstGaps, second: null, baseline });
    }

    if (settled.kind !== 'incomplete') {
      this.baselines.set(key, settled.gaps);
      if (useCache) await this.store(key, cacheSignals, { stdout: kept.stdout, producedAt: startedAt, confirmedGaps: settled.gaps });
    }
    return {
      kind: 'result',
      result: kept.result,
      startedAt,
      requestId: id,
      snapshot,
      completeness: describe(kept.result, settled.kind, settled.gaps),
      fromCache: false,
    };
  }

  /**
   * One finfocus process, including the passphrase prompt loop. Resolves to the
   * parsed result with its stdout, or to the terminal RunResult that ended it.
   *
   * @param {{
   *   key: string, ctx: NonNullable<ReturnType<Scheduler['contexts']['get']>>, id: number, projectDir: string,
   *   stack: string, stateOnly: boolean, isCurrent: () => boolean,
   * }} attempt
   * @returns {Promise<{terminal: RunResult} | {terminal?: undefined, result: OverviewResult, stdout: string}>}
   */
  async attempt({ key, ctx, id, projectDir, stack, stateOnly, isCurrent }) {
    let asked = false;
    for (;;) {
      /** @type {import('./runner').RunHandle} */
      let handle;
      try {
        handle = this.runner.overview({ projectDir, stack, stateOnly, passphrase: this.credentials.get(key) ?? null });
      } catch (error) {
        if (error instanceof RunnerError) {
          this.release(key, id, null);
          return { terminal: { kind: 'failure', message: error.message } };
        }
        throw error;
      }
      ctx.handle = handle;
      this.setState(key, 'running');
      const outcome = await handle.done;
      if (ctx.requestId !== id || !isCurrent()) {
        this.release(key, id, handle);
        return { terminal: { kind: 'superseded' } };
      }
      if (outcome.kind === 'failure' && outcome.failure?.category === 'passphrase_required') {
        ctx.handle = null;
        const answer = await this.prompt(key, ctx, asked);
        asked = true;
        if (ctx.requestId !== id || !isCurrent()) {
          this.release(key, id, null);
          return { terminal: { kind: 'superseded' } };
        }
        if (answer === null) {
          this.setState(key, 'idle');
          return { terminal: { kind: 'dismissed' } };
        }
        this.credentials.set(key, answer);
        continue;
      }
      ctx.handle = null;
      this.setState(key, 'idle');
      if (outcome.kind !== 'ok') return { terminal: { kind: 'failure', message: describeOutcome(outcome) } };
      try {
        return { result: parseOverview(outcome.stdout), stdout: outcome.stdout };
      } catch (error) {
        if (error instanceof ResultError) return { terminal: { kind: 'failure', message: error.message } };
        throw error;
      }
    }
  }

  /**
   * Gaps an earlier load confirmed for this context: this session's, else the
   * last stored entry's.
   *
   * @param {string} key
   * @returns {Promise<string[] | null>}
   */
  async baselineFor(key) {
    if (!this.baselines.has(key) && this.cache !== null) {
      const previous = await this.cache.previous(key);
      if (previous !== null) this.baselines.set(key, previous.confirmedGaps);
    }
    return this.baselines.get(key) ?? null;
  }

  /**
   * A cache that cannot be written only makes the next load slower.
   *
   * @param {string} key
   * @param {import('./cache').CacheSignals} signals
   * @param {{stdout: string, producedAt: Date, confirmedGaps: string[]}} fields
   */
  async store(key, signals, fields) {
    try {
      await this.cache?.save(key, signals, fields);
    } catch {
      // See above.
    }
  }

  /**
   * @param {string} key
   * @param {NonNullable<ReturnType<Scheduler['contexts']['get']>>} ctx
   * @param {boolean} retry
   * @returns {Promise<string | null>}
   */
  async prompt(key, ctx, retry) {
    const controller = new AbortController();
    ctx.prompt = controller;
    this.setState(key, 'awaitingPassphrase');
    try {
      return await this.askPassphrase({ key, retry, signal: controller.signal });
    } finally {
      if (ctx.prompt === controller) ctx.prompt = null;
    }
  }

  /**
   * Leaves a superseded request's bookkeeping to the request that replaced it.
   *
   * @param {string} key
   * @param {number} id
   * @param {import('./runner').RunHandle | null} handle
   */
  release(key, id, handle) {
    const ctx = this.context(key);
    if (ctx.requestId !== id) return;
    if (handle === null || ctx.handle === handle) ctx.handle = null;
    this.setState(key, 'idle');
  }

  /**
   * @param {string} key
   * @returns {RunResult}
   */
  blocked(key) {
    return { kind: 'blocked', message: 'Cleanup of the previous run has not been confirmed. Retry the cleanup or reload the window.' };
  }

  /**
   * @param {string} key
   * @param {StopResult} stopped
   * @returns {RunResult}
   */
  failCleanup(key, stopped) {
    this.setState(key, 'cleanupFailed');
    const pids = stopped.survivors.length > 0 ? ` (still running: ${stopped.survivors.join(', ')})` : '';
    return { kind: 'blocked', message: `Could not stop the previous finfocus run${pids}. No new run starts until it is stopped.` };
  }

  /**
   * Invalidates the context's request, then stops its process to a confirmed
   * end. Used on selection change and workspace removal.
   *
   * @param {string} key
   * @returns {Promise<StopResult | null>}
   */
  async cancel(key) {
    const ctx = this.contexts.get(key);
    if (ctx === undefined) return null;
    this.nextId += 1;
    ctx.requestId = this.nextId;
    ctx.prompt?.abort();
    ctx.prompt = null;
    if (ctx.handle === null) {
      if (ctx.state !== 'cleanupFailed') this.setState(key, 'idle');
      return { confirmed: true, forced: false, survivors: [] };
    }
    this.setState(key, 'cancelling');
    const stopped = await ctx.handle.stop();
    if (!stopped.confirmed) {
      this.setState(key, 'cleanupFailed');
      return stopped;
    }
    ctx.handle = null;
    this.setState(key, 'idle');
    return stopped;
  }

  /**
   * @param {string} key
   * @returns {Promise<StopResult>}
   */
  async retryCleanup(key) {
    const ctx = this.context(key);
    if (ctx.handle === null) {
      this.setState(key, 'idle');
      return { confirmed: true, forced: false, survivors: [] };
    }
    const stopped = await ctx.handle.stop();
    if (stopped.confirmed) {
      ctx.handle = null;
      this.setState(key, 'idle');
    }
    return stopped;
  }

  /**
   * Forgets the passphrase entered for one context, or every context.
   *
   * @param {string} [key]
   */
  clearCredentials(key) {
    if (key === undefined) this.credentials.clear();
    else this.credentials.delete(key);
  }

  /**
   * Stops every context; used on deactivation.
   *
   * @returns {Promise<StopResult[]>}
   */
  async closeAll() {
    const results = await Promise.all([...this.contexts.keys()].map((key) => this.cancel(key)));
    this.credentials.clear();
    return results.filter((r) => r !== null);
  }
}

module.exports = { Scheduler };
