// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const GITHUB_USER = 'stuffbucket';
const REPO_NAME = 'workiq';
const IS_USER_SITE = false;

export default defineConfig({
  site: `https://${GITHUB_USER}.github.io`,
  base: IS_USER_SITE ? '/' : `/${REPO_NAME}`,
  integrations: [
    starlight({
      title: 'workiq-proxy',
      description:
        'Microsoft 365 Copilot emails, docs, chats, and meetings from any AI assistant.',
      favicon: '/favicon.svg',
      lastUpdated: true,
      editLink: {
        baseUrl: `https://github.com/${GITHUB_USER}/${REPO_NAME}/edit/main/docs/`,
      },
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: `https://github.com/${GITHUB_USER}/${REPO_NAME}`,
        },
      ],
      head: [
        {
          tag: 'meta',
          attrs: { property: 'og:type', content: 'website' },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:description',
            content:
              'MCP compatibility proxy for Microsoft Work IQ. Search M365 from any AI assistant.',
          },
        },
      ],
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 4 },
      customCss: ['./src/styles/custom.css'],
      components: {
        Hero: './src/components/Hero.astro',
      },
      sidebar: [
        {
          label: 'Start Here',
          autogenerate: { directory: 'getting-started' },
        },
        {
          label: 'Configure',
          autogenerate: { directory: 'guides' },
        },
        {
          label: 'Reference',
          autogenerate: { directory: 'reference' },
        },
      ],
    }),
  ],
});
