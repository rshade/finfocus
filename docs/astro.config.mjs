// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import sitemap from '@astrojs/sitemap';

// https://astro.build/config
export default defineConfig({
  site: 'https://rshade.github.io',
  base: '/finfocus',
  integrations: [
    starlight({
      title: 'FinFocus Documentation',
      description: 'Cost visibility for Pulumi infrastructure.',
      customCss: ['./src/styles/theme-bridge.css'],
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/rshade/finfocus',
        },
      ],
      head: [
        {
          tag: 'link',
          attrs: {
            rel: 'alternate',
            type: 'text/plain',
            href: '/finfocus/llms.txt',
            title: 'LLM-friendly content',
          },
        },
        {
          tag: 'script',
          attrs: {
            type: 'module',
          },
          content: `
            import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11.15.0/dist/mermaid.esm.min.mjs';
            mermaid.initialize({ startOnLoad: false, theme: 'dark' });

            function renderMermaid() {
              document.querySelectorAll('pre[data-language="mermaid"]').forEach((pre, i) => {
                const lines = pre.querySelectorAll('.ec-line');
                const code = Array.from(lines).map(line => line.textContent).join('\\n');
                const div = document.createElement('div');
                div.className = 'mermaid';
                div.id = 'mermaid-' + i;
                div.textContent = code;
                
                const expressiveCode = pre.closest('.expressive-code');
                if (expressiveCode) {
                    expressiveCode.replaceWith(div);
                } else {
                    pre.replaceWith(div);
                }
              });
              mermaid.run();
            }

            renderMermaid();
            document.addEventListener('astro:page-load', renderMermaid);
          `,
        },
      ],
      sidebar: [
        { label: 'Home', link: '/' },
        {
          label: 'Getting Started',
          collapsed: false,
          items: [{ autogenerate: { directory: 'getting-started' } }],
        },
        {
          label: 'Guides',
          collapsed: false,
          items: [{ autogenerate: { directory: 'guides' } }],
        },
        {
          label: 'Architecture',
          collapsed: false,
          items: [{ autogenerate: { directory: 'architecture' } }],
        },
        {
          label: 'Plugins',
          collapsed: false,
          items: [{ autogenerate: { directory: 'plugins' } }],
        },
        {
          label: 'Reference',
          collapsed: true,
          items: [{ autogenerate: { directory: 'reference', collapsed: true } }],
        },
        {
          label: 'Deployment',
          collapsed: true,
          items: [{ autogenerate: { directory: 'deployment', collapsed: true } }],
        },
        {
          label: 'Support',
          collapsed: true,
          items: [{ autogenerate: { directory: 'support', collapsed: true } }],
        },
        {
          label: 'Commands',
          collapsed: true,
          items: [{ autogenerate: { directory: 'commands', collapsed: true } }],
        },
        {
          label: 'Testing',
          collapsed: true,
          items: [{ autogenerate: { directory: 'testing', collapsed: true } }],
        },
        {
          label: 'Project',
          items: [
            { label: 'README', link: '/readme/' },
            { label: 'Installation', link: '/installation/' },
            { label: 'User Guide', link: '/user-guide/' },
            { label: 'Cost Calculations', link: '/cost-calculations/' },
            { label: 'Plugin System', link: '/plugin-system/' },
            { label: 'Analyzer Integration', link: '/analyzer-integration/' },
            { label: 'Troubleshooting', link: '/troubleshooting/' },
            { label: 'Table of Contents', link: '/table-of-contents/' },
            { label: 'Plan', link: '/plan/' },
          ],
        },
      ],
    }),
    sitemap(),
  ],
});
