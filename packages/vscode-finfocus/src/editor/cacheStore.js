'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const path = require('node:path');

/**
 * Stores cache entries as one JSON file per context under the extension's
 * global storage (the extension host's side, so a remote workspace caches
 * where finfocus runs). Global storage is used, not workspace storage: VS Code
 * locks a workspace-storage folder per live extension host and gives a second
 * window on the same folder a `-1` sibling, which would start every window with
 * an empty cache. The context key already names the project and stack. A file name is a hash of the context key, never the
 * key itself. Writes go to a temporary name and are renamed into place.
 *
 * The `0600` mode applies on Unix. On Windows the extension sets no ACL; the
 * per-user storage directory is the protection. It satisfies the cache's
 * `CacheStorePort`.
 */
class FileCacheStore {
  /** @param {string} directory */
  constructor(directory) {
    this.directory = path.join(directory, 'results');
  }

  /**
   * @param {string} key
   * @returns {string}
   */
  fileFor(key) {
    return path.join(this.directory, `${crypto.createHash('sha256').update(key).digest('hex')}.json`);
  }

  /**
   * @param {string} key
   * @returns {Promise<import('../cache').CacheEntry | null>}
   */
  async read(key) {
    const file = this.fileFor(key);
    let text;
    try {
      text = await fs.readFile(file, 'utf8');
    } catch (error) {
      if (isMissing(error)) return null;
      throw error;
    }
    try {
      return JSON.parse(text);
    } catch {
      await fs.rm(file, { force: true });
      return null;
    }
  }

  /**
   * @param {string} key
   * @param {import('../cache').CacheEntry} entry
   */
  async write(key, entry) {
    await fs.mkdir(this.directory, { recursive: true });
    const file = this.fileFor(key);
    const temporary = `${file}.tmp-${process.pid}-${crypto.randomBytes(4).toString('hex')}`;
    try {
      await fs.writeFile(temporary, JSON.stringify(entry), { mode: 0o600 });
      await fs.rename(temporary, file);
    } catch (error) {
      await fs.rm(temporary, { force: true });
      throw error;
    }
  }

  /** @param {string} key */
  async remove(key) {
    await fs.rm(this.fileFor(key), { force: true });
  }

  /**
   * Removes entries, and temporary files left by a crash, not touched for
   * `maxAgeMs`.
   *
   * @param {number} maxAgeMs
   * @param {() => number} [now]
   */
  async prune(maxAgeMs, now = Date.now) {
    let names;
    try {
      names = await fs.readdir(this.directory);
    } catch (error) {
      if (isMissing(error)) return;
      throw error;
    }
    const cutoff = now() - maxAgeMs;
    await Promise.all(
      names.map(async (name) => {
        const file = path.join(this.directory, name);
        try {
          const { mtimeMs } = await fs.stat(file);
          if (mtimeMs < cutoff) await fs.rm(file, { force: true });
        } catch (error) {
          if (!isMissing(error)) throw error;
        }
      }),
    );
  }
}

/**
 * @param {unknown} error
 * @returns {boolean}
 */
function isMissing(error) {
  return error instanceof Error && 'code' in error && error.code === 'ENOENT';
}

module.exports = { FileCacheStore };
