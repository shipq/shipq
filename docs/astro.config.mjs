// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
  integrations: [
    starlight({
      title: 'ShipQ',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/shipq/shipq' }],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Quickstart', slug: 'getting-started/quickstart' },
          ],
        },
        {
          label: 'Core Concepts',
          items: [
            { label: 'The Compiler Chain', slug: 'concepts/compiler-chain' },
            { label: 'Configuration (shipq.ini)', slug: 'concepts/configuration' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Schema & Migrations', slug: 'guides/migrations' },
            { label: 'Queries (PortSQL)', slug: 'guides/queries' },
            { label: 'Handlers & Resources', slug: 'guides/handlers' },
            { label: 'Authentication', slug: 'guides/authentication' },
            { label: 'Multi-Tenancy', slug: 'guides/multi-tenancy' },
            { label: 'File Uploads', slug: 'guides/file-uploads' },
            { label: 'Workers & Channels', slug: 'guides/workers' },
            { label: 'LLM Tools', slug: 'guides/llm-tools' },
            { label: 'Task DAGs', slug: 'guides/task-dags' },
            { label: 'TypeScript Clients', slug: 'guides/typescript' },
            { label: 'Deployment', slug: 'guides/deployment' },
          ],
        },
        {
          label: 'E2E Example',
          items: [
            { label: 'Building a Full App', slug: 'e2e-example' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI Commands', slug: 'reference/cli' },
            { label: 'Column Types', slug: 'reference/column-types' },
            { label: 'shipq.ini Reference', slug: 'reference/ini-config' },
          ],
        },
        {
          label: 'LLMs',
          items: [
            { label: 'llms.txt', slug: 'llms' },
          ],
        },
      ],
    }),
  ],
});
