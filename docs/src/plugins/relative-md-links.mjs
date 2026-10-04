import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const MD_LINK = /^(?![a-z][a-z\d+.-]*:|[/#])([^#?]*?)\.mdx?(#.*)?$/i;

/**
 * Sätteri mdast plugin that rewrites relative links to Markdown files
 * (`./cache.md`, `../plugins/jev.md#setup`) into site routes
 * (`/finfocus/guides/cache/`, `/finfocus/plugins/jev/#setup`).
 *
 * Authors link file-to-file so the sources browse correctly on GitHub, but the
 * browser resolves a relative href against the page URL, which has one more path
 * segment than the source file and no `.md` extension, so those links 404.
 * Links to missing files or files outside the content directory are left
 * unchanged, so a dead link stays visible instead of becoming a different one.
 */
export function relativeMdLinks({ base, contentDir }) {
  const prefix = base.replace(/\/$/, '');

  const toRoute = (url, sourceFile) => {
    const match = MD_LINK.exec(url);
    if (!match) return null;
    const target = path.resolve(path.dirname(sourceFile), match[1]);
    const rel = path.relative(contentDir, target);
    if (rel.startsWith('..') || path.isAbsolute(rel)) return null;
    if (!['.md', '.mdx'].some((ext) => fs.existsSync(target + ext))) return null;
    const segments = rel.split(path.sep).map((s) => s.toLowerCase().replace(/\s+/g, '-'));
    if (segments.at(-1) === 'index') segments.pop();
    const slug = segments.join('/');
    return `${prefix}/${slug ? `${slug}/` : ''}${match[2] ?? ''}`;
  };

  return ({ fileURL }) => {
    if (!fileURL) return null;
    const sourceFile = fileURLToPath(fileURL);
    const rewrite = (node, ctx) => {
      const route = toRoute(node.url, sourceFile);
      if (route) ctx.setProperty(node, 'url', route);
    };
    return { name: 'relative-md-links', link: rewrite, definition: rewrite };
  };
}
