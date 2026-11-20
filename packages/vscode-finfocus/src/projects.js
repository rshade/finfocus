'use strict';

/**
 * Project and stack identity. Everything here works on URI strings and plain
 * data so it can be tested without an editor; a remote authority is part of
 * the URI, so it is part of every key.
 */

/**
 * @typedef {{
 *   name: string, current: boolean, lastUpdate?: string, resourceCount?: number, updateInProgress?: boolean,
 * }} StackEntry
 */

const EXCLUDED_DIRECTORIES = new Set(['node_modules', '.git', 'vendor', 'dist', 'build', 'out', 'target']);
const PROJECT_FILES = new Set(['Pulumi.yaml', 'Pulumi.yml']);
const SAFE_STACK_SEGMENT = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

class ProjectError extends Error {
  /**
   * @param {'malformed_stack_list'} code
   * @param {string} message
   */
  constructor(code, message) {
    super(message);
    this.name = 'ProjectError';
    this.code = code;
  }
}

/**
 * @param {string} uri
 * @returns {string}
 */
function trimSlash(uri) {
  return uri.length > 1 && uri.endsWith('/') ? uri.slice(0, -1) : uri;
}

/**
 * Parses `pulumi stack ls --json`. The identity is the `name` the command
 * reports. Besides `current`, only the cache's change signals are kept, and
 * only when they have the right type: `lastUpdate`, `resourceCount`, and
 * `updateInProgress`.
 *
 * @param {string} text
 * @returns {StackEntry[]}
 */
function parseStackList(text) {
  /** @type {unknown} */
  let doc;
  try {
    doc = JSON.parse(text);
  } catch {
    throw new ProjectError('malformed_stack_list', 'pulumi stack ls did not return JSON');
  }
  if (!Array.isArray(doc)) {
    throw new ProjectError('malformed_stack_list', 'pulumi stack ls did not return a list');
  }
  return doc.map((entry) => {
    if (entry === null || typeof entry !== 'object' || typeof entry.name !== 'string' || entry.name === '') {
      throw new ProjectError('malformed_stack_list', 'pulumi stack ls returned an entry with no name');
    }
    /** @type {StackEntry} */
    const stack = { name: entry.name, current: entry.current === true };
    if (typeof entry.lastUpdate === 'string' && entry.lastUpdate !== '') stack.lastUpdate = entry.lastUpdate;
    if (Number.isInteger(entry.resourceCount) && entry.resourceCount >= 0) stack.resourceCount = entry.resourceCount;
    if (typeof entry.updateInProgress === 'boolean') stack.updateInProgress = entry.updateInProgress;
    return stack;
  });
}

/**
 * Returns the project directories named by Pulumi.yaml or Pulumi.yml files,
 * skipping dependency, VCS and build directories below a workspace folder.
 *
 * @param {string[]} fileUris
 * @param {string[]} workspaceFolders
 * @returns {string[]} sorted, unique
 */
function discoverProjects(fileUris, workspaceFolders) {
  const folders = workspaceFolders.map(trimSlash).sort((a, b) => b.length - a.length);
  const found = new Set();
  for (const uri of fileUris) {
    const slash = uri.lastIndexOf('/');
    if (slash < 0 || !PROJECT_FILES.has(uri.slice(slash + 1))) continue;
    const dir = trimSlash(uri.slice(0, slash));
    const folder = folders.find((f) => dir === f || dir.startsWith(`${f}/`));
    if (folder === undefined) continue;
    const below = dir.slice(folder.length).split('/').filter(Boolean);
    if (below.some((segment) => EXCLUDED_DIRECTORIES.has(segment))) continue;
    found.add(dir);
  }
  return [...found].sort();
}

/**
 * The longest project directory that contains the file, or null.
 *
 * @param {string[]} projects
 * @param {string} fileUri
 * @returns {string | null}
 */
function nearestProject(projects, fileUri) {
  let best = null;
  for (const project of projects) {
    const root = trimSlash(project);
    if (fileUri !== root && !fileUri.startsWith(`${root}/`)) continue;
    if (best === null || root.length > best.length) best = root;
  }
  return best;
}

/**
 * @typedef {{kind: 'selected', project: string} | {kind: 'pick', candidates: string[]} | {kind: 'none'}} ProjectChoice
 */

/**
 * Applies the contract's project rules: nearest ancestor of the active file,
 * else the retained explicit choice, else the sole project, else ask.
 *
 * @param {{projects: string[], activeFile: string | null, explicit: string | null}} input
 * @returns {ProjectChoice}
 */
function resolveProject({ projects, activeFile, explicit }) {
  if (activeFile !== null) {
    const near = nearestProject(projects, activeFile);
    if (near !== null) return { kind: 'selected', project: near };
  }
  if (explicit !== null && projects.includes(explicit)) return { kind: 'selected', project: explicit };
  if (projects.length === 1) return { kind: 'selected', project: projects[0] };
  if (projects.length > 1) return { kind: 'pick', candidates: [...projects] };
  return { kind: 'none' };
}

/**
 * @typedef {{kind: 'selected', stack: string} | {kind: 'pick', stacks: string[]} | {kind: 'none'}} StackChoice
 */

/**
 * @param {StackEntry[]} stacks
 * @param {string | null} explicit
 * @returns {StackChoice}
 */
function chooseStack(stacks, explicit) {
  if (explicit !== null && stacks.some((s) => s.name === explicit)) {
    return { kind: 'selected', stack: explicit };
  }
  const current = stacks.find((s) => s.current);
  if (current !== undefined) return { kind: 'selected', stack: current.name };
  if (stacks.length > 0) return { kind: 'pick', stacks: stacks.map((s) => s.name) };
  return { kind: 'none' };
}

/**
 * The key for every per-context map. Encoding the pair as JSON keeps any
 * character in a URI or a stack name from colliding with another pair.
 *
 * @param {string} projectUri
 * @param {string} stackRef
 * @returns {string}
 */
function contextKey(projectUri, stackRef) {
  return JSON.stringify([trimSlash(projectUri), stackRef]);
}

/**
 * Stack settings file names core reads for a stack. A qualified reference
 * (org/project/stack) maps to its last segment only, and a name that is not a
 * plain file-name segment maps to nothing, so a stack name is never
 * interpolated into an arbitrary path.
 *
 * @param {string} stackRef
 * @returns {string[]}
 */
function stackSettingsFiles(stackRef) {
  const cut = Math.max(stackRef.lastIndexOf('/'), stackRef.lastIndexOf('\\'));
  const short = cut >= 0 ? stackRef.slice(cut + 1) : stackRef;
  if (!SAFE_STACK_SEGMENT.test(short) || short.includes('..')) return [];
  if (cut >= 0 && stackRef.split(/[/\\]/).some((part) => part === '' || part === '..')) return [];
  return [`Pulumi.${short}.yaml`, `Pulumi.${short}.yml`];
}

/**
 * @param {string} fileName base name of a changed file
 * @param {string} stackRef
 * @returns {boolean}
 */
function isStackSettingsFile(fileName, stackRef) {
  return stackSettingsFiles(stackRef).includes(fileName);
}

/**
 * Session-local project and stack choices. Every change of the active
 * context bumps the epoch, so a caller can reject late work from an old one.
 * Nothing here touches Pulumi's own selected stack.
 */
class Selection {
  constructor() {
    this.epoch = 0;
    /** @type {string | null} */
    this.project = null;
    /** @type {Map<string, string>} */
    this.stacks = new Map();
  }

  /**
   * @param {string} project
   * @returns {number} the epoch of the resulting selection
   */
  selectProject(project) {
    if (this.project !== project) {
      this.project = project;
      this.epoch += 1;
    }
    return this.epoch;
  }

  /**
   * @param {string} project
   * @param {string} stack
   * @returns {number}
   */
  selectStack(project, stack) {
    const changed = this.project !== project || this.stacks.get(project) !== stack;
    this.project = project;
    this.stacks.set(project, stack);
    if (changed) this.epoch += 1;
    return this.epoch;
  }

  /**
   * @param {string} project
   * @returns {string | null}
   */
  stackFor(project) {
    return this.stacks.get(project) ?? null;
  }

  /**
   * Forgets a project after it was removed or its backend or configuration
   * changed.
   *
   * @param {string} project
   * @returns {number}
   */
  invalidateProject(project) {
    this.stacks.delete(project);
    if (this.project === project) {
      this.project = null;
      this.epoch += 1;
    }
    return this.epoch;
  }

  /**
   * Drops a stack choice that no longer exists, keeping the project.
   *
   * @param {string} project
   * @returns {number}
   */
  forgetStack(project) {
    if (this.stacks.delete(project) && this.project === project) this.epoch += 1;
    return this.epoch;
  }

  /**
   * @param {number} epoch
   * @returns {boolean}
   */
  isCurrent(epoch) {
    return epoch === this.epoch;
  }
}

module.exports = {
  ProjectError,
  parseStackList,
  discoverProjects,
  nearestProject,
  resolveProject,
  chooseStack,
  contextKey,
  stackSettingsFiles,
  isStackSettingsFile,
  Selection,
};
